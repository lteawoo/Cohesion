package system

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestStatusStore(t *testing.T) *StatusStore {
	t.Helper()

	statusPath := filepath.Join(t.TempDir(), "system-status.json")
	t.Setenv(lifecycleStatusPathEnv, statusPath)
	store := NewStatusStore()
	t.Cleanup(func() {
		_ = os.Unsetenv(lifecycleStatusPathEnv)
	})
	return store
}

func TestGetVersion(t *testing.T) {
	handler := NewHandler(make(chan RestartRequest, 1), make(chan struct{}, 1), Meta{
		Version:   "v0.3.0",
		Commit:    "abc123",
		BuildDate: "2026-02-24T00:00:00Z",
	}, newTestStatusStore(t))

	req := httptest.NewRequest(http.MethodGet, "/api/system/version", nil)
	rec := httptest.NewRecorder()

	if err := handler.GetVersion(rec, req); err != nil {
		t.Fatalf("expected no error, got %+v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]string
	if decodeErr := json.Unmarshal(rec.Body.Bytes(), &payload); decodeErr != nil {
		t.Fatalf("failed to decode response: %v", decodeErr)
	}

	if payload["version"] != "v0.3.0" {
		t.Fatalf("expected version v0.3.0, got %q", payload["version"])
	}
	if payload["commit"] != "abc123" {
		t.Fatalf("expected commit abc123, got %q", payload["commit"])
	}
	if payload["buildDate"] != "2026-02-24T00:00:00Z" {
		t.Fatalf("expected buildDate 2026-02-24T00:00:00Z, got %q", payload["buildDate"])
	}
}

func TestGetUpdateCheckReturnsGracefulPayloadOnFailure(t *testing.T) {
	failingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer failingServer.Close()

	handler := NewHandler(make(chan RestartRequest, 1), make(chan struct{}, 1), Meta{
		Version: "v0.3.0",
	}, newTestStatusStore(t))
	handler.updateChecker = NewUpdateChecker(UpdateCheckerConfig{
		APIBaseURL:     failingServer.URL,
		CacheTTL:       time.Minute,
		RequestTimeout: time.Second,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/system/update-check", nil)
	rec := httptest.NewRecorder()

	if err := handler.GetUpdateCheck(rec, req); err != nil {
		t.Fatalf("expected no error, got %+v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload UpdateCheckResult
	if decodeErr := json.Unmarshal(rec.Body.Bytes(), &payload); decodeErr != nil {
		t.Fatalf("failed to decode response: %v", decodeErr)
	}

	if payload.CurrentVersion != "v0.3.0" {
		t.Fatalf("expected currentVersion v0.3.0, got %q", payload.CurrentVersion)
	}
	if payload.UpdateAvailable {
		t.Fatal("expected updateAvailable=false when release check fails")
	}
	if payload.Error == "" {
		t.Fatal("expected error message when update check fails")
	}
}

func TestGetUpdateStatus(t *testing.T) {
	store := newTestStatusStore(t)
	if _, err := store.MarkRestartAccepted(RestartRequest{Actor: "admin", RequestID: "req-1", Port: "3000"}, "v0.3.0"); err != nil {
		t.Fatalf("failed to seed restart state: %v", err)
	}

	handler := NewHandler(make(chan RestartRequest, 1), make(chan struct{}, 1), Meta{
		Version: "v0.3.0",
	}, store)

	req := httptest.NewRequest(http.MethodGet, "/api/system/update/status", nil)
	rec := httptest.NewRecorder()

	if err := handler.GetUpdateStatus(rec, req); err != nil {
		t.Fatalf("expected no error, got %+v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload SelfUpdateStatus
	if decodeErr := json.Unmarshal(rec.Body.Bytes(), &payload); decodeErr != nil {
		t.Fatalf("failed to decode response: %v", decodeErr)
	}
	if payload.State == "" {
		t.Fatal("expected state to be set")
	}
	if payload.RuntimeState != "restarting" {
		t.Fatalf("expected runtimeState restarting, got %q", payload.RuntimeState)
	}
}

func TestStartUpdateReturnsBadRequestOnDevBuild(t *testing.T) {
	handler := NewHandler(make(chan RestartRequest, 1), make(chan struct{}, 1), Meta{
		Version: "dev",
	}, newTestStatusStore(t))

	req := httptest.NewRequest(http.MethodPost, "/api/system/update/start", nil)
	rec := httptest.NewRecorder()

	err := handler.StartUpdate(rec, req)
	if err == nil {
		t.Fatal("expected error response for dev build")
	}
	if err.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, err.Code)
	}
	if !strings.Contains(err.Message, "release builds") {
		t.Fatalf("unexpected message: %s", err.Message)
	}
}

func TestRestartServerReturnsAcceptedResponse(t *testing.T) {
	restartChan := make(chan RestartRequest, 1)
	handler := NewHandler(restartChan, make(chan struct{}, 1), Meta{
		Version: "v0.3.0",
	}, newTestStatusStore(t))

	req := httptest.NewRequest(http.MethodPost, "/api/system/restart", nil)
	req.Header.Set("X-Request-Id", "req-restart")
	rec := httptest.NewRecorder()

	if err := handler.RestartServer(rec, req); err != nil {
		t.Fatalf("expected no error, got %+v", err)
	}
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, rec.Code)
	}

	var payload map[string]string
	if decodeErr := json.Unmarshal(rec.Body.Bytes(), &payload); decodeErr != nil {
		t.Fatalf("failed to decode response: %v", decodeErr)
	}
	if payload["status"] != "accepted" {
		t.Fatalf("expected accepted status, got %q", payload["status"])
	}

	select {
	case restartRequest := <-restartChan:
		if restartRequest.RequestID != "req-restart" {
			t.Fatalf("expected request id req-restart, got %q", restartRequest.RequestID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected restart signal to be sent")
	}
}
