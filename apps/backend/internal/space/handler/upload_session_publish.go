package handler

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	"taeu.kr/cohesion/internal/audit"
	"taeu.kr/cohesion/internal/platform/web"
	store "taeu.kr/cohesion/internal/space/store"
)

func (r *TransferRuntime) lockSession(id string) func() {
	lock := &r.sessionLocks[id[0]%byte(len(r.sessionLocks))]
	lock.Lock()
	return lock.Unlock
}

func (h *Handler) completeUploadSession(w http.ResponseWriter, r *http.Request, u *store.UploadSession) *web.Error {
	if u.Offset != u.Size {
		return &web.Error{Code: 409, Message: "Upload is incomplete"}
	}
	data, e := h.getSpace(r, u.SpaceID)
	if e != nil {
		return e
	}
	currentRoot, err := filepath.Abs(data.SpacePath)
	if err != nil || currentRoot != u.Root {
		return &web.Error{Code: 409, Message: "Upload destination changed"}
	}
	dir, err := openUploadDir(u)
	if err != nil {
		return storageOperationWebError(err, "Cannot access upload directory")
	}
	defer dir.Close()
	rt := h.transferRuntime
	rt.publishMu.Lock()
	defer rt.publishMu.Unlock()
	ctx := r.Context()
	h.invalidateQuotaForSpaces(u.SpaceID)
	if u.Destination == "" {
		if policy := r.URL.Query().Get("conflictPolicy"); policy != "" {
			u.Policy = policy
		}
		plan, e := h.sessionUploadPlan(ctx, u, dir)
		if e != nil {
			return e
		}
		u.Destination = plan.resultFileName
		if plan.skip {
			return h.finishUploadResponse(w, r, u, "skipped")
		}
		if e := h.reserveSession(ctx, u, plan); e != nil {
			return e
		}
		if err := rt.store.SaveSession(ctx, u); err != nil {
			return uploadIOError(err)
		}
	}
	temp := uploadStagePrefix + u.ID
	rt.quota.TrackUploadStage(filepath.Join(u.Root, u.Path, temp))
	if !u.PublishReady {
		if err := copyForPublication(dir, temp, u); err != nil {
			return storageOperationWebError(err, "Cannot prepare uploaded file")
		}
		u.PublishReady = true
		if err := rt.store.SaveSession(ctx, u); err != nil {
			return uploadIOError(err)
		}
	}
	if _, err := dir.Lstat(temp); errors.Is(err, os.ErrNotExist) {
		// The process may have stopped after rename and before the completion receipt.
		if !matchesPublishedFile(dir, u) {
			return &web.Error{Code: 409, Message: "Published file changed"}
		}
	} else if err != nil {
		return uploadIOError(err)
	} else {
		if _, err := dir.Lstat(u.Destination); err == nil && u.Policy != "overwrite" {
			// Re-evaluate the chosen name/policy on the next complete request.
			u.Destination = ""
			u.PublishReady = false
			if err := dir.Remove(temp); err != nil {
				return uploadIOError(err)
			}
			rt.quota.ForgetUploadStage(filepath.Join(u.Root, u.Path, temp))
			if err := rt.store.SaveSession(ctx, u); err != nil {
				return uploadIOError(err)
			}
			return &web.Error{Code: 409, Message: "File already exists"}
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return uploadIOError(err)
		}
		if err := dir.Rename(temp, u.Destination); err != nil {
			return storageOperationWebError(err, "Cannot publish uploaded file")
		}
	}
	return h.finishUploadResponse(w, r, u, "uploaded")
}

func copyForPublication(dir *os.Root, temp string, u *store.UploadSession) error {
	source, err := os.Open(u.Stage)
	if err != nil {
		return err
	}
	defer source.Close()
	// Only a previous interrupted preparation can own this session-specific name.
	if err := dir.Remove(temp); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	f, err := dir.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	written, err := io.Copy(f, source)
	if err != nil {
		return err
	}
	if written != u.Size {
		return fmt.Errorf("unexpected staged size")
	}
	if err := f.Sync(); err != nil {
		return err
	}
	return f.Close()
}

