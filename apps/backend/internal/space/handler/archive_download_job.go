package handler

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"taeu.kr/cohesion/internal/audit"
	"taeu.kr/cohesion/internal/auth"
	"taeu.kr/cohesion/internal/platform/logging"
	"taeu.kr/cohesion/internal/platform/web"
)

const (
	defaultArchiveDownloadWorkerLimit = 2
	defaultArchiveDownloadTTL         = 10 * time.Minute
)

type archiveDownloadState string

const (
	archiveDownloadStateQueued   archiveDownloadState = "queued"
	archiveDownloadStateRunning  archiveDownloadState = "running"
	archiveDownloadStateReady    archiveDownloadState = "ready"
	archiveDownloadStateFailed   archiveDownloadState = "failed"
	archiveDownloadStateCanceled archiveDownloadState = "canceled"
	archiveDownloadStateExpired  archiveDownloadState = "expired"
)

type archiveDownloadJob struct {
	ID                   string
	Owner                string
	SpaceID              int64
	RequestID            string
	RequestedPaths       []string
	FileName             string
	State                archiveDownloadState
	FailureReason        string
	ArtifactPath         string
	ArtifactSize         int64
	SourceCount          int
	TotalItems           int
	ProcessedItems       int
	TotalSourceBytes     int64
	ProcessedSourceBytes int64
	CreatedAt            time.Time
	UpdatedAt            time.Time
	ExpiresAt            time.Time
	ctx                  context.Context
	cancel               context.CancelFunc
}

type archiveDownloadManager struct {
	mu      sync.Mutex
	ttl     time.Duration
	workers chan struct{}
	jobs    map[string]*archiveDownloadJob
}

type archiveDownloadSource struct {
	RelativePath string
	AbsPath      string
	BaseName     string
}

type archiveZipEntry struct {
	AbsPath string
	ZipPath string
	Size    int64
	IsDir   bool
}

type archiveDownloadStatusResponse struct {
	JobID                string  `json:"jobId"`
	Status               string  `json:"status"`
	FileName             string  `json:"fileName"`
	SourceCount          int     `json:"sourceCount"`
	TotalItems           int     `json:"totalItems"`
	ProcessedItems       int     `json:"processedItems"`
	TotalSourceBytes     int64   `json:"totalSourceBytes"`
	ProcessedSourceBytes int64   `json:"processedSourceBytes"`
	FailureReason        string  `json:"failureReason,omitempty"`
	ArtifactSize         int64   `json:"artifactSize,omitempty"`
	ExpiresAt            *string `json:"expiresAt,omitempty"`
}

func newArchiveDownloadManager(workerLimit int, ttl time.Duration) *archiveDownloadManager {
	if workerLimit < 1 {
		workerLimit = 1
	}
	if ttl <= 0 {
		ttl = defaultArchiveDownloadTTL
	}
	return &archiveDownloadManager{
		ttl:     ttl,
		workers: make(chan struct{}, workerLimit),
		jobs:    make(map[string]*archiveDownloadJob),
	}
}

