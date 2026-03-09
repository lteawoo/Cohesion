package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"taeu.kr/cohesion/internal/space"
)

type fakeTransferSpaceStore struct {
	spacesByID map[int64]*space.Space
}

func (f *fakeTransferSpaceStore) GetAll(ctx context.Context) ([]*space.Space, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeTransferSpaceStore) GetByName(ctx context.Context, name string) (*space.Space, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeTransferSpaceStore) GetByID(ctx context.Context, id int64) (*space.Space, error) {
	spaceData, ok := f.spacesByID[id]
	if !ok {
		return nil, errors.New("space not found")
	}
	return spaceData, nil
}

func (f *fakeTransferSpaceStore) Create(ctx context.Context, req *space.CreateSpaceRequest) (*space.Space, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeTransferSpaceStore) Delete(ctx context.Context, id int64) error {
	return errors.New("not implemented")
}

func TestHandleFileDownload_SupportsRangeRequests(t *testing.T) {
	spaceRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(spaceRoot, "report.bin"), []byte("0123456789"), 0o644); err != nil {
		t.Fatalf("failed to seed download file: %v", err)
	}

	store := &fakeTransferSpaceStore{
		spacesByID: map[int64]*space.Space{
			1: {
				ID:        1,
				SpaceName: "Transfer",
				SpacePath: spaceRoot,
			},
		},
	}
	handler := NewHandler(space.NewService(store), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/spaces/1/files/download?path=report.bin", nil)
	req.Header.Set("Range", "bytes=2-5")
	rec := httptest.NewRecorder()

	webErr := handler.handleFileDownload(rec, req, 1)
	if webErr != nil {
		t.Fatalf("unexpected web error: %+v", webErr)
	}

	resp := rec.Result()
	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("expected status %d, got %d", http.StatusPartialContent, resp.StatusCode)
	}
	if contentRange := resp.Header.Get("Content-Range"); contentRange != "bytes 2-5/10" {
		t.Fatalf("unexpected content-range: %q", contentRange)
	}
	body := rec.Body.String()
	if body != "2345" {
		t.Fatalf("expected partial body %q, got %q", "2345", body)
	}
}

func TestHandleDownloadByTicket_SupportsRangeRequests(t *testing.T) {
	spaceRoot := t.TempDir()
	filePath := filepath.Join(spaceRoot, "report.bin")
	if err := os.WriteFile(filePath, []byte("abcdefghij"), 0o644); err != nil {
		t.Fatalf("failed to seed ticket file: %v", err)
	}

	store := &fakeTransferSpaceStore{
		spacesByID: map[int64]*space.Space{
			1: {
				ID:        1,
				SpaceName: "Transfer",
				SpacePath: spaceRoot,
			},
		},
	}
	handler := NewHandler(space.NewService(store), nil, nil)

	spaceID := int64(1)
	ticket, err := handler.issueDownloadTicket("tester", filePath, "report.bin", "file.download-ticket", &spaceID, "application/octet-stream", 10, false)
	if err != nil {
		t.Fatalf("failed to issue download ticket: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/downloads/%s", ticket.Token), nil)
	req.Header.Set("Range", "bytes=3-6")
	req = withClaims(req, "tester")
	rec := httptest.NewRecorder()

	webErr := handler.handleDownloadByTicket(rec, req)
	if webErr != nil {
		t.Fatalf("unexpected web error: %+v", webErr)
	}

	resp := rec.Result()
	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("expected status %d, got %d", http.StatusPartialContent, resp.StatusCode)
	}
	if contentRange := resp.Header.Get("Content-Range"); contentRange != "bytes 3-6/10" {
		t.Fatalf("unexpected content-range: %q", contentRange)
	}
	if rec.Body.String() != "defg" {
		t.Fatalf("expected partial body %q, got %q", "defg", rec.Body.String())
	}
}

