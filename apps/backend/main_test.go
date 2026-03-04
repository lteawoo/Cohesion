package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"taeu.kr/cohesion/internal/account"
	"taeu.kr/cohesion/internal/platform/logging"
)

func TestRegisterWebDAVRoutes_NoRedirectForBasePath(t *testing.T) {
	mux := http.NewServeMux()
	registerWebDAVRoutes(mux, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	testPaths := []string{"/dav", "/dav/", "/dav/test-space"}
	for _, path := range testPaths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest("PROPFIND", path, nil)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusNoContent {
				t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
			}
			if location := rec.Header().Get("Location"); location != "" {
				t.Fatalf("expected no redirect location header, got %q", location)
			}
		})
	}
}

func TestEmitAccessLog_IncludesQueryWhenPresent(t *testing.T) {
	previousLogger := accessLogger
	defer func() {
		accessLogger = previousLogger
	}()

	var buffer bytes.Buffer
	accessLogger = newAccessLogger(&buffer)

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=hello&sort=asc", nil)
	emitAccessLog(req, http.StatusOK, 128, 250*time.Millisecond)

	line := buffer.String()
	if !strings.Contains(line, "event="+logging.EventHTTPAccess) {
		t.Fatalf("expected access event in log line, got %q", line)
	}
	if !strings.Contains(line, "path=/api/search") {
		t.Fatalf("expected path field in log line, got %q", line)
	}
	if !strings.Contains(line, `query="q=hello&sort=asc"`) {
		t.Fatalf("expected query field in log line, got %q", line)
	}
}

func TestEmitAccessLog_OmitsQueryWhenEmpty(t *testing.T) {
	previousLogger := accessLogger
	defer func() {
		accessLogger = previousLogger
	}()

	var buffer bytes.Buffer
	accessLogger = newAccessLogger(&buffer)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	emitAccessLog(req, http.StatusOK, 64, 50*time.Millisecond)

	line := buffer.String()
	if strings.Contains(line, "query=") {
		t.Fatalf("expected no query field for empty query string, got %q", line)
	}
}

func TestResolveJWTSecret_ProductionRejectsMissingSecret(t *testing.T) {
	setGoEnvForTest(t, "production")
	t.Setenv("COHESION_JWT_SECRET", "")

	secretFile := filepath.Join(t.TempDir(), "jwt_secret")
	t.Setenv("COHESION_JWT_SECRET_FILE", secretFile)

	_, err := resolveJWTSecret()
	if err == nil {
		t.Fatal("expected error when production jwt secret is missing")
	}
	if !strings.Contains(err.Error(), "COHESION_JWT_SECRET is required in production") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveJWTSecret_DevelopmentGeneratesSecretFile(t *testing.T) {
	setGoEnvForTest(t, "development")
	t.Setenv("COHESION_JWT_SECRET", "")

	secretFile := filepath.Join(t.TempDir(), "jwt_secret")
	t.Setenv("COHESION_JWT_SECRET_FILE", secretFile)

	secret, err := resolveJWTSecret()
	if err != nil {
		t.Fatalf("resolve jwt secret: %v", err)
	}
	if strings.TrimSpace(secret) == "" {
		t.Fatal("expected non-empty jwt secret")
	}
	content, err := os.ReadFile(secretFile)
	if err != nil {
		t.Fatalf("read generated jwt secret file: %v", err)
	}
	if strings.TrimSpace(string(content)) == "" {
		t.Fatalf("expected generated jwt secret file to be non-empty, got %q", string(content))
	}
}

func TestConfigureSMBMaterialKeyPolicy_ProductionRequiresKeyWhenSMBEnabled(t *testing.T) {
	setGoEnvForTest(t, "production")
	account.SetSMBMaterialKeyRequired(false)
	t.Cleanup(func() {
		account.SetSMBMaterialKeyRequired(false)
	})
	t.Setenv("COHESION_SMB_MATERIAL_KEY", "")

	err := configureSMBMaterialKeyPolicy(true)
	if err == nil {
		t.Fatal("expected error when smb key is missing under production+smb_enabled")
	}
	if !strings.Contains(err.Error(), "COHESION_SMB_MATERIAL_KEY is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigureSMBMaterialKeyPolicy_ProductionAllowsMissingKeyWhenSMBDisabled(t *testing.T) {
	setGoEnvForTest(t, "production")
	account.SetSMBMaterialKeyRequired(false)
	t.Cleanup(func() {
		account.SetSMBMaterialKeyRequired(false)
	})
	t.Setenv("COHESION_SMB_MATERIAL_KEY", "")

	if err := configureSMBMaterialKeyPolicy(false); err != nil {
		t.Fatalf("expected no error when smb key missing but smb disabled: %v", err)
	}
	if account.IsSMBMaterialKeyRequired() {
		t.Fatal("expected smb material key requirement to remain disabled")
	}
}

func setGoEnvForTest(t *testing.T, env string) {
	t.Helper()

	previous := goEnv
	goEnv = env
	t.Cleanup(func() {
		goEnv = previous
	})
}
