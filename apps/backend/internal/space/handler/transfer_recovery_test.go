package handler

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"taeu.kr/cohesion/internal/platform/database"
	"taeu.kr/cohesion/internal/space"
)

func openTransferTestDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := database.Migrate(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	return db
}

func openTransferTestRuntime(t *testing.T, db *sql.DB, service *space.Service, path string) *TransferRuntime {
	t.Helper()
	runtime, err := NewTransferRuntime(t.Context(), db, service, path)
	if err != nil {
		t.Fatal(err)
	}
	return runtime
}

func TestArchiveRecoveryAfterDatabaseReopen(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "cohesion.db")
	statePath := filepath.Join(root, "transfers")
	spaceRoot := t.TempDir()
	reports := filepath.Join(spaceRoot, "reports")
	if err := os.Mkdir(reports, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(reports, "report.txt"), []byte("report"), 0644); err != nil {
		t.Fatal(err)
	}
	service := space.NewService(&fakeTransferSpaceStore{spacesByID: map[int64]*space.Space{
		1: {ID: 1, SpaceName: "Reports", SpacePath: spaceRoot},
	}})
	db := openTransferTestDB(t, dbPath)
	first := openTransferTestRuntime(t, db, service, statePath)
	handler := NewHandler(service, nil, nil)
	handler.SetTransferRuntime(first)

	req := httptest.NewRequest(http.MethodPost, "/api/spaces/1/files/archive-downloads", strings.NewReader(`{"paths":["reports"]}`))
	rec := httptest.NewRecorder()
	if err := handler.handleArchiveDownloadCreate(rec, withClaims(req, "tester"), 1); err != nil {
		t.Fatal(err)
	}
	var created archiveDownloadStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		job, err := first.archive.getJob(created.JobID, "tester")
		if err != nil {
			t.Fatal(err)
		}
		if job.State == archiveDownloadStateReady {
			break
		}
		if job.State == archiveDownloadStateFailed || time.Now().After(deadline) {
			t.Fatalf("archive was not prepared: %s %s", job.State, job.FailureReason)
		}
		time.Sleep(10 * time.Millisecond)
	}

	type savedJob struct {
		id    string
		state archiveDownloadState
	}
	var interrupted []savedJob
	for _, state := range []archiveDownloadState{archiveDownloadStateQueued, archiveDownloadStateRunning, archiveDownloadStateReady} {
		job, err := first.archive.createJob("tester", 1, "req", []string{"report.txt"}, "report.zip")
		if err != nil {
			t.Fatal(err)
		}
		if state != archiveDownloadStateQueued {
			first.archive.updateJob(job.ID, func(job *archiveDownloadJob) {
				job.State = state
				job.ExpiresAt = time.Now().Add(time.Minute)
			})
		}
		if state == archiveDownloadStateRunning {
			if err := os.WriteFile(first.archive.artifactPath(job.ID), []byte("partial"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		interrupted = append(interrupted, savedJob{id: job.ID, state: state})
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db = openTransferTestDB(t, dbPath)
	restored := openTransferTestRuntime(t, db, service, statePath)
	handler = NewHandler(service, nil, nil)
	handler.SetTransferRuntime(restored)
	for _, saved := range interrupted {
		job, err := restored.archive.getJob(saved.id, "tester")
		if err != nil || job.State != archiveDownloadStateFailed {
			t.Fatalf("restoring %s: job=%v error=%v", saved.state, job, err)
		}
		if _, err := os.Stat(restored.archive.artifactPath(saved.id)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("partial artifact was not cleaned up: %v", err)
		}
	}

	ticketRequest := httptest.NewRequest(http.MethodPost, "/api/spaces/1/files/archive-download-ticket", strings.NewReader(`{"jobId":"`+created.JobID+`"}`))
	if err := handler.handleArchiveDownloadTicket(httptest.NewRecorder(), withClaims(ticketRequest, "other"), 1); err == nil || err.Code != http.StatusForbidden {
		t.Fatalf("restored archive provided a ticket to another owner: %v", err)
	}
	ticketRequest = httptest.NewRequest(http.MethodPost, "/api/spaces/1/files/archive-download-ticket", strings.NewReader(`{"jobId":"`+created.JobID+`"}`))
	ticketRec := httptest.NewRecorder()
	if err := handler.handleArchiveDownloadTicket(ticketRec, withClaims(ticketRequest, "tester"), 1); err != nil {
		t.Fatal(err)
	}
	var ticket struct {
		DownloadURL string `json:"downloadUrl"`
	}
	if err := json.Unmarshal(ticketRec.Body.Bytes(), &ticket); err != nil {
		t.Fatal(err)
	}
	download := httptest.NewRequest(http.MethodGet, ticket.DownloadURL, nil)
	downloadRec := httptest.NewRecorder()
	if err := handler.handleDownloadByTicket(downloadRec, withClaims(download, "tester")); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(downloadRec.Body.Bytes()), int64(downloadRec.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(archive.File) != 1 || archive.File[0].Name != "reports/report.txt" {
		t.Fatal("restored ZIP did not contain reports/report.txt")
	}

	job, webErr := restored.archive.getJob(created.JobID, "tester")
	if webErr != nil {
		t.Fatal(webErr)
	}
	restored.archive.mu.Lock()
	restored.archive.cleanupExpiredJobsLocked(job.ExpiresAt.Add(time.Second))
	restored.archive.mu.Unlock()
	if _, err := os.Stat(job.ArtifactPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expired archive was not removed: %v", err)
	}
}

func TestUploadRecoveryReclaimsReservationsAndStagedFiles(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "cohesion.db")
	statePath := filepath.Join(root, "transfers")
	spaceRoot := t.TempDir()
	quota := int64(12)
	service := space.NewService(&fakeQuotaSpaceStore{spacesByID: map[int64]*space.Space{
		1: {ID: 1, SpaceName: "Uploads", SpacePath: spaceRoot, QuotaBytes: &quota},
	}})
	db := openTransferTestDB(t, dbPath)
	first := openTransferTestRuntime(t, db, service, statePath)
	handler := NewHandler(service, nil, nil)
	handler.SetTransferRuntime(first)

	rec := httptest.NewRecorder()
	if err := handler.handleFileUpload(rec, newUploadRequest(t, "complete.txt", "0123456789", map[string]string{"size": "10"}), 1); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("upload with a staged file did not fit the quota: %v", err)
	}
	if _, err := first.quota.AcquireWriteReservation(t.Context(), 1, "abandoned", 2); err != nil {
		t.Fatal(err)
	}
	if got := first.quota.ReservedBytes(1); got != 2 {
		t.Fatalf("expected two reserved bytes, got %d", got)
	}
	var reserved int64
	if err := db.QueryRow("SELECT delta_bytes FROM upload_quota_reservations WHERE id = ?", "abandoned").Scan(&reserved); err != nil || reserved != 2 {
		t.Fatalf("active reservation was not stored: %d, %v", reserved, err)
	}
	var paths []string
	for _, target := range []string{"", filepath.Join(spaceRoot, "another.txt")} {
		file, path, err := first.createUploadStage(t.Context(), target)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.WriteString("00"); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db = openTransferTestDB(t, dbPath)
	restored := openTransferTestRuntime(t, db, service, statePath)
	for _, path := range paths {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("abandoned stage still exists: %v", err)
		}
	}
	if data, err := os.ReadFile(filepath.Join(spaceRoot, "complete.txt")); err != nil || string(data) != "0123456789" {
		t.Fatalf("committed upload changed: %q %v", data, err)
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM upload_quota_reservations").Scan(&count); err != nil || count != 0 {
		t.Fatalf("abandoned reservations still present: %d, %v", count, err)
	}
	if err := db.QueryRow("SELECT count(*) FROM upload_staging_files").Scan(&count); err != nil || count != 0 {
		t.Fatalf("staging records still present: %d, %v", count, err)
	}
	if _, err := restored.quota.AcquireWriteReservation(t.Context(), 1, "new-upload", 2); err != nil {
		t.Fatalf("abandoned reservation blocked a new upload: %v", err)
	}
}

func TestUploadRecoveryRetriesUnavailableStage(t *testing.T) {
	root := t.TempDir()
	db := openTransferTestDB(t, filepath.Join(root, "cohesion.db"))
	statePath := filepath.Join(root, "transfers")
	dir := filepath.Join(root, "destination")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	first := openTransferTestRuntime(t, db, nil, statePath)
	file, stage, err := first.createUploadStage(t.Context(), filepath.Join(dir, "upload.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	offline := filepath.Join(root, "offline")
	if err := os.Rename(dir, offline); err != nil {
		t.Fatal(err)
	}

	restored := openTransferTestRuntime(t, db, nil, statePath)
	paths, err := restored.store.ListAbandonedUploadStages(t.Context())
	if err != nil || len(paths) != 1 || paths[0] != stage {
		t.Fatalf("unavailable stage was not retained: %v, %v", paths, err)
	}
	if err := os.Rename(offline, dir); err != nil {
		t.Fatal(err)
	}
	if err := restored.cleanUploadStages(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stage); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("abandoned stage survived retry: %v", err)
	}
}
