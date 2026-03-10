package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"taeu.kr/cohesion/internal/browse"
)

type fakeBrowseService struct {
	baseDirectories   []browse.FileInfo
	initialBrowseRoot string
	listDirectory     func(onlyDir bool, path string) ([]browse.FileInfo, error)
}

func (f *fakeBrowseService) GetBaseDirectories() []browse.FileInfo {
	return f.baseDirectories
}

func (f *fakeBrowseService) GetInitialBrowseRoot() string {
	return f.initialBrowseRoot
}

func (f *fakeBrowseService) ListDirectory(onlyDir bool, path string) ([]browse.FileInfo, error) {
	if f.listDirectory != nil {
		return f.listDirectory(onlyDir, path)
	}
	return nil, nil
}

func TestHandleBrowseTreatsRequestsAsSystemBrowse(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	visibleDir := filepath.Join(tempDir, "visible")
	if err := os.Mkdir(visibleDir, 0o755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	tests := []struct {
		name     string
		rawQuery string
	}{
		{
			name:     "without system query",
			rawQuery: "path=" + url.QueryEscape(tempDir),
		},
		{
			name:     "with legacy system query",
			rawQuery: "path=" + url.QueryEscape(tempDir) + "&system=true",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := NewHandler(browse.NewService(), nil)
			req := httptest.NewRequest(http.MethodGet, "/api/browse?"+tc.rawQuery, nil)
			rec := httptest.NewRecorder()

			if webErr := h.handleBrowse(rec, req); webErr != nil {
				t.Fatalf("expected no error, got %+v", webErr)
			}
			if rec.Code != http.StatusOK {
				t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
			}

			var payload []browse.FileInfo
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if len(payload) == 0 {
				t.Fatalf("expected at least one entry in response, got %d", len(payload))
			}
		})
	}
}

func TestHandleBrowseReturnsForbiddenForWrappedPermissionErrors(t *testing.T) {
	t.Parallel()

	h := NewHandler(&fakeBrowseService{
		initialBrowseRoot: "/Users/twlee/Downloads",
		listDirectory: func(_ bool, _ string) ([]browse.FileInfo, error) {
			return nil, fmt.Errorf(
				"fail to read directory: %w",
				&os.PathError{Op: "open", Path: "/Users/twlee/Downloads", Err: syscall.EPERM},
			)
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/browse", nil)
	rec := httptest.NewRecorder()

	webErr := h.handleBrowse(rec, req)
	if webErr == nil {
		t.Fatal("expected browse error")
	}
	if webErr.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, webErr.Code)
	}
	if webErr.Message != "Permission denied" {
		t.Fatalf("expected permission denied message, got %q", webErr.Message)
	}
}