func TestHandleDownloadByTicket_RemoveAfterUseArtifactsRemainUntilTicketExpiry(t *testing.T) {
	spaceRoot := t.TempDir()
	filePath := filepath.Join(spaceRoot, "archive.zip")
	if err := os.WriteFile(filePath, []byte("zip-bytes"), 0o644); err != nil {
		t.Fatalf("failed to seed archive file: %v", err)
	}

	store := &fakeTransferSpaceStore{
		spacesByID: map[int64]*space.Space{
			1: {
				ID:        1,
				SpaceName: "Transfer",
				SpacePath: spaceRoot,
			},
		},
	}
	handler := NewHandler(space.NewService(store), nil, nil)

	spaceID := int64(1)
	ticket, err := handler.issueDownloadTicket("tester", filePath, "archive.zip", "file.archive-download-ticket", &spaceID, "application/zip", 9, true)
	if err != nil {
		t.Fatalf("failed to issue archive download ticket: %v", err)
	}

	firstReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/downloads/%s", ticket.Token), nil)
	firstReq = withClaims(firstReq, "tester")
	firstRec := httptest.NewRecorder()
	if webErr := handler.handleDownloadByTicket(firstRec, firstReq); webErr != nil {
		t.Fatalf("unexpected first download web error: %+v", webErr)
	}
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("expected archive artifact to remain after first download, got %v", err)
	}

	secondReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/downloads/%s", ticket.Token), nil)
	secondReq.Header.Set("Range", "bytes=0-2")
	secondReq = withClaims(secondReq, "tester")
	secondRec := httptest.NewRecorder()
	if webErr := handler.handleDownloadByTicket(secondRec, secondReq); webErr != nil {
		t.Fatalf("unexpected second download web error: %+v", webErr)
	}
	if secondRec.Code != http.StatusPartialContent {
		t.Fatalf("expected second request to support range, got %d", secondRec.Code)
	}

	handler.ticketMu.Lock()
	expiredTicket := handler.downloadTickets[ticket.Token]
	expiredTicket.ExpiresAt = time.Now().Add(-time.Millisecond)
	handler.downloadTickets[ticket.Token] = expiredTicket
	handler.ticketMu.Unlock()
	handler.cleanupExpiredDownloadTicketsLocked(time.Now())

	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatalf("expected archive artifact to be removed after ticket expiry, err=%v", err)
	}
}