func (m *archiveDownloadManager) createJob(owner string, spaceID int64, requestID string, requestedPaths []string, fileName string) (*archiveDownloadJob, error) {
	jobID, err := generateDownloadTicketToken()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	jobCtx, cancel := context.WithCancel(context.Background())
	job := &archiveDownloadJob{
		ID:             jobID,
		Owner:          owner,
		SpaceID:        spaceID,
		RequestID:      requestID,
		RequestedPaths: append([]string(nil), requestedPaths...),
		FileName:       fileName,
		State:          archiveDownloadStateQueued,
		SourceCount:    len(requestedPaths),
		CreatedAt:      now,
		UpdatedAt:      now,
		ctx:            jobCtx,
		cancel:         cancel,
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs[jobID] = job
	return job.copy(), nil
}

func (m *archiveDownloadManager) cancelJob(jobID string, owner string) (*archiveDownloadJob, context.CancelFunc, string, *web.Error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[jobID]
	if !ok {
		return nil, nil, "", &web.Error{Code: http.StatusNotFound, Message: "Archive job not found"}
	}
	if job.Owner != owner {
		return nil, nil, "", &web.Error{Code: http.StatusForbidden, Message: "Archive job access denied"}
	}
	if job.State == archiveDownloadStateCanceled {
		return job.copy(), nil, "", nil
	}
	if job.State == archiveDownloadStateFailed || job.State == archiveDownloadStateExpired {
		return nil, nil, "", &web.Error{Code: http.StatusConflict, Message: "Archive job can no longer be canceled"}
	}

	artifactPath := job.ArtifactPath
	job.ArtifactPath = ""
	job.ArtifactSize = 0
	job.State = archiveDownloadStateCanceled
	job.FailureReason = "archive canceled"
	job.ExpiresAt = time.Now().Add(m.ttl)
	job.UpdatedAt = time.Now()
	return job.copy(), job.cancel, artifactPath, nil
}

func (m *archiveDownloadManager) getJob(jobID string, owner string) (*archiveDownloadJob, *web.Error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[jobID]
	if !ok {
		return nil, &web.Error{Code: http.StatusNotFound, Message: "Archive job not found"}
	}
	if job.Owner != owner {
		jobCopy := job.copy()
		return jobCopy, &web.Error{Code: http.StatusForbidden, Message: "Archive job access denied"}
	}
	return job.copy(), nil
}

func (m *archiveDownloadManager) updateJob(jobID string, update func(job *archiveDownloadJob)) *archiveDownloadJob {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[jobID]
	if !ok {
		return nil
	}
	update(job)
	job.UpdatedAt = time.Now()
	return job.copy()
}

func (m *archiveDownloadManager) cleanupExpiredJobs() []*archiveDownloadJob {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cleanupExpiredJobsLocked(time.Now())
}

func (m *archiveDownloadManager) cleanupExpiredJobsLocked(now time.Time) []*archiveDownloadJob {
	expiredJobs := make([]*archiveDownloadJob, 0)
	for jobID, job := range m.jobs {
		if !job.ExpiresAt.IsZero() && now.After(job.ExpiresAt) && job.State != archiveDownloadStateExpired {
			if job.ArtifactPath != "" {
				_ = os.Remove(job.ArtifactPath)
				job.ArtifactPath = ""
			}
			job.ArtifactSize = 0
			job.FailureReason = "archive expired"
			job.State = archiveDownloadStateExpired
			job.UpdatedAt = now
			expiredJobs = append(expiredJobs, job.copy())
			continue
		}
		if job.State == archiveDownloadStateExpired && !job.ExpiresAt.IsZero() && now.After(job.ExpiresAt.Add(m.ttl)) {
			delete(m.jobs, jobID)
		}
	}
	return expiredJobs
}

func (m *archiveDownloadJob) copy() *archiveDownloadJob {
	if m == nil {
		return nil
	}
	jobCopy := *m
	jobCopy.RequestedPaths = append([]string(nil), m.RequestedPaths...)
	return &jobCopy
}

func (m *archiveDownloadManager) ttlDuration() time.Duration {
	return m.ttl
}

func (h *Handler) handleArchiveDownloads(w http.ResponseWriter, r *http.Request, spaceID int64) *web.Error {
	h.cleanupArchiveDownloadJobs()

	switch r.Method {
	case http.MethodPost:
		return h.handleArchiveDownloadCreate(w, r, spaceID)
	case http.MethodGet:
		return h.handleArchiveDownloadStatus(w, r, spaceID)
	case http.MethodDelete:
		return h.handleArchiveDownloadCancel(w, r, spaceID)
	default:
		return &web.Error{Code: http.StatusMethodNotAllowed, Message: "Method not allowed"}
	}
}

func (h *Handler) handleArchiveDownloadCreate(w http.ResponseWriter, r *http.Request, spaceID int64) *web.Error {
	spaceData, webErr := h.getSpace(r, spaceID)
	if webErr != nil {
		return webErr
	}

	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		return &web.Error{Code: http.StatusUnauthorized, Message: "Unauthorized"}
	}

	var req struct {
		Paths []string `json:"paths"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return &web.Error{Code: http.StatusBadRequest, Message: "Invalid request body", Err: err}
	}

	sources, archiveFileName, webErr := h.resolveArchiveDownloadSources(spaceData.SpacePath, req.Paths)
	if webErr != nil {
		h.recordSpaceAudit(r, audit.Event{
			Action: "file.archive-download",
			Result: audit.ResultFailure,
			Target: fmt.Sprintf("%d items", len(req.Paths)),
			Metadata: map[string]any{
				"sourceCount": len(req.Paths),
				"status":      "failed",
				"reason":      "invalid_sources",
			},
		}, spaceID)
		return webErr
	}

	job, err := h.archiveDownloads.createJob(
		claims.Username,
		spaceID,
		strings.TrimSpace(r.Header.Get("X-Request-Id")),
		req.Paths,
		archiveFileName,
	)
	if err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to create archive job", Err: err}
	}

	h.logArchiveJobEvent("info.archive.job_created", job, nil)
	h.recordSpaceAudit(r, audit.Event{
		Action: "file.archive-download",
		Result: audit.ResultSuccess,
		Target: fmt.Sprintf("%d items", len(req.Paths)),
		Metadata: map[string]any{
			"jobId":       job.ID,
			"sourceCount": len(req.Paths),
			"filename":    archiveFileName,
			"status":      string(job.State),
		},
	}, spaceID)

	go h.runArchiveDownloadJob(job.ID, job.ctx, claims.Username, spaceID, strings.TrimSpace(r.Header.Get("X-Request-Id")), archiveFileName, sources)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(job.toStatusResponse()); err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to encode archive job response", Err: err}
	}
	return nil
}

func (h *Handler) handleArchiveDownloadCancel(w http.ResponseWriter, r *http.Request, spaceID int64) *web.Error {
	h.cleanupArchiveDownloadJobs()

	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		return &web.Error{Code: http.StatusUnauthorized, Message: "Unauthorized"}
	}

	jobID := strings.TrimSpace(r.URL.Query().Get("jobId"))
	if jobID == "" {
		return &web.Error{Code: http.StatusBadRequest, Message: "jobId is required"}
	}

	jobSnapshot, webErr := h.archiveDownloads.getJob(jobID, claims.Username)
	if webErr != nil {
		return webErr
	}
	if jobSnapshot.SpaceID != spaceID {
		return &web.Error{Code: http.StatusForbidden, Message: "Archive job access denied"}
	}

	job, cancelFn, artifactPath, webErr := h.archiveDownloads.cancelJob(jobID, claims.Username)
	if webErr != nil {
		return webErr
	}
	if cancelFn != nil {
		cancelFn()
	}
	if strings.TrimSpace(artifactPath) != "" {
		_ = os.Remove(artifactPath)
	}

	h.logArchiveJobEvent("info.archive.job_canceled", job, nil)
	h.recordSpaceAudit(r, audit.Event{
		Action: "file.archive-download",
		Result: audit.ResultSuccess,
		Target: job.ID,
		Metadata: map[string]any{
			"jobId":                job.ID,
			"filename":             job.FileName,
			"status":               string(job.State),
			"sourceCount":          job.SourceCount,
			"processedItems":       job.ProcessedItems,
			"totalItems":           job.TotalItems,
			"processedSourceBytes": job.ProcessedSourceBytes,
			"totalSourceBytes":     job.TotalSourceBytes,
		},
	}, spaceID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(job.toStatusResponse()); err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to encode archive job cancel response", Err: err}
	}
	return nil
}

func (h *Handler) handleArchiveDownloadStatus(w http.ResponseWriter, r *http.Request, spaceID int64) *web.Error {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		return &web.Error{Code: http.StatusUnauthorized, Message: "Unauthorized"}
	}

	jobID := strings.TrimSpace(r.URL.Query().Get("jobId"))
	if jobID == "" {
		return &web.Error{Code: http.StatusBadRequest, Message: "jobId is required"}
	}

	job, webErr := h.archiveDownloads.getJob(jobID, claims.Username)
	if webErr != nil {
		return webErr
	}
	if job.SpaceID != spaceID {
		return &web.Error{Code: http.StatusForbidden, Message: "Archive job access denied"}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(job.toStatusResponse()); err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to encode archive job status", Err: err}
	}
	return nil
}

func (h *Handler) handleArchiveDownloadTicket(w http.ResponseWriter, r *http.Request, spaceID int64) *web.Error {
	if r.Method != http.MethodPost {
		return &web.Error{Code: http.StatusMethodNotAllowed, Message: "Method not allowed"}
	}
	h.cleanupArchiveDownloadJobs()

	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		return &web.Error{Code: http.StatusUnauthorized, Message: "Unauthorized"}
	}

	var req struct {
		JobID string `json:"jobId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return &web.Error{Code: http.StatusBadRequest, Message: "Invalid request body", Err: err}
	}
	if strings.TrimSpace(req.JobID) == "" {
		return &web.Error{Code: http.StatusBadRequest, Message: "jobId is required"}
	}

	job, webErr := h.archiveDownloads.getJob(req.JobID, claims.Username)
	if webErr != nil {
		return webErr
	}
	if job.SpaceID != spaceID {
		return &web.Error{Code: http.StatusForbidden, Message: "Archive job access denied"}
	}
	if job.State == archiveDownloadStateExpired {
		return &web.Error{Code: http.StatusGone, Message: "Archive job expired"}
	}
	if job.State == archiveDownloadStateCanceled {
		return &web.Error{Code: http.StatusConflict, Message: "Archive job was canceled"}
	}
	if job.State != archiveDownloadStateReady || strings.TrimSpace(job.ArtifactPath) == "" {
		return &web.Error{Code: http.StatusConflict, Message: "Archive job is not ready"}
	}

	ticket, err := h.issueDownloadTicket(
		claims.Username,
		job.ArtifactPath,
		job.FileName,
		"file.archive-download-ticket",
		&spaceID,
		"application/zip",
		job.ArtifactSize,
		false,
	)
	if err != nil {
		h.recordSpaceAudit(r, audit.Event{
			Action: "file.archive-download-ticket",
			Result: audit.ResultFailure,
			Target: req.JobID,
			Metadata: map[string]any{
				"jobId":    req.JobID,
				"filename": job.FileName,
				"size":     job.ArtifactSize,
				"status":   "failed",
			},
		}, spaceID)
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to issue archive download ticket", Err: err}
	}

	h.logArchiveJobEvent("info.archive.job_handoff", job, nil)
	h.recordSpaceAudit(r, audit.Event{
		Action: "file.archive-download-ticket",
		Result: audit.ResultSuccess,
		Target: req.JobID,
		Metadata: map[string]any{
			"jobId":    req.JobID,
			"filename": ticket.FileName,
			"size":     job.ArtifactSize,
			"status":   "ticket_issued",
		},
	}, spaceID)

	type downloadTicketResponse struct {
		DownloadURL string `json:"downloadUrl"`
		FileName    string `json:"fileName"`
		ExpiresAt   string `json:"expiresAt"`
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(downloadTicketResponse{
		DownloadURL: fmt.Sprintf("/api/downloads/%s", ticket.Token),
		FileName:    ticket.FileName,
		ExpiresAt:   ticket.ExpiresAt.Format(time.RFC3339),
	}); err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to encode archive handoff response", Err: err}
	}
	return nil
}

