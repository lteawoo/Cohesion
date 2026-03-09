package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"taeu.kr/cohesion/internal/space"
)

type fakeUploadSpaceStore struct {
	spacesByID map[int64]*space.Space
}

func (f *fakeUploadSpaceStore) GetAll(ctx context.Context) ([]*space.Space, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeUploadSpaceStore) GetByName(ctx context.Context, name string) (*space.Space, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeUploadSpaceStore) GetByID(ctx context.Context, id int64) (*space.Space, error) {
	spaceData, ok := f.spacesByID[id]
	if !ok {
		return nil, errors.New("space not found")
	}
	return spaceData, nil
}

func (f *fakeUploadSpaceStore) Create(ctx context.Context, req *space.CreateSpaceRequest) (*space.Space, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeUploadSpaceStore) Delete(ctx context.Context, id int64) error {
	return errors.New("not implemented")
}

func newUploadRequest(t *testing.T, fileName, fileContent string, extraFields map[string]string) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("path", ""); err != nil {
		t.Fatalf("failed to write path field: %v", err)
	}
	if err := writer.WriteField("size", strconv.Itoa(len(fileContent))); err != nil {
		t.Fatalf("failed to write size field: %v", err)
	}
	for key, value := range extraFields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("failed to write field %s: %v", key, err)
		}
	}
	formFile, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	if _, err := io.WriteString(formFile, fileContent); err != nil {
		t.Fatalf("failed to write form file content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/spaces/1/files/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func decodeUploadResponse(t *testing.T, rec *httptest.ResponseRecorder) map[string]string {
	t.Helper()

	var payload map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode upload response: %v", err)
	}
	return payload
}

type failingUploadReader struct {
	data      []byte
	failAfter int
	offset    int
}

func (r *failingUploadReader) Read(p []byte) (int, error) {
	if r.offset >= len(r.data) {
		return 0, io.EOF
	}

	remainingUntilFailure := r.failAfter - r.offset
	if remainingUntilFailure <= 0 {
		return 0, errors.New("forced upload stream failure")
	}

	n := remainingUntilFailure
	if n > len(p) {
		n = len(p)
	}
	if remainingData := len(r.data) - r.offset; n > remainingData {
		n = remainingData
	}
	copy(p[:n], r.data[r.offset:r.offset+n])
	r.offset += n
	if r.offset >= r.failAfter {
		return n, errors.New("forced upload stream failure")
	}
	return n, nil
}