func TestArchiveDownloadJobLifecycle(t *testing.T) {
	spaceRoot := t.TempDir()
	docsDir := filepath.Join(spaceRoot, "docs")
	if err := os.MkdirAll(filepath.Join(docsDir, "nested"), 0o755); err != nil {
		t.Fatalf("failed to create archive directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "guide.txt"), []byte("guide"), 0o644); err != nil {
		t.Fatalf("failed to seed guide file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "nested", "notes.txt"), []byte("notes"), 0o644); err != nil {
		t.Fatalf("failed to seed notes file: %v", err)
	}

	store := &fakeTransferSpaceStore{
		spacesByID: map[int64]*space.Space{
			1: {
				ID:        1,
				SpaceName: "Archive",
				SpacePath: spaceRoot,
			},
		},
	}
	handler := NewHandler(space.NewService(store), nil, nil)
	handler.archiveDownloads = newArchiveDownloadManager(1, 100*time.Millisecond)

	createBody, err := json.Marshal(map[string]any{
		"paths": []string{"docs"},
	})
	if err != nil {
		t.Fatalf("failed to marshal create body: %v", err)
	}
	createReq := httptest.NewRequest(http.MethodPost, "/api/spaces/1/files/archive-downloads", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq = withClaims(createReq, "tester")
	createRec := httptest.NewRecorder()

	webErr := handler.handleArchiveDownloadCreate(createRec, createReq, 1)
	if webErr != nil {
		t.Fatalf("unexpected create web error: %+v", webErr)
	}
	if createRec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, createRec.Code)
	}

	var createPayload archiveDownloadStatusResponse
	if err := json.NewDecoder(createRec.Body).Decode(&createPayload); err != nil {
		t.Fatalf("failed to decode archive create response: %v", err)
	}
	if createPayload.JobID == "" {
		t.Fatal("expected archive job id")
	}

	var statusPayload archiveDownloadStatusResponse
	deadline := time.Now().Add(2 * time.Second)
	for {
		statusReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/spaces/1/files/archive-downloads?jobId=%s", createPayload.JobID), nil)
		statusReq = withClaims(statusReq, "tester")
		statusRec := httptest.NewRecorder()
		webErr = handler.handleArchiveDownloadStatus(statusRec, statusReq, 1)
		if webErr != nil {
			t.Fatalf("unexpected status web error: %+v", webErr)
		}
		if err := json.NewDecoder(statusRec.Body).Decode(&statusPayload); err != nil {
			t.Fatalf("failed to decode archive status response: %v", err)
		}
		if statusPayload.Status == string(archiveDownloadStateReady) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("archive job did not reach ready state: %+v", statusPayload)
		}
		time.Sleep(20 * time.Millisecond)
	}

	if statusPayload.TotalItems < 2 {
		t.Fatalf("expected archive to report processed items, got %+v", statusPayload)
	}
	if statusPayload.ProcessedItems != statusPayload.TotalItems {
		t.Fatalf("expected archive to finish all items, got %+v", statusPayload)
	}

	ticketBody, err := json.Marshal(map[string]string{"jobId": createPayload.JobID})
	if err != nil {
		t.Fatalf("failed to marshal ticket body: %v", err)
	}
	ticketReq := httptest.NewRequest(http.MethodPost, "/api/spaces/1/files/archive-download-ticket", bytes.NewReader(ticketBody))
	ticketReq.Header.Set("Content-Type", "application/json")
	ticketReq = withClaims(ticketReq, "tester")
	ticketRec := httptest.NewRecorder()

	webErr = handler.handleArchiveDownloadTicket(ticketRec, ticketReq, 1)
	if webErr != nil {
		t.Fatalf("unexpected ticket web error: %+v", webErr)
	}
	if ticketRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, ticketRec.Code)
	}

	var ticketPayload struct {
		DownloadURL string `json:"downloadUrl"`
		FileName    string `json:"fileName"`
	}
	if err := json.NewDecoder(ticketRec.Body).Decode(&ticketPayload); err != nil {
		t.Fatalf("failed to decode ticket response: %v", err)
	}
	if !strings.HasPrefix(ticketPayload.DownloadURL, "/api/downloads/") {
		t.Fatalf("unexpected download URL: %q", ticketPayload.DownloadURL)
	}
	if ticketPayload.FileName != "docs.zip" {
		t.Fatalf("expected archive file name docs.zip, got %q", ticketPayload.FileName)
	}

	handler.archiveDownloads.updateJob(createPayload.JobID, func(job *archiveDownloadJob) {
		job.ExpiresAt = time.Now().Add(-time.Millisecond)
	})
	handler.cleanupArchiveDownloadJobs()

	expiredReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/spaces/1/files/archive-downloads?jobId=%s", createPayload.JobID), nil)
	expiredReq = withClaims(expiredReq, "tester")
	expiredRec := httptest.NewRecorder()
	webErr = handler.handleArchiveDownloadStatus(expiredRec, expiredReq, 1)
	if webErr != nil {
		t.Fatalf("unexpected expired status web error: %+v", webErr)
	}

	var expiredPayload archiveDownloadStatusResponse
	if err := json.NewDecoder(expiredRec.Body).Decode(&expiredPayload); err != nil {
		t.Fatalf("failed to decode expired archive status: %v", err)
	}
	if expiredPayload.Status != string(archiveDownloadStateExpired) {
		t.Fatalf("expected expired status, got %+v", expiredPayload)
	}

	expiredTicketReq := httptest.NewRequest(http.MethodPost, "/api/spaces/1/files/archive-download-ticket", bytes.NewReader(ticketBody))
	expiredTicketReq.Header.Set("Content-Type", "application/json")
	expiredTicketReq = withClaims(expiredTicketReq, "tester")
	expiredTicketRec := httptest.NewRecorder()
	webErr = handler.handleArchiveDownloadTicket(expiredTicketRec, expiredTicketReq, 1)
	if webErr == nil {
		t.Fatal("expected archive ticket request to fail after expiration")
	}
	if webErr.Code != http.StatusGone {
		t.Fatalf("expected status %d, got %+v", http.StatusGone, webErr)
	}
}