func (h *Handler) resolveArchiveDownloadSources(spaceRoot string, paths []string) ([]archiveDownloadSource, string, *web.Error) {
	if len(paths) == 0 {
		return nil, "", &web.Error{Code: http.StatusBadRequest, Message: "at least 1 path is required"}
	}

	sources := make([]archiveDownloadSource, 0, len(paths))
	for _, relPath := range paths {
		if err := ensurePathOutsideTrash(relPath); err != nil {
			return nil, "", &web.Error{Code: http.StatusForbidden, Message: fmt.Sprintf("Access denied: invalid path %s", relPath), Err: err}
		}
		absPath, err := resolveAbsPath(spaceRoot, relPath)
		if err != nil {
			return nil, "", &web.Error{Code: http.StatusForbidden, Message: fmt.Sprintf("Access denied: invalid path %s", relPath)}
		}
		info, err := os.Stat(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, "", &web.Error{Code: http.StatusNotFound, Message: fmt.Sprintf("Path not found: %s", relPath), Err: err}
			}
			if storageErr := storageAccessWebError(err, "", fmt.Sprintf("Failed to access path: %s", relPath)); storageErr != nil {
				return nil, "", storageErr
			}
		}
		sources = append(sources, archiveDownloadSource{
			RelativePath: relPath,
			AbsPath:      absPath,
			BaseName:     info.Name(),
		})
	}

	if len(paths) == 1 {
		info, err := os.Stat(sources[0].AbsPath)
		if err != nil {
			return nil, "", storageAccessWebError(err, "Path not found", "Failed to inspect archive source")
		}
		if !info.IsDir() {
			return nil, "", &web.Error{Code: http.StatusBadRequest, Message: "Single-file downloads must use the direct download flow"}
		}
		return sources, info.Name() + ".zip", nil
	}

	return sources, fmt.Sprintf("download-%d.zip", time.Now().Unix()), nil
}

