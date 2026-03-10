package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"taeu.kr/cohesion/internal/space"
)

func TestStorageAccessWebError_ClassifiesOperationNotPermittedAsForbidden(t *testing.T) {
	err := &os.PathError{
		Op:   "stat",
		Path: "/Users/tester/Downloads",
		Err:  errors.New("operation not permitted"),
	}

	webErr := storageAccessWebError(err, "File not found", "Failed to access file")
	if webErr == nil {
		t.Fatal("expected web error")
	}
	if webErr.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden status, got %d", webErr.Code)
	}
	if webErr.Message != "Permission denied" {
		t.Fatalf("expected permission denied message, got %q", webErr.Message)
	}
}

func TestSafeFilesystemReason_DoesNotLeakAbsolutePaths(t *testing.T) {
	err := &os.PathError{
		Op:   "rename",
		Path: "/Users/tester/private/secret.txt",
		Err:  errors.New("permission denied"),
	}

	reason := safeFilesystemReason("Failed to move", err)
	if reason != "Failed to move: permission denied" {
		t.Fatalf("unexpected sanitized reason: %q", reason)
	}
	if strings.Contains(reason, "/Users/tester/private/secret.txt") {
		t.Fatalf("expected absolute path to be omitted, got %q", reason)
	}
}

func TestHandleFileDownload_FailsWhenDirectoryArchiveContainsUnreadablePath(t *testing.T) {
	spaceRoot := t.TempDir()
	docsDir := filepath.Join(spaceRoot, "docs")
	blockedDir := filepath.Join(docsDir, "blocked")
	if err := os.MkdirAll(blockedDir, 0o755); err != nil {
		t.Fatalf("failed to create blocked directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(blockedDir, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatalf("failed to create blocked file: %v", err)
	}
	if err := os.Chmod(blockedDir, 0o000); err != nil {
		t.Fatalf("failed to block directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(blockedDir, 0o755)
	})

	store := &fakeTransferSpaceStore{
		spacesByID: map[int64]*space.Space{
			1: {ID: 1, SpaceName: "Transfer", SpacePath: spaceRoot},
		},
	}
	handler := NewHandler(space.NewService(store), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/spaces/1/files/download?path=docs", nil)
	rec := httptest.NewRecorder()
	webErr := handler.handleFileDownload(rec, req, 1)
	if webErr == nil {
		t.Fatal("expected zip creation to fail")
	}
	if webErr.Message != "Failed to create zip archive" {
		t.Fatalf("unexpected web error message: %q", webErr.Message)
	}
}

func TestHandleFileDownloadMultipleTicket_FailsWhenZipEntryCannotBeAdded(t *testing.T) {
	spaceRoot := t.TempDir()
	docsDir := filepath.Join(spaceRoot, "docs")
	blockedDir := filepath.Join(docsDir, "blocked")
	if err := os.MkdirAll(blockedDir, 0o755); err != nil {
		t.Fatalf("failed to create blocked directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(blockedDir, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatalf("failed to create blocked file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(spaceRoot, "report.txt"), []byte("report"), 0o644); err != nil {
		t.Fatalf("failed to create report file: %v", err)
	}
	if err := os.Chmod(blockedDir, 0o000); err != nil {
		t.Fatalf("failed to block directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(blockedDir, 0o755)
	})

	store := &fakeTransferSpaceStore{
		spacesByID: map[int64]*space.Space{
			1: {ID: 1, SpaceName: "Transfer", SpacePath: spaceRoot},
		},
	}
	handler := NewHandler(space.NewService(store), nil, nil)

	body, err := json.Marshal(map[string]any{
		"paths": []string{"docs", "report.txt"},
	})
	if err != nil {
		t.Fatalf("failed to marshal ticket request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/spaces/1/files/download-multiple-ticket", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withClaims(req, "tester")
	rec := httptest.NewRecorder()
	webErr := handler.handleFileDownloadMultipleTicket(rec, req, 1)
	if webErr == nil {
		t.Fatal("expected multi-ticket zip creation to fail")
	}
	if webErr.Message != "Failed to create zip archive" {
		t.Fatalf("unexpected web error message: %q", webErr.Message)
	}
}

func TestHandleFileCopy_SanitizesPermissionDeniedFailureReason(t *testing.T) {
	spaceRoot := t.TempDir()
	srcDir := filepath.Join(spaceRoot, "src")
	dstDir := filepath.Join(spaceRoot, "dst")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		t.Fatalf("failed to create destination dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("from-src"), 0o644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}
	if err := os.Chmod(dstDir, 0o000); err != nil {
		t.Fatalf("failed to block destination dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(dstDir, 0o755)
	})

	store := &fakeUploadSpaceStore{
		spacesByID: map[int64]*space.Space{
			1: {ID: 1, SpaceName: "Transfer", SpacePath: spaceRoot},
		},
	}
	handler := NewHandler(space.NewService(store), nil, &allowAllSpaceAccessService{})

	req := newTransferRequest(t, "copy", "", 1, "src/a.txt", "dst")
	rec := httptest.NewRecorder()
	webErr := handler.handleFileCopy(rec, req, 1)
	if webErr != nil {
		t.Fatalf("unexpected web error: %+v", webErr)
	}

	payload := decodeTransferResponse(t, rec)
	if len(payload.Failed) != 1 {
		t.Fatalf("expected one failed item, got %+v", payload.Failed)
	}
	reason := payload.Failed[0].Reason
	if !strings.Contains(reason, "permission denied") {
		t.Fatalf("expected permission guidance in reason, got %q", reason)
	}
	if strings.Contains(reason, spaceRoot) {
		t.Fatalf("expected absolute path to be omitted, got %q", reason)
	}
}
