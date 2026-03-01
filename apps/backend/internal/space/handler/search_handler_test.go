package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"taeu.kr/cohesion/internal/account"
	"taeu.kr/cohesion/internal/auth"
	"taeu.kr/cohesion/internal/space"
)

type fakeSearchSpaceStore struct {
	spaces []*space.Space
	err    error
}

func (f *fakeSearchSpaceStore) GetAll(ctx context.Context) ([]*space.Space, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.spaces, nil
}

func (f *fakeSearchSpaceStore) GetByName(ctx context.Context, name string) (*space.Space, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeSearchSpaceStore) GetByID(ctx context.Context, id int64) (*space.Space, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeSearchSpaceStore) Create(ctx context.Context, req *space.CreateSpaceRequest) (*space.Space, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeSearchSpaceStore) Delete(ctx context.Context, id int64) error {
	return errors.New("not implemented")
}

type fakeSearchSpaceAccessService struct {
	allowedBySpace map[int64]bool
	err            error
}

func (f *fakeSearchSpaceAccessService) CanAccessSpaceByID(ctx context.Context, username string, spaceID int64, required account.Permission) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	allowed, ok := f.allowedBySpace[spaceID]
	return ok && allowed, nil
}

func withClaims(req *http.Request, username string) *http.Request {
	return req.WithContext(auth.WithClaims(req.Context(), &auth.Claims{
		Username: username,
	}))
}

func TestHandleSearchFiles_UnauthorizedWithoutClaims(t *testing.T) {
	store := &fakeSearchSpaceStore{}
	h := NewHandler(space.NewService(store), nil, &fakeSearchSpaceAccessService{})

	req := httptest.NewRequest(http.MethodGet, "/api/search/files?q=report", nil)
	rec := httptest.NewRecorder()

	webErr := h.handleSearchFiles(rec, req)
	if webErr == nil {
		t.Fatal("expected unauthorized error")
	}
	if webErr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, webErr.Code)
	}
}

func TestHandleSearchFiles_ReturnsOnlyReadableSpaceResults(t *testing.T) {
	rootOne := t.TempDir()
	rootTwo := t.TempDir()

	if err := os.MkdirAll(filepath.Join(rootOne, "docs"), 0o755); err != nil {
		t.Fatalf("failed to create docs dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootOne, "report.md"), []byte("a"), 0o644); err != nil {
		t.Fatalf("failed to create report.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootOne, "docs", "report-plan.txt"), []byte("b"), 0o644); err != nil {
		t.Fatalf("failed to create report-plan.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootOne, ".report-hidden.txt"), []byte("hidden"), 0o644); err != nil {
		t.Fatalf("failed to create hidden report file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootTwo, "report-secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatalf("failed to create report-secret.txt: %v", err)
	}

	store := &fakeSearchSpaceStore{
		spaces: []*space.Space{
			{ID: 11, SpaceName: "Design", SpacePath: rootOne},
			{ID: 22, SpaceName: "Secret", SpacePath: rootTwo},
		},
	}
	access := &fakeSearchSpaceAccessService{
		allowedBySpace: map[int64]bool{
			11: true,
			22: false,
		},
	}
	h := NewHandler(space.NewService(store), nil, access)

	req := httptest.NewRequest(http.MethodGet, "/api/search/files?q=report&limit=10", nil)
	req = withClaims(req, "tester")
	rec := httptest.NewRecorder()

	webErr := h.handleSearchFiles(rec, req)
	if webErr != nil {
		t.Fatalf("expected no error, got %+v", webErr)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var got []fileSearchResult
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}

	for _, item := range got {
		if item.SpaceID != 11 {
			t.Fatalf("expected only space 11 results, got space %d", item.SpaceID)
		}
		if item.Path == ".report-hidden.txt" {
			t.Fatal("hidden file must be excluded from search results")
		}
	}
}

func TestHandleSearchFiles_InvalidLimit(t *testing.T) {
	store := &fakeSearchSpaceStore{}
	h := NewHandler(space.NewService(store), nil, &fakeSearchSpaceAccessService{})

	req := httptest.NewRequest(http.MethodGet, "/api/search/files?q=abc&limit=oops", nil)
	req = withClaims(req, "tester")
	rec := httptest.NewRecorder()

	webErr := h.handleSearchFiles(rec, req)
	if webErr == nil {
		t.Fatal("expected bad request error")
	}
	if webErr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, webErr.Code)
	}
}

func TestHandleSearchFiles_ExcludesPathOnlyMatches(t *testing.T) {
	root := t.TempDir()

	if err := os.MkdirAll(filepath.Join(root, "project-report"), 0o755); err != nil {
		t.Fatalf("failed to create report directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "project-report", "notes.txt"), []byte("memo"), 0o644); err != nil {
		t.Fatalf("failed to create notes.txt: %v", err)
	}

	store := &fakeSearchSpaceStore{
		spaces: []*space.Space{
			{ID: 11, SpaceName: "Design", SpacePath: root},
		},
	}
	access := &fakeSearchSpaceAccessService{
		allowedBySpace: map[int64]bool{
			11: true,
		},
	}
	h := NewHandler(space.NewService(store), nil, access)

	req := httptest.NewRequest(http.MethodGet, "/api/search/files?q=report&limit=10", nil)
	req = withClaims(req, "tester")
	rec := httptest.NewRecorder()

	webErr := h.handleSearchFiles(rec, req)
	if webErr != nil {
		t.Fatalf("expected no error, got %+v", webErr)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var got []fileSearchResult
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(got) == 0 {
		t.Fatal("expected at least one name-match result")
	}

	for _, item := range got {
		if item.Path == "project-report/notes.txt" {
			t.Fatal("path-only match must be excluded from search results")
		}
	}
}