func matchesPublishedFile(dir *os.Root, u *store.UploadSession) bool {
	info, err := dir.Lstat(u.Destination)
	if err != nil || !info.Mode().IsRegular() || info.Size() != u.Size {
		return false
	}
	dest, err := dir.Open(u.Destination)
	if err != nil {
		return false
	}
	defer dest.Close()
	src, err := os.Open(u.Stage)
	if err != nil {
		return false
	}
	defer src.Close()
	a, b := sha256.New(), sha256.New()
	if _, err = io.Copy(a, src); err != nil {
		return false
	}
	if _, err = io.Copy(b, dest); err != nil {
		return false
	}
	return string(a.Sum(nil)) == string(b.Sum(nil))
}

func (h *Handler) finishUploadResponse(w http.ResponseWriter, r *http.Request, u *store.UploadSession, status string) *web.Error {
	if err := h.transferRuntime.closeUploadSession(r.Context(), u, status); err != nil {
		return uploadIOError(err)
	}
	result := audit.ResultSuccess
	if status == "skipped" {
		result = audit.ResultPartial
	} else {
		h.invalidateQuotaForSpaces(u.SpaceID)
		h.markSearchIndexDirty(r.Context(), u.SpaceID, "upload")
	}
	h.recordSpaceAudit(r, audit.Event{Action: "file.upload", Result: result, Target: filepath.ToSlash(filepath.Join(u.Path, u.Destination)), Metadata: map[string]any{"status": status, "size": u.Size}}, u.SpaceID)
	return sendUploadResponse(w, uploadSessionResponse{ID: u.ID, Status: status, Filename: u.Destination, Offset: u.Offset})
}

func (rt *TransferRuntime) closeUploadSession(ctx context.Context, u *store.UploadSession, status string) error {
	// A disconnected destination volume may need another cleanup pass. Stop
	// accepting chunks and release the reservation before waiting for it.
	if status == "" {
		if u.Expires != 0 {
			u.Expires = 0
			if err := rt.store.SaveSession(ctx, u); err != nil {
				return err
			}
		}
		rt.quota.ReleaseWriteReservation(u.ID)
	}
	if u.Destination != "" && status != "skipped" {
		dir, err := openUploadDir(u)
		if err != nil {
			return err
		}
		defer dir.Close()
		temp := uploadStagePrefix + u.ID
		if err = dir.Remove(temp); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if status == "" && u.PublishReady && errors.Is(err, os.ErrNotExist) && matchesPublishedFile(dir, u) {
			status = "uploaded"
		}
		rt.quota.ForgetUploadStage(filepath.Join(u.Root, u.Path, temp))
	}
	if err := rt.store.CloseSession(ctx, u, status, sessionExpiry()); err != nil {
		return err
	}
	rt.quota.ReleaseWriteReservation(u.ID)
	if err := removeUploadStage(u.Stage); err == nil {
		rt.forgetUploadStage(u.Stage)
	}
	return nil
}

func (rt *TransferRuntime) restoreUploadSessions(ctx context.Context) error {
	if err := rt.store.RestoreSessionReservations(ctx, rt.quota.RestoreWriteReservation); err != nil {
		return err
	}
	ids, err := rt.store.SessionIDs(ctx, math.MaxInt64)
	if err != nil {
		return err
	}
	for _, id := range ids {
		u, err := rt.store.Session(ctx, id)
		if err != nil {
			return err
		}
		rt.quota.TrackUploadStage(u.Stage)
		if u.Destination != "" {
			rt.quota.TrackUploadStage(filepath.Join(u.Root, u.Path, uploadStagePrefix+id))
		}
	}
	return rt.expireUploadSessions(ctx)
}

func (rt *TransferRuntime) expireUploadSessions(ctx context.Context) error {
	now := time.Now().Unix()
	ids, err := rt.store.SessionIDs(ctx, now)
	if err != nil {
		return err
	}
	for _, id := range ids {
		unlock := rt.lockSession(id)
		u, err := rt.store.Session(ctx, id)
		if err == nil && u.Expires <= now {
			err = rt.closeUploadSession(ctx, u, "")
		}
		unlock()
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			log.Warn().Err(err).Msg("failed to clean up expired upload session")
		}
	}
	return rt.store.ExpireReceipts(ctx, now)
}