func (h *Handler) runArchiveDownloadJob(
	jobID string,
	jobCtx context.Context,
	owner string,
	spaceID int64,
	requestID string,
	fileName string,
	sources []archiveDownloadSource,
) {
	acquiredWorker := false
	select {
	case h.archiveDownloads.workers <- struct{}{}:
		acquiredWorker = true
	case <-jobCtx.Done():
		return
	}
	defer func() {
		if acquiredWorker {
			<-h.archiveDownloads.workers
		}
		h.cleanupArchiveDownloadJobs()
	}()

	job := h.archiveDownloads.updateJob(jobID, func(job *archiveDownloadJob) {
		if job.State == archiveDownloadStateCanceled {
			return
		}
		job.State = archiveDownloadStateRunning
		job.FailureReason = ""
	})
	if job == nil || job.State == archiveDownloadStateCanceled {
		return
	}
	if err := jobCtx.Err(); err != nil {
		return
	}

	entries, totalItems, totalBytes, err := scanArchiveZipEntries(sources)
	if err != nil {
		h.failArchiveDownloadJob(jobID, owner, requestID, safeFilesystemReason("Failed to scan archive sources", err))
		return
	}

	h.archiveDownloads.updateJob(jobID, func(job *archiveDownloadJob) {
		job.TotalItems = totalItems
		job.TotalSourceBytes = totalBytes
		job.ProcessedItems = 0
		job.ProcessedSourceBytes = 0
	})

	zipTempPath, zipSize, webErr := h.buildZipTempArchive(func(zipWriter *zip.Writer) *web.Error {
		for _, entry := range entries {
			if err := jobCtx.Err(); err != nil {
				return &web.Error{Code: http.StatusConflict, Message: "Archive download canceled", Err: err}
			}
			writtenBytes, writeErr := writeArchiveZipEntry(jobCtx, zipWriter, entry)
			if writeErr != nil {
				if errors.Is(writeErr, context.Canceled) {
					return &web.Error{Code: http.StatusConflict, Message: "Archive download canceled", Err: writeErr}
				}
				return &web.Error{Code: http.StatusInternalServerError, Message: safeFilesystemReason("Failed to prepare archive entry", writeErr), Err: writeErr}
			}
			h.archiveDownloads.updateJob(jobID, func(job *archiveDownloadJob) {
				job.ProcessedItems++
				job.ProcessedSourceBytes += writtenBytes
			})
		}
		return nil
	})
	if webErr != nil {
		if errors.Is(webErr.Err, context.Canceled) {
			return
		}
		h.failArchiveDownloadJob(jobID, owner, requestID, webErr.Message)
		return
	}
	if err := jobCtx.Err(); err != nil {
		_ = os.Remove(zipTempPath)
		return
	}

	expiresAt := time.Now().Add(h.archiveDownloads.ttlDuration())
	publishCanceled := false
	job = h.archiveDownloads.updateJob(jobID, func(job *archiveDownloadJob) {
		if job.State == archiveDownloadStateCanceled {
			publishCanceled = true
			return
		}
		job.State = archiveDownloadStateReady
		job.FailureReason = ""
		job.ArtifactPath = zipTempPath
		job.ArtifactSize = zipSize
		job.FileName = fileName
		job.ExpiresAt = expiresAt
	})
	if job == nil {
		_ = os.Remove(zipTempPath)
		return
	}
	if publishCanceled {
		_ = os.Remove(zipTempPath)
		return
	}

	h.logArchiveJobEvent("info.archive.job_ready", job, nil)
	h.recordSpaceAuditBackground(owner, requestID, audit.Event{
		Action:    "file.archive-download",
		Result:    audit.ResultSuccess,
		Target:    job.ID,
		RequestID: requestID,
		SpaceID:   &spaceID,
		Metadata: map[string]any{
			"jobId":                job.ID,
			"filename":             job.FileName,
			"size":                 zipSize,
			"status":               string(job.State),
			"sourceCount":          job.SourceCount,
			"totalItems":           job.TotalItems,
			"processedItems":       job.ProcessedItems,
			"totalSourceBytes":     job.TotalSourceBytes,
			"processedSourceBytes": job.ProcessedSourceBytes,
		},
	})
}

