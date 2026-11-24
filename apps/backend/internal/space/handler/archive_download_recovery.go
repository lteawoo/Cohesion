package handler

import (
	"archive/zip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	"taeu.kr/cohesion/internal/platform/web"
)

type archiveJobStore interface {
	ListArchiveJobs(ctx context.Context) (map[string]json.RawMessage, error)
	SaveArchiveJob(ctx context.Context, id string, snapshot []byte) error
	DeleteArchiveJob(ctx context.Context, id string) error
}

func validTransferID(id string) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(id)
	return err == nil && len(id) == 32 && len(decoded) == 24
}

func (j *archiveDownloadJob) setTerminalState(state archiveDownloadState, reason string, now, expiresAt time.Time) {
	j.State = state
	j.FailureReason = reason
	j.ArtifactPath = ""
	j.ArtifactSize = 0
	j.UpdatedAt = now
	j.ExpiresAt = expiresAt
}

func (m *archiveDownloadManager) saveJob(job *archiveDownloadJob) error {
	if m.store != nil {
		snapshot, err := json.Marshal(job)
		if err != nil {
			return err
		}
		if err := m.store.SaveArchiveJob(context.Background(), job.ID, snapshot); err != nil {
			return err
		}
	}
	job.needsSave = false
	return nil
}

func (m *archiveDownloadManager) artifactPath(id string) string {
	return filepath.Join(m.dir, id+".zip")
}

func (m *archiveDownloadManager) restore(ctx context.Context) error {
	snapshots, err := m.store.ListArchiveJobs(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	for id, snapshot := range snapshots {
		var job archiveDownloadJob
		if err := json.Unmarshal(snapshot, &job); err != nil {
			return fmt.Errorf("decode archive job %q: %w", id, err)
		}
		if job.ID != id || !validTransferID(id) {
			return fmt.Errorf("invalid persisted archive job ID %q", id)
		}

		switch job.State {
		case archiveDownloadStateQueued, archiveDownloadStateRunning:
			job.setTerminalState(archiveDownloadStateFailed, "Archive preparation interrupted by server restart", now, now.Add(m.ttl))
		case archiveDownloadStateReady:
			path := m.artifactPath(id)
			info, err := os.Lstat(path)
			switch {
			case !job.ExpiresAt.After(now):
				job.setTerminalState(archiveDownloadStateExpired, "archive expired", now, job.ExpiresAt)
			case err != nil || !info.Mode().IsRegular() || info.Size() != job.ArtifactSize:
				job.setTerminalState(archiveDownloadStateFailed, "Archive artifact is unavailable", now, now.Add(m.ttl))
			default:
				job.ArtifactPath = path
			}
		case archiveDownloadStateCanceled, archiveDownloadStateFailed, archiveDownloadStateExpired:
			job.ArtifactPath = ""
		default:
			return fmt.Errorf("invalid persisted archive state for job %q", id)
		}
		if err := m.saveJob(&job); err != nil {
			return err
		}
		m.jobs[id] = &job
	}
	m.cleanupExpiredJobsLocked(now)
	return nil
}

func (m *archiveDownloadManager) cleanTerminalArtifact(job *archiveDownloadJob) bool {
	if m.dir == "" || job.artifactCleaned {
		return true
	}
	switch job.State {
	case archiveDownloadStateCanceled, archiveDownloadStateFailed, archiveDownloadStateExpired:
		if err := os.Remove(m.artifactPath(job.ID)); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Warn().Err(err).Str("job_id", job.ID).Msg("failed to remove archive artifact")
			return false
		}
		job.artifactCleaned = true
	}
	return true
}

func (h *Handler) buildArchiveJobZIP(id string, writeZip func(*zip.Writer) *web.Error) (string, int64, *web.Error) {
	if h.archiveDownloads.dir == "" {
		return h.buildZipTempArchive(writeZip)
	}
	file, err := os.OpenFile(h.archiveDownloads.artifactPath(id), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", 0, &web.Error{Code: http.StatusInternalServerError, Message: "Failed to prepare zip archive", Err: err}
	}
	return writeZIPArtifact(file, writeZip)
}
