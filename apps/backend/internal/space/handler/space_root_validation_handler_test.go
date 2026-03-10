package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"taeu.kr/cohesion/internal/space"
)

type fakeCreateSpaceStore struct {
	spacesByName map[string]*space.Space
	createCalls  int
}

func (f *fakeCreateSpaceStore) GetAll(context.Context) ([]*space.Space, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeCreateSpaceStore) GetByName(_ context.Context, name string) (*space.Space, error) {
	item, ok := f.spacesByName[name]
	if !ok {
		return nil, errors.New("space not found")
	}
	return cloneSpace(item), nil
}

func (f *fakeCreateSpaceStore) GetByID(context.Context, int64) (*space.Space, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeCreateSpaceStore) Create(_ context.Context, req *space.CreateSpaceRequest) (*space.Space, error) {
	f.createCalls++
	return &space.Space{
		ID:        int64(f.createCalls),
		SpaceName: req.SpaceName,
		SpacePath: req.SpacePath,
	}, nil
}

func (f *fakeCreateSpaceStore) Delete(context.Context, int64) error {
	return errors.New("not implemented")
}

func TestHandleValidateSpaceRootReturnsStructuredResult(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filePath := filepath.Join(root, "note.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	tests := []struct {
		name          string
		path          string
		expectedCode  space.SpaceRootValidationCode
		expectedValid bool
	}{
		{
			name:          "valid directory",
			path:          root,
			expectedCode:  space.SpaceRootValidationCodeValid,
			expectedValid: true,
		},
		{
			name:          "missing path",
			path:          filepath.Join(root, "missing"),
			expectedCode:  space.SpaceRootValidationCodeNotFound,
			expectedValid: false,
		},
		{
			name:          "not a directory",
			path:          filePath,
			expectedCode:  space.SpaceRootValidationCodeNotDirectory,
			expectedValid: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handler := NewHandler(space.NewService(&fakeCreateSpaceStore{}), nil, nil)
			body := bytes.NewBufferString(`{"space_path":"` + tc.path + `"}`)
			req := httptest.NewRequest(http.MethodPost, "/api/spaces/validate-root", body)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			if webErr := handler.handleValidateSpaceRoot(rec, req); webErr != nil {
				t.Fatalf("unexpected web error: %+v", webErr)
			}
			if rec.Code != http.StatusOK {
				t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
			}

			var resp space.SpaceRootValidationResult
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if resp.Code != tc.expectedCode {
				t.Fatalf("expected code %q, got %q", tc.expectedCode, resp.Code)
			}
			if resp.Valid != tc.expectedValid {
				t.Fatalf("expected valid=%t, got %#v", tc.expectedValid, resp)
			}
		})
	}
}

func TestHandleValidateSpaceRootReturnsPermissionDeniedResult(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission-denied directory semantics differ on Windows")
	}

	root := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "child.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatalf("failed to create child file: %v", err)
	}
	if err := os.Chmod(root, 0o000); err != nil {
		t.Fatalf("failed to remove directory permissions: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(root, 0o755)
	})

	handler := NewHandler(space.NewService(&fakeCreateSpaceStore{}), nil, nil)
	body := bytes.NewBufferString(`{"space_path":"` + root + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/spaces/validate-root", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	if webErr := handler.handleValidateSpaceRoot(rec, req); webErr != nil {
		t.Fatalf("unexpected web error: %+v", webErr)
	}

	var resp space.SpaceRootValidationResult
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Valid {
		t.Fatalf("expected invalid result, got %#v", resp)
	}
	if resp.Code != space.SpaceRootValidationCodePermissionDenied {
		t.Fatalf("expected code %q, got %q", space.SpaceRootValidationCodePermissionDenied, resp.Code)
	}
}

func TestHandleCreateSpaceRevalidatesUnreadableRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission-denied directory semantics differ on Windows")
	}

	root := filepath.Join(t.TempDir(), "space-root")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "child.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("failed to create child file: %v", err)
	}

	store := &fakeCreateSpaceStore{
		spacesByName: map[string]*space.Space{},
	}
	service := space.NewService(store)

	result, err := service.ValidateSpaceRoot(context.Background(), root)
	if err != nil {
		t.Fatalf("expected preflight validation to succeed, got %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected preflight validation to be valid, got %#v", result)
	}

	if err := os.Chmod(root, 0o000); err != nil {
		t.Fatalf("failed to remove directory permissions: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(root, 0o755)
	})

	handler := NewHandler(service, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/spaces", bytes.NewBufferString(`{"space_name":"Docs","space_path":"`+root+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	if webErr := handler.handleCreateSpace(rec, req); webErr != nil {
		t.Fatalf("unexpected web error: %+v", webErr)
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if store.createCalls != 0 {
		t.Fatalf("expected store.Create not to be called, got %d", store.createCalls)
	}

	var resp struct {
		Error string                        `json:"error"`
		Code  space.SpaceRootValidationCode `json:"code"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != space.SpaceRootValidationCodePermissionDenied {
		t.Fatalf("expected code %q, got %q", space.SpaceRootValidationCodePermissionDenied, resp.Code)
	}
	if resp.Error == "" {
		t.Fatal("expected error message")
	}
}
