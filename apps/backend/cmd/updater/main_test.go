package main

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"taeu.kr/cohesion/internal/platform/logging"
	"taeu.kr/cohesion/internal/system"
)

func TestNewUpdaterLogger_MirroredWriter(t *testing.T) {
	var appBuffer bytes.Buffer
	var terminalBuffer bytes.Buffer

	logger := newUpdaterLogger(logging.NewMirroredWriter(&appBuffer, &terminalBuffer))
	logging.Event(logger.Info(), logging.ComponentUpdater, logging.EventBootStart).
		Int("pid", 1234).
		Msg("updater flow started")

	appLine := appBuffer.String()
	if !strings.Contains(appLine, "event=boot.start") {
		t.Fatalf("expected key=value event in updater file sink, got %q", appLine)
	}
	if !strings.Contains(appLine, "component=updater") {
		t.Fatalf("expected component field in updater file sink, got %q", appLine)
	}

	terminalLine := terminalBuffer.String()
	if !strings.Contains(terminalLine, "INFO [updater] boot.start - updater flow started") {
		t.Fatalf("expected terminal pattern output, got %q", terminalLine)
	}
	if strings.Contains(terminalLine, "event=boot.start") {
		t.Fatalf("expected pattern-style terminal output, got key=value output %q", terminalLine)
	}
}

func TestWaitForHealthyProcessSucceeds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/health":
			w.WriteHeader(http.StatusOK)
		case "/api/system/version":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"v0.5.16"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	exitCh := make(chan error, 1)
	if err := waitForReadyProcess(server.URL+"/api/health", server.URL+"/api/system/version", "v0.5.16", 2*time.Second, exitCh); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestWaitForHealthyProcessReturnsExitError(t *testing.T) {
	exitCh := make(chan error, 1)
	exitCh <- errors.New("process exited")

	err := waitForReadyProcess("http://127.0.0.1:1/api/health", "http://127.0.0.1:1/api/system/version", "v0.5.16", time.Second, exitCh)
	if err == nil {
		t.Fatal("expected process exit error")
	}
	if !strings.Contains(err.Error(), "process exited") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWaitForHealthyProcessTimesOut(t *testing.T) {
	exitCh := make(chan error, 1)

	err := waitForReadyProcess("http://127.0.0.1:1/api/health", "http://127.0.0.1:1/api/system/version", "v0.5.16", 1200*time.Millisecond, exitCh)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "readiness check did not succeed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProbeVersionEndpointRejectsUnexpectedVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"v0.5.15"}`))
	}))
	defer server.Close()

	err := probeVersionEndpoint(server.URL, "v0.5.16")
	if err == nil {
		t.Fatal("expected version mismatch error")
	}
	if !strings.Contains(err.Error(), "unexpected version") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRollbackAndRestartPreviousPersistsFailureWhenRollbackFails(t *testing.T) {
	statusPath := filepath.Join(t.TempDir(), "system-status.json")
	t.Setenv("COHESION_SYSTEM_STATUS_PATH", statusPath)

	store := system.NewStatusStore()
	logger := newUpdaterLogger(io.Discard)
	cause := errors.New("replacement start failed")

	err := rollbackAndRestartPrevious(
		updaterArgs{
			target:     filepath.Join(t.TempDir(), "app"),
			healthURL:  "http://127.0.0.1:1/api/health",
			versionURL: "http://127.0.0.1:1/api/system/version",
		},
		filepath.Join(t.TempDir(), "missing.bak"),
		nil,
		filepath.Join(t.TempDir(), "app.log"),
		logger,
		store,
		cause,
	)
	if err == nil {
		t.Fatal("expected rollback failure error")
	}

	status, loadErr := store.Load()
	if loadErr != nil && !errors.Is(loadErr, os.ErrNotExist) {
		t.Fatalf("failed to load persisted status: %v", loadErr)
	}
	if status.State != "failed" {
		t.Fatalf("expected failed state, got %q", status.State)
	}
	if status.RuntimeState != "failed" {
		t.Fatalf("expected failed runtime state, got %q", status.RuntimeState)
	}
}
