package handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"taeu.kr/cohesion/internal/space"
	spaceStore "taeu.kr/cohesion/internal/space/store"
)

const uploadStagePrefix = ".cohesion-upload-"

// TransferRuntime is shared by HTTP handlers in the same server process.
type TransferRuntime struct {
	shared           *downloadSnapshots
	store            *spaceStore.TransferStore
	archive          *archiveDownloadManager
	quota            *space.QuotaService
	uploads          string
	sessionLocks     [64]sync.Mutex
	publishMu        sync.Mutex
	requestPublishMu sync.Mutex
}

func NewTransferRuntime(ctx context.Context, db *sql.DB, spaceService *space.Service, root string) (*TransferRuntime, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	archivePath := filepath.Join(root, "archives")
	uploadPath := filepath.Join(root, "uploads")
	for _, path := range []string{archivePath, uploadPath} {
		if err := os.MkdirAll(path, 0700); err != nil {
			return nil, err
		}
	}

	snapshots, err := newDownloadSnapshots(filepath.Join(root, "download-links"))
	if err != nil {
		return nil, err
	}
	if spaceService != nil {
		spaceService.SetDownloadStorage(snapshots.dir)
	}
	transferStore := spaceStore.NewTransferStore(db)
	if err := transferStore.RecoverUploadTransfers(ctx); err != nil {
		return nil, err
	}
	archive := newArchiveDownloadManager(defaultArchiveDownloadWorkerLimit, defaultArchiveDownloadTTL)
	archive.store = transferStore
	archive.dir = archivePath
	if err := archive.restore(ctx); err != nil {
		return nil, err
	}
	quota := space.NewQuotaService(spaceService)
	quota.SetUploadReservationStore(transferStore)
	runtime := &TransferRuntime{shared: snapshots, store: transferStore, archive: archive, quota: quota, uploads: uploadPath}
	if err := snapshots.cleanup(ctx, transferStore); err != nil {
		return nil, err
	}
	if err := runtime.restoreUploadSessions(ctx); err != nil {
		return nil, err
	}
	if err := runtime.cleanUploadStages(ctx); err != nil {
		return nil, err
	}
	return runtime, nil
}

func (h *Handler) SetTransferRuntime(runtime *TransferRuntime) {
	h.transferRuntime = runtime
	h.archiveDownloads = runtime.archive
	h.quotaService = runtime.quota
}

func (h *Handler) RunTransferCleanup(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.cleanupArchiveDownloadJobs()
			if err := h.transferRuntime.shared.cleanup(ctx, h.transferRuntime.store); err != nil {
				log.Warn().Err(err).Msg("failed to clean download link snapshots")
			}
			if err := h.transferRuntime.expireUploadSessions(ctx); err != nil {
				log.Warn().Err(err).Msg("failed to expire upload sessions")
			}
			if err := h.transferRuntime.cleanUploadStages(ctx); err != nil {
				log.Warn().Err(err).Msg("failed to clean up abandoned upload stages")
			}
		}
	}
}

func (r *TransferRuntime) createUploadStage(ctx context.Context, destPath string) (*os.File, string, error) {
	id, err := generateDownloadTicketToken()
	if err != nil {
		return nil, "", err
	}
	dir, mode := r.uploads, os.FileMode(0600)
	if destPath != "" {
		dir, mode = filepath.Dir(destPath), 0666
	}
	path, err := filepath.Abs(filepath.Join(dir, uploadStagePrefix+id))
	if err != nil {
		return nil, "", err
	}
	if err := r.store.AddUploadStage(ctx, path); err != nil {
		return nil, "", err
	}
	r.quota.TrackUploadStage(path)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		r.forgetUploadStage(path)
		return nil, "", err
	}
	return file, path, nil
}

func (r *TransferRuntime) forgetUploadStage(path string) {
	r.quota.ForgetUploadStage(path)
	if err := r.store.DeleteUploadStage(context.Background(), path); err != nil {
		log.Warn().Err(err).Msg("failed to remove upload stage record")
	}
}

func (h *Handler) discardUploadStage(path string) {
	if h.transferRuntime == nil {
		_ = os.Remove(path)
		return
	}
	if err := removeUploadStage(path); err == nil {
		h.transferRuntime.forgetUploadStage(path)
	} else {
		log.Warn().Err(err).Msg("failed to remove staged upload")
		if err := h.transferRuntime.store.AbandonUploadStage(context.Background(), path); err != nil {
			log.Warn().Err(err).Msg("failed to mark upload stage for cleanup")
		}
	}
}

func (r *TransferRuntime) cleanUploadStages(ctx context.Context) error {
	paths, err := r.store.ListAbandonedUploadStages(ctx)
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := removeUploadStage(path); err != nil {
			r.quota.TrackUploadStage(path)
			log.Warn().Err(err).Msg("failed to remove abandoned upload stage")
			continue
		}
		r.forgetUploadStage(path)
	}
	return nil
}

func removeUploadStage(path string) error {
	base := filepath.Base(path)
	id, found := strings.CutPrefix(base, uploadStagePrefix)
	if !filepath.IsAbs(path) || !found || !validTransferID(id) {
		return fmt.Errorf("invalid upload staging path")
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		// Keep the record if the destination volume may be offline.
		_, err := os.Stat(filepath.Dir(path))
		return err
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("upload stage is not a regular file")
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
