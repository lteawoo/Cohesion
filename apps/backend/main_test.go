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

func TestResolveJWTSecret_ProductionGeneratesSecretFileWhenMissing(t *testing.T) {
	setGoEnvForTest(t, "production")
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

func TestPrewarmRequiredSecrets_FirstRunGeneratesAndRestartReuses(t *testing.T) {
	setGoEnvForTest(t, "development")

	secretDir := t.TempDir()
	jwtPath := filepath.Join(secretDir, "jwt_secret")
	sftpPath := filepath.Join(secretDir, "sftp_host_key")

	t.Setenv("COHESION_JWT_SECRET", "")
	t.Setenv("COHESION_JWT_SECRET_FILE", jwtPath)
	t.Setenv("COHESION_SFTP_HOST_KEY_FILE", sftpPath)

	first, err := prewarmRequiredSecrets()
	if err != nil {
		t.Fatalf("first prewarm: %v", err)
	}
	if strings.TrimSpace(first.jwtSecret) == "" {
		t.Fatal("expected prewarmed jwt secret")
	}

	jwtBefore := readTrimmedFile(t, jwtPath)
	sftpBefore := readTrimmedFile(t, sftpPath)
	if jwtBefore == "" || sftpBefore == "" {
		t.Fatal("expected first prewarm to generate all required secret files")
	}

	second, err := prewarmRequiredSecrets()
	if err != nil {
		t.Fatalf("second prewarm: %v", err)
	}
	if first.jwtSecret != second.jwtSecret {
		t.Fatalf("expected jwt secret reuse, first=%q second=%q", first.jwtSecret, second.jwtSecret)
	}

	if jwtBefore != readTrimmedFile(t, jwtPath) {
		t.Fatal("expected jwt secret file to be reused on restart")
	}
	if sftpBefore != readTrimmedFile(t, sftpPath) {
		t.Fatal("expected sftp host key file to be reused on restart")
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

func TestResolveJWTSecretPath_ProductionUsesHiddenHomeDir(t *testing.T) {
	setGoEnvForTest(t, "production")
	t.Setenv("COHESION_JWT_SECRET_FILE", "")
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	secretPath, err := resolveJWTSecretPath()
	if err != nil {
		t.Fatalf("resolve jwt secret path: %v", err)
	}

	expected := filepath.Join(homeDir, ".cohesion", "secrets", "jwt_secret")
	if secretPath != expected {
		t.Fatalf("expected %q, got %q", expected, secretPath)
	}
}

func TestDetectInstallChannel(t *testing.T) {
	t.Run("returns homebrew for Cellar path", func(t *testing.T) {
		if actual := detectInstallChannelFromPath("/opt/homebrew/Cellar/cohesion/0.5.17/bin/cohesion"); actual != "homebrew" {
			t.Fatalf("expected homebrew, got %q", actual)
		}
	})

	t.Run("returns direct for non Cellar path", func(t *testing.T) {
		if actual := detectInstallChannelFromPath("/usr/local/bin/cohesion"); actual != "direct" {
			t.Fatalf("expected direct, got %q", actual)
		}
	})

	t.Run("returns env override when provided", func(t *testing.T) {
		if actual := detectInstallChannelFromEnvAndPath("systemd", "/usr/local/bin/cohesion"); actual != "systemd" {
			t.Fatalf("expected systemd, got %q", actual)
		}
	})
}

func readTrimmedFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	return strings.TrimSpace(string(content))
}

func setGoEnvForTest(t *testing.T, env string) {
	t.Helper()

	previous := goEnv
	goEnv = env
	t.Cleanup(func() {
		goEnv = previous
	})
}