func (h *Handler) failArchiveDownloadJob(jobID string, owner string, requestID string, reason string) {
	job := h.archiveDownloads.updateJob(jobID, func(job *archiveDownloadJob) {
		if job.State == archiveDownloadStateCanceled {
			return
		}
		if job.ArtifactPath != "" {
			_ = os.Remove(job.ArtifactPath)
			job.ArtifactPath = ""
		}
		job.ArtifactSize = 0
		job.State = archiveDownloadStateFailed
		job.FailureReason = reason
		job.ExpiresAt = time.Now().Add(h.archiveDownloads.ttlDuration())
	})
	if job == nil {
		return
	}

	h.logArchiveJobEvent("warn.archive.job_failed", job, errors.New(reason))
	h.recordSpaceAuditBackground(owner, requestID, audit.Event{
		Action:    "file.archive-download",
		Result:    audit.ResultFailure,
		Target:    job.ID,
		RequestID: requestID,
		SpaceID:   &job.SpaceID,
		Metadata: map[string]any{
			"jobId":                job.ID,
			"filename":             job.FileName,
			"status":               string(job.State),
			"reason":               reason,
			"sourceCount":          job.SourceCount,
			"totalItems":           job.TotalItems,
			"processedItems":       job.ProcessedItems,
			"totalSourceBytes":     job.TotalSourceBytes,
			"processedSourceBytes": job.ProcessedSourceBytes,
		},
	})
}

