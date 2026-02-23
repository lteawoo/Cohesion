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
	"testing"

	"taeu.kr/cohesion/internal/space"
)

type fakeQuotaSpaceStore struct {
	spacesByID map[int64]*space.Space
}

func (f *fakeQuotaSpaceStore) GetAll(ctx context.Context) ([]*space.Space, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeQuotaSpaceStore) GetByName(ctx context.Context, name string) (*space.Space, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeQuotaSpaceStore) GetByID(ctx context.Context, id int64) (*space.Space, error) {
	spaceData, ok := f.spacesByID[id]
	if !ok {
		return nil, errors.New("space not found")
	}
	return spaceData, nil
}

func (f *fakeQuotaSpaceStore) Create(ctx context.Context, req *space.CreateSpaceRequest) (*space.Space, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeQuotaSpaceStore) Delete(ctx context.Context, id int64) error {
	return errors.New("not implemented")
}

func TestHandleFileUpload_BlockWhenQuotaExceeded(t *testing.T) {
	spaceRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(spaceRoot, "seed.bin"), []byte("12345"), 0o644); err != nil {
		t.Fatalf("failed to prepare seed file: %v", err)
	}

	quota := int64(5)
	store := &fakeQuotaSpaceStore{
		spacesByID: map[int64]*space.Space{
			1: {
				ID:         1,
				SpaceName:  "Quota",
				SpacePath:  spaceRoot,
				QuotaBytes: &quota,
			},
		},
	}

	handler := NewHandler(space.NewService(store), nil, nil)
	req := newUploadRequest(t, "new.bin", "x", nil)
	rec := httptest.NewRecorder()

	webErr := handler.handleFileUpload(rec, req, 1)
	if webErr == nil {
		t.Fatal("expected quota exceeded error")
	}
	if webErr.Code != http.StatusInsufficientStorage {
		t.Fatalf("expected status %d, got %d", http.StatusInsufficientStorage, webErr.Code)
	}

	if _, err := os.Stat(filepath.Join(spaceRoot, "new.bin")); !os.IsNotExist(err) {
		t.Fatalf("upload target file should not be created when quota exceeded, err=%v", err)
	}
}

func TestHandleFileCopy_FailedWithQuotaExceededCode(t *testing.T) {
	spaceRoot := t.TempDir()
	srcDir := filepath.Join(spaceRoot, "src")
	dstDir := filepath.Join(spaceRoot, "dst")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		t.Fatalf("failed to create dst dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("12345"), 0o644); err != nil {
		t.Fatalf("failed to create src file: %v", err)
	}

	quota := int64(5)
	store := &fakeQuotaSpaceStore{
		spacesByID: map[int64]*space.Space{
			1: {
				ID:         1,
				SpaceName:  "Quota",
				SpacePath:  spaceRoot,
				QuotaBytes: &quota,
			},
		},
	}

	handler := NewHandler(space.NewService(store), nil, &allowAllSpaceAccessService{})

	payload := map[string]interface{}{
		"sources": []string{"src/a.txt"},
		"destination": map[string]interface{}{
			"spaceId": 1,
			"path":    "dst",
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/spaces/1/files/copy", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withClaims(req, "tester")

	rec := httptest.NewRecorder()
	webErr := handler.handleFileCopy(rec, req, 1)
	if webErr != nil {
		t.Fatalf("unexpected web error: %+v", webErr)
	}

	resp := decodeTransferResponse(t, rec)
	if len(resp.Succeeded) != 0 {
		t.Fatalf("expected no success, got %v", resp.Succeeded)
	}
	if len(resp.Failed) != 1 {
		t.Fatalf("expected one failed item, got %+v", resp.Failed)
	}
	if resp.Failed[0].Code != fileConflictCodeQuotaExceeded {
		t.Fatalf("expected failed code %q, got %+v", fileConflictCodeQuotaExceeded, resp.Failed[0])
	}

	if _, err := os.Stat(filepath.Join(dstDir, "a.txt")); !os.IsNotExist(err) {
		t.Fatalf("copied file should not exist when quota exceeded, err=%v", err)
	}
}
