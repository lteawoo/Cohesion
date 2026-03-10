package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"taeu.kr/cohesion/internal/browse"
	"taeu.kr/cohesion/internal/space"
)

type permissionBrowseService struct{}

func (permissionBrowseService) ListDirectory(_ bool, _ string) ([]browse.FileInfo, error) {
	return nil, fmt.Errorf(
		"fail to read directory: %w",
		&os.PathError{Op: "open", Path: "/Users/twlee/Downloads", Err: syscall.EPERM},
	)
}

func TestHandleSpaceBrowseReturnsForbiddenForWrappedPermissionErrors(t *testing.T) {
	t.Parallel()

	spaceRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(spaceRoot, "root"), 0o755); err != nil {
		t.Fatalf("failed to prepare space root: %v", err)
	}

	store := &fakeTransferSpaceStore{
		spacesByID: map[int64]*space.Space{
			1: {
				ID:        1,
				SpaceName: "Space",
				SpacePath: filepath.Join(spaceRoot, "root"),
			},
		},
	}
	handler := NewHandler(space.NewService(store), permissionBrowseService{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/spaces/1/browse?path=", nil)
	rec := httptest.NewRecorder()

	webErr := handler.handleSpaceBrowse(rec, req, 1)
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