func (h *Handler) logArchiveJobEvent(eventName string, job *archiveDownloadJob, err error) {
	if job == nil {
		return
	}

	logger := logging.Event(log.Info(), logging.ComponentStorage, eventName).
		Str("job_id", job.ID).
		Int64("space_id", job.SpaceID).
		Str("owner", job.Owner).
		Str("status", string(job.State)).
		Str("filename", job.FileName).
		Int("source_count", job.SourceCount).
		Int("total_items", job.TotalItems).
		Int("processed_items", job.ProcessedItems).
		Int64("total_source_bytes", job.TotalSourceBytes).
		Int64("processed_source_bytes", job.ProcessedSourceBytes)
	if err != nil {
		logger = logger.Err(err)
	}
	logger.Msg("archive download job updated")
}

func (h *Handler) recordSpaceAuditBackground(actor string, requestID string, event audit.Event) {
	if h.auditRecorder == nil {
		return
	}
	if strings.TrimSpace(actor) != "" {
		event.Actor = actor
	}
	if event.RequestID == "" {
		event.RequestID = strings.TrimSpace(requestID)
	}
	h.auditRecorder.RecordBestEffort(event)
}

func (h *Handler) cleanupArchiveDownloadJobs() {
	expiredJobs := h.archiveDownloads.cleanupExpiredJobs()
	for _, job := range expiredJobs {
		h.logArchiveJobEvent("info.archive.job_expired", job, nil)
		h.recordSpaceAuditBackground(job.Owner, job.RequestID, audit.Event{
			Action:    "file.archive-download",
			Result:    audit.ResultPartial,
			Target:    job.ID,
			RequestID: job.RequestID,
			SpaceID:   &job.SpaceID,
			Metadata: map[string]any{
				"jobId":                job.ID,
				"filename":             job.FileName,
				"status":               string(job.State),
				"reason":               job.FailureReason,
				"sourceCount":          job.SourceCount,
				"totalItems":           job.TotalItems,
				"processedItems":       job.ProcessedItems,
				"totalSourceBytes":     job.TotalSourceBytes,
				"processedSourceBytes": job.ProcessedSourceBytes,
			},
		})
	}
}

