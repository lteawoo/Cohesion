package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"taeu.kr/cohesion/internal/audit"
	"taeu.kr/cohesion/internal/auth"
	"taeu.kr/cohesion/internal/platform/web"
	"taeu.kr/cohesion/internal/space"
	store "taeu.kr/cohesion/internal/space/store"
)

const maxUploadChunk = 4 << 20
const uploadSessionTTL = 24 * time.Hour

type uploadSessionResponse struct {
	ID       string `json:"id,omitempty"`
	Offset   int64  `json:"offset"`
	Status   string `json:"status"`
	Filename string `json:"filename,omitempty"`
}

func sendUploadResponse(w http.ResponseWriter, result uploadSessionResponse) *web.Error {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		return uploadIOError(err)
	}
	return nil
}

func uploadIOError(err error) *web.Error {
	return &web.Error{Code: 500, Message: "Failed to process upload session", Err: err}
}

func sessionExpiry() int64 { return time.Now().Add(uploadSessionTTL).Unix() }

func (h *Handler) handleUploadSessions(w http.ResponseWriter, r *http.Request, spaceID int64) *web.Error {
	if r.URL.Path != "/api/spaces/"+strconv.FormatInt(spaceID, 10)+"/files/upload-sessions" {
		return &web.Error{Code: 404, Message: "Unknown upload route"}
	}
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims.UserID <= 0 {
		return &web.Error{Code: 401, Message: "Unauthorized"}
	}
	rt := h.transferRuntime
	if rt == nil {
		return &web.Error{Code: 503, Message: "Transfer storage unavailable"}
	}
	id := r.URL.Query().Get("id")
	if id == "" && r.Method == http.MethodPost {
		return h.createUploadSession(w, r, spaceID, claims.UserID)
	}
	missing := &web.Error{Code: 404, Message: "Upload session expired or not found"}
	if !validTransferID(id) {
		return missing
	}
	unlock := rt.lockSession(id)
	defer unlock()
	ctx := r.Context()
	u, err := rt.store.Session(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		status, name, err := rt.store.UploadReceipt(ctx, id, claims.UserID, spaceID, time.Now().Unix())
		if errors.Is(err, sql.ErrNoRows) {
			return missing
		}
		if err != nil {
			return uploadIOError(err)
		}
		return sendUploadResponse(w, uploadSessionResponse{ID: id, Status: status, Filename: name})
	}
	if err != nil {
		return uploadIOError(err)
	}
	if u.Owner != claims.UserID || u.SpaceID != spaceID {
		return missing
	}
	if u.Expires <= time.Now().Unix() {
		return missing
	}
	switch r.Method {
	case http.MethodGet:
	case http.MethodPatch:
		if e := h.appendUploadChunk(r, u); e != nil {
			return e
		}
	case http.MethodPost:
		return h.completeUploadSession(w, r, u)
	case http.MethodDelete:
		if err := rt.closeUploadSession(ctx, u, ""); err != nil {
			return uploadIOError(err)
		}
		w.WriteHeader(http.StatusNoContent)
		return nil
	default:
		return &web.Error{Code: 405, Message: "Method not allowed"}
	}
	return sendUploadResponse(w, uploadSessionResponse{ID: id, Offset: u.Offset, Status: "active"})
}

type uploadSessionInput struct {
	Path, Filename, ConflictPolicy string
	Size                           *int64
}

func (h *Handler) createUploadSession(w http.ResponseWriter, r *http.Request, spaceID, owner int64) *web.Error {
	var req uploadSessionInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&req); err != nil {
		return &web.Error{Code: 400, Message: "Invalid upload request"}
	}
	return h.beginUploadSession(w, r, spaceID, owner, req, nil, "")
}

