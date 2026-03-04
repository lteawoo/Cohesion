package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"taeu.kr/cohesion/internal/account"
	accountStore "taeu.kr/cohesion/internal/account/store"
	"taeu.kr/cohesion/internal/platform/database"
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
	smbPath := filepath.Join(secretDir, "smb_material_key")
	sftpPath := filepath.Join(secretDir, "sftp_host_key")

	t.Setenv("COHESION_JWT_SECRET", "")
	t.Setenv("COHESION_JWT_SECRET_FILE", jwtPath)
	t.Setenv("COHESION_SMB_MATERIAL_KEY", "")
	t.Setenv("COHESION_SMB_MATERIAL_KEY_FILE", smbPath)
	t.Setenv("COHESION_SFTP_HOST_KEY_FILE", sftpPath)

	svc, db := setupSMBMaterialPolicyService(t)
	defer db.Close()

	first, err := prewarmRequiredSecrets(context.Background(), svc)
	if err != nil {
		t.Fatalf("first prewarm: %v", err)
	}
	if strings.TrimSpace(first.jwtSecret) == "" {
		t.Fatal("expected prewarmed jwt secret")
	}

	jwtBefore := readTrimmedFile(t, jwtPath)
	smbBefore := readTrimmedFile(t, smbPath)
	sftpBefore := readTrimmedFile(t, sftpPath)
	if jwtBefore == "" || smbBefore == "" || sftpBefore == "" {
		t.Fatal("expected first prewarm to generate all required secret files")
	}

	second, err := prewarmRequiredSecrets(context.Background(), svc)
	if err != nil {
		t.Fatalf("second prewarm: %v", err)
	}
	if first.jwtSecret != second.jwtSecret {
		t.Fatalf("expected jwt secret reuse, first=%q second=%q", first.jwtSecret, second.jwtSecret)
	}

	if jwtBefore != readTrimmedFile(t, jwtPath) {
		t.Fatal("expected jwt secret file to be reused on restart")
	}
	if smbBefore != readTrimmedFile(t, smbPath) {
		t.Fatal("expected smb key file to be reused on restart")
	}
	if sftpBefore != readTrimmedFile(t, sftpPath) {
		t.Fatal("expected sftp host key file to be reused on restart")
	}
}

func TestPrewarmRequiredSecrets_FailsWhenSMBKeyMissingWithExistingCredentialData(t *testing.T) {
	setGoEnvForTest(t, "development")

	secretDir := t.TempDir()
	jwtPath := filepath.Join(secretDir, "jwt_secret")
	smbPath := filepath.Join(secretDir, "smb_material_key")
	sftpPath := filepath.Join(secretDir, "sftp_host_key")

	t.Setenv("COHESION_JWT_SECRET", "")
	t.Setenv("COHESION_JWT_SECRET_FILE", jwtPath)
	t.Setenv("COHESION_SMB_MATERIAL_KEY", "seed-smb-key")
	t.Setenv("COHESION_SMB_MATERIAL_KEY_FILE", smbPath)
	t.Setenv("COHESION_SFTP_HOST_KEY_FILE", sftpPath)

	svc, db := setupSMBMaterialPolicyService(t)
	defer db.Close()

	_, err := svc.CreateUser(context.Background(), &account.CreateUserRequest{
		Username: "prewarm-fail-user",
		Password: "prewarm-fail-password",
		Nickname: "Prewarm Fail User",
		Role:     account.RoleUser,
	})
	if err != nil {
		t.Fatalf("create user for smb credential seed: %v", err)
	}

	t.Setenv("COHESION_SMB_MATERIAL_KEY", "")
	t.Setenv("COHESION_SMB_MATERIAL_KEY_FILE", filepath.Join(secretDir, "missing_smb_material_key"))

	_, err = prewarmRequiredSecrets(context.Background(), svc)
	if err == nil {
		t.Fatal("expected startup prewarm failure when smb key is missing with existing credential data")
	}
	if !errors.Is(err, account.ErrSMBCredentialRecoveryRequired) {
		t.Fatalf("expected recoverable smb key guidance error, got %v", err)
	}
	if !strings.Contains(err.Error(), "restore COHESION_SMB_MATERIAL_KEY or COHESION_SMB_MATERIAL_KEY_FILE") {
		t.Fatalf("expected restore guidance in error, got %v", err)
	}
}

func TestPrewarmRequiredSecrets_FailsWithUnderlyingSMBKeyReadError(t *testing.T) {
	setGoEnvForTest(t, "development")

	secretDir := t.TempDir()
	jwtPath := filepath.Join(secretDir, "jwt_secret")
	sftpPath := filepath.Join(secretDir, "sftp_host_key")
	smbPathAsDirectory := filepath.Join(secretDir, "smb_key_dir")
	if err := os.MkdirAll(smbPathAsDirectory, 0o755); err != nil {
		t.Fatalf("create smb key directory fixture: %v", err)
	}

	t.Setenv("COHESION_JWT_SECRET", "")
	t.Setenv("COHESION_JWT_SECRET_FILE", jwtPath)
	t.Setenv("COHESION_SMB_MATERIAL_KEY", "")
	t.Setenv("COHESION_SMB_MATERIAL_KEY_FILE", smbPathAsDirectory)
	t.Setenv("COHESION_SFTP_HOST_KEY_FILE", sftpPath)

	svc, db := setupSMBMaterialPolicyService(t)
	defer db.Close()

	_, err := prewarmRequiredSecrets(context.Background(), svc)
	if err == nil {
		t.Fatal("expected startup prewarm failure when smb key path is unreadable")
	}
	if errors.Is(err, account.ErrSMBCredentialRecoveryRequired) {
		t.Fatalf("expected raw bootstrap read error, got recoverable guidance: %v", err)
	}
	if !strings.Contains(err.Error(), "[smb_material_key]") {
		t.Fatalf("expected smb bootstrap context in error, got %v", err)
	}
	if !strings.Contains(err.Error(), smbPathAsDirectory) {
		t.Fatalf("expected filesystem read error details in error, got %v", err)
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
	t.Setenv("COHESION_SMB_MATERIAL_KEY_FILE", filepath.Join(t.TempDir(), "missing_smb_material_key"))

	svc, db := setupSMBMaterialPolicyService(t)
	defer db.Close()

	err := configureSMBMaterialKeyPolicy(svc, true)
	if err == nil {
		t.Fatal("expected error when smb key is missing under production+smb_enabled")
	}
	if !strings.Contains(err.Error(), "smb material key missing") {
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

	if err := configureSMBMaterialKeyPolicy(nil, false); err != nil {
		t.Fatalf("expected no error when smb key missing but smb disabled: %v", err)
	}
	if account.IsSMBMaterialKeyRequired() {
		t.Fatal("expected smb material key requirement to remain disabled")
	}
}

func setupSMBMaterialPolicyService(t *testing.T) (*account.Service, *sql.DB) {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	return account.NewService(accountStore.NewStore(db)), db
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