func (j *archiveDownloadJob) toStatusResponse() archiveDownloadStatusResponse {
	response := archiveDownloadStatusResponse{
		JobID:                j.ID,
		Status:               string(j.State),
		FileName:             j.FileName,
		SourceCount:          j.SourceCount,
		TotalItems:           j.TotalItems,
		ProcessedItems:       j.ProcessedItems,
		TotalSourceBytes:     j.TotalSourceBytes,
		ProcessedSourceBytes: j.ProcessedSourceBytes,
		FailureReason:        j.FailureReason,
		ArtifactSize:         j.ArtifactSize,
	}
	if !j.ExpiresAt.IsZero() {
		expiresAt := j.ExpiresAt.Format(time.RFC3339)
		response.ExpiresAt = &expiresAt
	}
	return response
}

func scanArchiveZipEntries(sources []archiveDownloadSource) ([]archiveZipEntry, int, int64, error) {
	entries := make([]archiveZipEntry, 0)
	totalItems := 0
	var totalBytes int64

	for _, source := range sources {
		info, err := os.Stat(source.AbsPath)
		if err != nil {
			return nil, 0, 0, err
		}

		if !info.IsDir() {
			entries = append(entries, archiveZipEntry{
				AbsPath: source.AbsPath,
				ZipPath: filepath.ToSlash(source.BaseName),
				Size:    info.Size(),
			})
			totalItems++
			totalBytes += info.Size()
			continue
		}

		err = filepath.Walk(source.AbsPath, func(currentPath string, entryInfo os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entryInfo.Mode()&os.ModeSymlink != 0 {
				return nil
			}
			relPath, err := filepath.Rel(source.AbsPath, currentPath)
			if err != nil {
				return err
			}
			if relPath == "." {
				return nil
			}

			zipPath := filepath.ToSlash(filepath.Join(source.BaseName, relPath))
			entries = append(entries, archiveZipEntry{
				AbsPath: currentPath,
				ZipPath: zipPath,
				Size:    entryInfo.Size(),
				IsDir:   entryInfo.IsDir(),
			})
			totalItems++
			if !entryInfo.IsDir() {
				totalBytes += entryInfo.Size()
			}
			return nil
		})
		if err != nil {
			return nil, 0, 0, err
		}
	}

	return entries, totalItems, totalBytes, nil
}

func writeArchiveZipEntry(ctx context.Context, zipWriter *zip.Writer, entry archiveZipEntry) (int64, error) {
	info, err := os.Stat(entry.AbsPath)
	if err != nil {
		return 0, err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return 0, err
	}
	header.Name = filepath.ToSlash(entry.ZipPath)
	header.Method = zip.Deflate

	if entry.IsDir {
		header.Name += "/"
		if _, err := zipWriter.CreateHeader(header); err != nil {
			return 0, err
		}
		return 0, nil
	}

	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return 0, err
	}

	file, err := os.Open(entry.AbsPath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	writtenBytes, err := copyWithContext(ctx, writer, file)
	if err != nil {
		return writtenBytes, err
	}
	return writtenBytes, nil
}

func copyWithContext(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	buffer := make([]byte, 64*1024)
	var written int64
	for {
		if err := ctx.Err(); err != nil {
			return written, err
		}
		readBytes, readErr := src.Read(buffer)
		if readBytes > 0 {
			writeBytes, writeErr := dst.Write(buffer[:readBytes])
			written += int64(writeBytes)
			if writeErr != nil {
				return written, writeErr
			}
			if writeBytes != readBytes {
				return written, io.ErrShortWrite
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return written, nil
			}
			return written, readErr
		}
	}
}