func (h *Handler) beginUploadSession(w http.ResponseWriter, r *http.Request, spaceID, owner int64, req uploadSessionInput, link *store.FileRequest, id string) *web.Error {
	if req.Size == nil || *req.Size < 0 || *req.Size > 1<<53-1 {
		return &web.Error{Code: 400, Message: "Invalid upload size"}
	}
	if !filepath.IsLocal(req.Filename) || strings.ContainsAny(req.Filename, "/\\\x00") || req.Filename == "." || strings.TrimSpace(req.Filename) == "" || ensureNameIsNotTrashDirectory(req.Filename) != nil || strings.HasPrefix(req.Filename, uploadStagePrefix) {
		return &web.Error{Code: 400, Message: "Invalid filename"}
	}
	data, e := h.getSpace(r, spaceID)
	if e != nil {
		return e
	}
	root, err := filepath.Abs(data.SpacePath)
	if err != nil {
		return uploadIOError(err)
	}
	u := &store.UploadSession{Owner: owner, SpaceID: spaceID, Root: root, Path: filepath.Clean(req.Path), Name: req.Filename, Policy: req.ConflictPolicy, Size: *req.Size, Expires: sessionExpiry()}
	if link != nil {
		if root != link.Root {
			return fileRequestUnavailable()
		}
		u.RequestID = link.ID
	}
	dir, err := openUploadDir(u)
	if err != nil {
		return &web.Error{Code: 400, Message: "Invalid upload directory", Err: err}
	}
	defer dir.Close()
	plan, e := h.sessionUploadPlan(r.Context(), u, dir)
	if e != nil {
		return e
	}
	if plan.skip {
		h.recordSpaceAudit(r, audit.Event{Action: "file.upload", Result: audit.ResultPartial, Target: filepath.ToSlash(filepath.Join(u.Path, u.Name)), Metadata: map[string]any{"status": "skipped"}}, spaceID)
		return sendUploadResponse(w, uploadSessionResponse{Status: "skipped", Filename: u.Name})
	}
	u.ID = id
	if u.ID == "" {
		u.ID, err = generateDownloadTicketToken()
	}
	if err != nil {
		return uploadIOError(err)
	}
	if e := h.reserveSession(r.Context(), u, plan); e != nil {
		return e
	}
	rt := h.transferRuntime
	file, stage, err := rt.createUploadStage(r.Context(), "")
	if err != nil {
		rt.quota.ReleaseWriteReservation(u.ID)
		return uploadIOError(err)
	}
	u.Stage = stage
	if err = file.Close(); err == nil {
		if link != nil {
			err = rt.store.StartFileRequestUpload(r.Context(), u)
		} else {
			err = rt.store.SaveSession(r.Context(), u)
		}
	}
	if err != nil {
		h.discardUploadStage(stage)
		rt.quota.ReleaseWriteReservation(u.ID)
		if errors.Is(err, store.ErrFileRequestFull) {
			return &web.Error{Code: 409, Message: "Request capacity unavailable"}
		}
		return uploadIOError(err)
	}
	if link != nil {
		return sendUploadResponse(w, uploadSessionResponse{Status: "active"})
	}
	return sendUploadResponse(w, uploadSessionResponse{ID: u.ID, Status: "active"})
}

func openUploadDir(u *store.UploadSession) (*os.Root, error) {
	if !filepath.IsLocal(u.Path) || ensurePathOutsideTrash(u.Path) != nil {
		return nil, os.ErrPermission
	}
	root, err := os.OpenRoot(u.Root)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.OpenRoot(u.Path)
}

func (h *Handler) sessionUploadPlan(ctx context.Context, u *store.UploadSession, dir *os.Root) (*uploadPlan, *web.Error) {
	if u.RequestID != 0 {
		window, e := h.calculateUploadQuotaWindow(ctx, u.SpaceID, 0, u.Size)
		if e != nil {
			return nil, e
		}
		name := u.Name
		for i := 1; ; i++ {
			_, err := dir.Lstat(name)
			if errors.Is(err, os.ErrNotExist) {
				return &uploadPlan{resultFileName: name, quotaWindow: window}, nil
			}
			if err != nil {
				return nil, uploadIOError(err)
			}
			name = numberedUploadName(u.Name, i)
		}
	}
	info, err := dir.Lstat(u.Name)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		return nil, &web.Error{Code: 409, Message: "Destination is a symbolic link"}
	}
	return h.buildUploadPlan(ctx, u.SpaceID, u.Root, u.Path, u.Name, u.Size, u.Policy, false)
}

func (h *Handler) reserveSession(ctx context.Context, u *store.UploadSession, plan *uploadPlan) *web.Error {
	delta := u.Size - plan.existingBytes
	if !plan.quotaWindow.enabled || delta <= 0 {
		return nil
	}
	_, err := h.quotaService.AcquireWriteReservation(ctx, u.SpaceID, u.ID, delta)
	if err == nil {
		return nil
	}
	var exceeded *space.QuotaExceededError
	if errors.As(err, &exceeded) {
		return &web.Error{Code: 507, Message: "Space quota exceeded", Err: err}
	}
	return uploadIOError(err)
}

func (h *Handler) appendUploadChunk(r *http.Request, u *store.UploadSession) *web.Error {
	offset, err := strconv.ParseInt(r.Header.Get("Upload-Offset"), 10, 64)
	if err != nil {
		return &web.Error{Code: 400, Message: "Invalid Upload-Offset"}
	}
	if offset != u.Offset || u.Destination != "" {
		return &web.Error{Code: 409, Message: "Upload offset mismatch"}
	}
	length := r.ContentLength
	if length <= 0 || length > maxUploadChunk || length > u.Size-offset {
		return &web.Error{Code: 400, Message: "Invalid chunk length"}
	}
	f, err := os.OpenFile(u.Stage, os.O_WRONLY, 0)
	if err != nil {
		return uploadIOError(err)
	}
	defer f.Close()
	// An interrupted request can leave an unacknowledged tail. Overwrite it on retry.
	if _, err = f.Seek(offset, io.SeekStart); err != nil {
		return uploadIOError(err)
	}
	if _, err = io.CopyN(f, r.Body, length); err != nil {
		return &web.Error{Code: 400, Message: "Incomplete upload chunk", Err: err}
	}
	if err = f.Sync(); err != nil {
		return uploadIOError(err)
	}
	u.Offset += length
	u.Expires = sessionExpiry()
	if err = h.transferRuntime.store.SaveSession(r.Context(), u); err != nil {
		return uploadIOError(err)
	}
	return nil
}