func newFailingUploadRequest(t *testing.T, fileName, fileContent string, failAfter int) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("path", ""); err != nil {
		t.Fatalf("failed to write path field: %v", err)
	}
	if err := writer.WriteField("size", strconv.Itoa(len(fileContent))); err != nil {
		t.Fatalf("failed to write size field: %v", err)
	}
	formFile, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	if _, err := io.WriteString(formFile, fileContent); err != nil {
		t.Fatalf("failed to write form file content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	reader := &failingUploadReader{
		data:      body.Bytes(),
		failAfter: failAfter,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/spaces/1/files/upload", reader)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestHandleFileUpload_ConflictPolicies(t *testing.T) {
	setup := func(t *testing.T) (*Handler, string) {
		t.Helper()

		spaceRoot := t.TempDir()
		store := &fakeUploadSpaceStore{
			spacesByID: map[int64]*space.Space{
				1: {
					ID:        1,
					SpaceName: "Upload",
					SpacePath: spaceRoot,
				},
			},
		}
		handler := NewHandler(space.NewService(store), nil, nil)
		return handler, spaceRoot
	}

	t.Run("returns conflict when policy is not specified", func(t *testing.T) {
		handler, root := setup(t)
		existingPath := filepath.Join(root, "report.txt")
		if err := os.WriteFile(existingPath, []byte("old"), 0o644); err != nil {
			t.Fatalf("failed to prepare existing file: %v", err)
		}

		req := newUploadRequest(t, "report.txt", "new", nil)
		rec := httptest.NewRecorder()
		webErr := handler.handleFileUpload(rec, req, 1)
		if webErr == nil {
			t.Fatal("expected conflict error")
		}
		if webErr.Code != http.StatusConflict {
			t.Fatalf("expected status %d, got %d", http.StatusConflict, webErr.Code)
		}

		got, err := os.ReadFile(existingPath)
		if err != nil {
			t.Fatalf("failed to read existing file: %v", err)
		}
		if string(got) != "old" {
			t.Fatalf("expected existing file content to remain old, got %q", string(got))
		}
	})

	t.Run("overwrites when conflictPolicy=overwrite", func(t *testing.T) {
		handler, root := setup(t)
		existingPath := filepath.Join(root, "report.txt")
		if err := os.WriteFile(existingPath, []byte("old"), 0o644); err != nil {
			t.Fatalf("failed to prepare existing file: %v", err)
		}

		req := newUploadRequest(t, "report.txt", "new-overwrite", map[string]string{"conflictPolicy": "overwrite"})
		rec := httptest.NewRecorder()
		webErr := handler.handleFileUpload(rec, req, 1)
		if webErr != nil {
			t.Fatalf("expected success, got %+v", webErr)
		}

		payload := decodeUploadResponse(t, rec)
		if payload["status"] != "uploaded" {
			t.Fatalf("expected uploaded status, got %q", payload["status"])
		}
		if payload["filename"] != "report.txt" {
			t.Fatalf("expected filename report.txt, got %q", payload["filename"])
		}

		got, err := os.ReadFile(existingPath)
		if err != nil {
			t.Fatalf("failed to read overwritten file: %v", err)
		}
		if string(got) != "new-overwrite" {
			t.Fatalf("expected overwritten content, got %q", string(got))
		}
	})

	t.Run("renames when conflictPolicy=rename", func(t *testing.T) {
		handler, root := setup(t)
		existingPath := filepath.Join(root, "report.txt")
		if err := os.WriteFile(existingPath, []byte("old"), 0o644); err != nil {
			t.Fatalf("failed to prepare existing file: %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, "report (1).txt"), []byte("old-1"), 0o644); err != nil {
			t.Fatalf("failed to prepare existing renamed file: %v", err)
		}

		req := newUploadRequest(t, "report.txt", "new-rename", map[string]string{"conflictPolicy": "rename"})
		rec := httptest.NewRecorder()
		webErr := handler.handleFileUpload(rec, req, 1)
		if webErr != nil {
			t.Fatalf("expected success, got %+v", webErr)
		}

		payload := decodeUploadResponse(t, rec)
		if payload["status"] != "uploaded" {
			t.Fatalf("expected uploaded status, got %q", payload["status"])
		}
		if payload["filename"] != "report (2).txt" {
			t.Fatalf("expected filename report (2).txt, got %q", payload["filename"])
		}

		original, err := os.ReadFile(existingPath)
		if err != nil {
			t.Fatalf("failed to read original file: %v", err)
		}
		if string(original) != "old" {
			t.Fatalf("expected original file content old, got %q", string(original))
		}

		renamedPath := filepath.Join(root, "report (2).txt")
		renamed, err := os.ReadFile(renamedPath)
		if err != nil {
			t.Fatalf("failed to read renamed file: %v", err)
		}
		if string(renamed) != "new-rename" {
			t.Fatalf("expected renamed file content new-rename, got %q", string(renamed))
		}
	})

	t.Run("skips when conflictPolicy=skip", func(t *testing.T) {
		handler, root := setup(t)
		existingPath := filepath.Join(root, "report.txt")
		if err := os.WriteFile(existingPath, []byte("old"), 0o644); err != nil {
			t.Fatalf("failed to prepare existing file: %v", err)
		}

		req := newUploadRequest(t, "report.txt", "new-skip", map[string]string{"conflictPolicy": "skip"})
		rec := httptest.NewRecorder()
		webErr := handler.handleFileUpload(rec, req, 1)
		if webErr != nil {
			t.Fatalf("expected success, got %+v", webErr)
		}

		payload := decodeUploadResponse(t, rec)
		if payload["status"] != "skipped" {
			t.Fatalf("expected skipped status, got %q", payload["status"])
		}
		if payload["filename"] != "report.txt" {
			t.Fatalf("expected filename report.txt, got %q", payload["filename"])
		}

		got, err := os.ReadFile(existingPath)
		if err != nil {
			t.Fatalf("failed to read existing file: %v", err)
		}
		if string(got) != "old" {
			t.Fatalf("expected existing content to remain old, got %q", string(got))
		}
	})

	t.Run("supports legacy overwrite=true", func(t *testing.T) {
		handler, root := setup(t)
		existingPath := filepath.Join(root, "report.txt")
		if err := os.WriteFile(existingPath, []byte("old"), 0o644); err != nil {
			t.Fatalf("failed to prepare existing file: %v", err)
		}

		req := newUploadRequest(t, "report.txt", "new-legacy", map[string]string{"overwrite": "true"})
		rec := httptest.NewRecorder()
		webErr := handler.handleFileUpload(rec, req, 1)
		if webErr != nil {
			t.Fatalf("expected success, got %+v", webErr)
		}

		payload := decodeUploadResponse(t, rec)
		if payload["status"] != "uploaded" {
			t.Fatalf("expected uploaded status, got %q", payload["status"])
		}

		got, err := os.ReadFile(existingPath)
		if err != nil {
			t.Fatalf("failed to read overwritten file: %v", err)
		}
		if string(got) != "new-legacy" {
			t.Fatalf("expected overwritten content new-legacy, got %q", string(got))
		}
	})

	t.Run("cleans up staged file when upload stream fails", func(t *testing.T) {
		handler, root := setup(t)

		req := newFailingUploadRequest(t, "broken.bin", strings.Repeat("x", 512), 256)
		rec := httptest.NewRecorder()
		webErr := handler.handleFileUpload(rec, req, 1)
		if webErr == nil {
			t.Fatal("expected upload failure")
		}

		if _, err := os.Stat(filepath.Join(root, "broken.bin")); !os.IsNotExist(err) {
			t.Fatalf("destination file should not exist after interrupted upload, err=%v", err)
		}

		stageMatches, err := filepath.Glob(filepath.Join(root, ".broken.bin.cohesion-upload-*"))
		if err != nil {
			t.Fatalf("failed to inspect staged files: %v", err)
		}
		if len(stageMatches) != 0 {
			t.Fatalf("expected staged files to be removed, found %v", stageMatches)
		}
	})
}
