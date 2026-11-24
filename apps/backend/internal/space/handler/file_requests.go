package handler

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"taeu.kr/cohesion/internal/auth"
	"taeu.kr/cohesion/internal/platform/web"
	store "taeu.kr/cohesion/internal/space/store"
)

func fileRequestUnavailable() *web.Error {
	return &web.Error{Code: 404, Message: "File request unavailable"}
}

func (h *Handler) handleFileRequests(w http.ResponseWriter, r *http.Request, spaceID int64) *web.Error {
	if r.URL.Path != "/api/spaces/"+strconv.FormatInt(spaceID, 10)+"/files/file-requests" {
		return fileRequestUnavailable()
	}
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims.UserID <= 0 {
		return &web.Error{Code: 401, Message: "Unauthorized"}
	}
	rt := h.transferRuntime
	if rt == nil {
		return &web.Error{Code: 503, Message: "Transfer storage unavailable"}
	}
	w.Header().Set("Cache-Control", "no-store")
	ctx := r.Context()
	folder := filepath.Clean(r.URL.Query().Get("folder"))
	switch r.Method {
	case http.MethodGet:
		list, err := rt.store.FileRequests(ctx, claims.UserID, spaceID, folder)
		if err != nil {
			return uploadIOError(err)
		}
		return writeJSON(w, 200, list)
	case http.MethodPost:
		var req struct {
			Days      int
			ByteLimit int64
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil || req.Days < 1 || req.Days > 365 || req.ByteLimit < 1 || req.ByteLimit > 1<<53-1 {
			return &web.Error{Code: 400, Message: "Invalid expiry or byte limit"}
		}
		space, e := h.getSpace(r, spaceID)
		if e != nil {
			return e
		}
		root, err := filepath.Abs(space.SpacePath)
		if err != nil {
			return uploadIOError(err)
		}
		dir, err := openUploadDir(&store.UploadSession{Root: root, Path: folder})
		if err != nil {
			return &web.Error{Code: 400, Message: "Invalid request folder"}
		}
		dir.Close()
		title := filepath.Base(folder)
		if folder == "." {
			title = space.SpaceName
		}
		token, err := generateDownloadTicketToken()
		if err != nil {
			return uploadIOError(err)
		}
		link := &store.FileRequest{Owner: claims.UserID, SpaceID: spaceID, Root: root, Folder: folder, Token: token, Title: title, Expires: time.Now().AddDate(0, 0, req.Days).Unix(), ByteLimit: req.ByteLimit}
		if err := rt.store.CreateFileRequest(ctx, link); err != nil {
			return uploadIOError(err)
		}
		return writeJSON(w, 201, link)
	case http.MethodDelete:
		id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if err != nil {
			return fileRequestUnavailable()
		}
		rt.requestPublishMu.Lock()
		err = rt.store.CloseFileRequest(ctx, id, claims.UserID, spaceID)
		rt.requestPublishMu.Unlock()
		if errors.Is(err, sql.ErrNoRows) {
			return fileRequestUnavailable()
		}
		if err != nil {
			return uploadIOError(err)
		}
		w.WriteHeader(204)
		return nil
	default:
		return &web.Error{Code: 405, Message: "Method not allowed"}
	}
}

func (h *Handler) activeFileRequest(r *http.Request) (*store.FileRequest, *web.Error) {
	token := r.Header.Get("X-File-Request")
	if !validTransferID(token) || h.transferRuntime == nil || h.canReceiveFiles == nil {
		return nil, fileRequestUnavailable()
	}
	f, err := h.transferRuntime.store.FileRequestForToken(r.Context(), token)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fileRequestUnavailable()
	}
	if err != nil {
		return nil, uploadIOError(err)
	}
	allowed, err := h.canReceiveFiles(r.Context(), f.Owner, f.SpaceID)
	if err != nil || !allowed {
		return nil, fileRequestUnavailable()
	}
	return f, nil
}

func readSubmissionInput(w http.ResponseWriter, r *http.Request) (uploadSessionInput, *web.Error) {
	var input struct {
		Filename string
		Size     *int64
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&input); err != nil || input.Size == nil {
		return uploadSessionInput{}, &web.Error{Code: 400, Message: "Invalid submission"}
	}
	return uploadSessionInput{Filename: input.Filename, Size: input.Size, ConflictPolicy: "rename"}, nil
}

// The upload ID is never accepted as a credential. Each submission requires its own random key.
func submissionID(token, key string) string {
	digest := sha256.Sum256([]byte(token + "." + key))
	return base64.RawURLEncoding.EncodeToString(digest[:24])
}

func (h *Handler) handleFileDrop(w http.ResponseWriter, r *http.Request) *web.Error {
	w.Header().Set("Cache-Control", "no-store")
	f, e := h.activeFileRequest(r)
	if e != nil {
		return e
	}
	key := r.Header.Get("X-Submission-Key")
	if key == "" && r.Method == http.MethodGet {
		return writeJSON(w, 200, map[string]any{"title": f.Title, "expires": f.Expires})
	}
	if !validTransferID(key) {
		return &web.Error{Code: 400, Message: "Submission key required"}
	}
	rt := h.transferRuntime
	id := submissionID(f.Token, key)
	unlock := rt.lockSession(id)
	defer unlock()
	ctx := r.Context()
	u, err := rt.store.Session(ctx, id)
	if err == nil && u.Expires <= time.Now().Unix() && r.Method == http.MethodPost {
		// The janitor may not have reclaimed this expired submission yet.
		if err := rt.closeUploadSession(ctx, u, ""); err != nil {
			return uploadIOError(err)
		}
		err = sql.ErrNoRows
	}
	if errors.Is(err, sql.ErrNoRows) {
		status, _, err := rt.store.UploadReceipt(ctx, id, 0, f.SpaceID, time.Now().Unix())
		if err == nil {
			return sendUploadResponse(w, uploadSessionResponse{Status: status})
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return uploadIOError(err)
		}
		if r.Method != http.MethodPost {
			return fileRequestUnavailable()
		}
		req, e := readSubmissionInput(w, r)
		if e != nil {
			return e
		}
		req.Path = f.Folder
		return h.beginUploadSession(w, r, f.SpaceID, 0, req, f, id)
	}
	if err != nil {
		return uploadIOError(err)
	}
	if u.RequestID != f.ID || u.Owner != 0 || u.SpaceID != f.SpaceID || u.Expires <= time.Now().Unix() {
		return fileRequestUnavailable()
	}
	switch r.Method {
	case http.MethodGet:
	case http.MethodPost:
		req, e := readSubmissionInput(w, r)
		if e != nil {
			return e
		}
		if req.Filename != u.Name || *req.Size != u.Size {
			return &web.Error{Code: 409, Message: "Submission metadata changed"}
		}
	case http.MethodPatch:
		if e := h.appendUploadChunk(r, u); e != nil {
			return e
		}
	case http.MethodPut:
		return h.completeUploadSession(w, r, u)
	case http.MethodDelete:
		if err := rt.closeUploadSession(ctx, u, ""); err != nil {
			return uploadIOError(err)
		}
		w.WriteHeader(204)
		return nil
	default:
		return &web.Error{Code: 405, Message: "Method not allowed"}
	}
	return sendUploadResponse(w, uploadSessionResponse{Status: "active", Offset: u.Offset})
}
