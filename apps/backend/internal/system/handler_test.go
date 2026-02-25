package system

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGetVersion(t *testing.T) {
	handler := NewHandler(make(chan bool, 1), make(chan struct{}, 1), Meta{
		Version:   "v0.3.0",
		Commit:    "abc123",
		BuildDate: "2026-02-24T00:00:00Z",
	})

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

	handler := NewHandler(make(chan bool, 1), make(chan struct{}, 1), Meta{
		Version: "v0.3.0",
	})
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
	handler := NewHandler(make(chan bool, 1), make(chan struct{}, 1), Meta{
		Version: "v0.3.0",
	})

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
}

func TestStartUpdateReturnsBadRequestOnDevBuild(t *testing.T) {
	handler := NewHandler(make(chan bool, 1), make(chan struct{}, 1), Meta{
		Version: "dev",
	})

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
