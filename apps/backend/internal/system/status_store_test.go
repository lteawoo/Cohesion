package system

import (
	"path/filepath"
	"testing"
)

func setupStatusStore(t *testing.T) *StatusStore {
	t.Helper()

	t.Setenv(lifecycleStatusPathEnv, filepath.Join(t.TempDir(), "system-status.json"))
	return NewStatusStore()
}

func TestStatusStoreMarksRestartLifecycle(t *testing.T) {
	store := setupStatusStore(t)

	status, err := store.MarkRestartAccepted(RestartRequest{
		Actor:     "admin",
		RequestID: "req-1",
		Port:      "3000",
	}, "v0.5.15")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if status.State != "restarting" {
		t.Fatalf("expected restarting state, got %q", status.State)
	}
	if status.RuntimeState != "restarting" {
		t.Fatalf("expected restarting runtime state, got %q", status.RuntimeState)
	}

	status, err = store.MarkServerReady("v0.5.15")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if status.State != "succeeded" {
		t.Fatalf("expected succeeded state, got %q", status.State)
	}
	if status.RuntimeState != "healthy" {
		t.Fatalf("expected healthy runtime state, got %q", status.RuntimeState)
	}
}

func TestStatusStorePersistsRestartFailureAcrossBoot(t *testing.T) {
	store := setupStatusStore(t)

	if _, err := store.MarkRestartAccepted(RestartRequest{Port: "3000"}, "v0.5.15"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	status, err := store.MarkRestartFailed(assertionError("boot failed"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if status.State != "failed" {
		t.Fatalf("expected failed state, got %q", status.State)
	}
	if status.RuntimeState != "failed" {
		t.Fatalf("expected failed runtime state, got %q", status.RuntimeState)
	}

	status, err = store.MarkServerReady("v0.5.15")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if status.State != "failed" {
		t.Fatalf("expected failed transition state after boot, got %q", status.State)
	}
	if status.RuntimeState != "healthy" {
		t.Fatalf("expected healthy runtime state after boot, got %q", status.RuntimeState)
	}
}

func TestStatusStoreMarksUpdateResult(t *testing.T) {
	store := setupStatusStore(t)

	if err := store.Save(SelfUpdateStatus{
		State:          "switching",
		Operation:      "update",
		CurrentVersion: "v0.5.15",
		TargetVersion:  "v0.5.16",
		StartedAt:      "2026-03-07T00:00:00Z",
		RuntimeState:   "updating",
		RuntimeMessage: "새 버전으로 전환 중입니다",
	}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	status, err := store.MarkUpdateSucceeded("v0.5.16")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if status.State != "succeeded" {
		t.Fatalf("expected succeeded state, got %q", status.State)
	}
	if status.CurrentVersion != "v0.5.16" {
		t.Fatalf("expected currentVersion v0.5.16, got %q", status.CurrentVersion)
	}
	if status.RuntimeState != "healthy" {
		t.Fatalf("expected healthy runtime state, got %q", status.RuntimeState)
	}
}

type assertionError string

func (e assertionError) Error() string {
	return string(e)
}
