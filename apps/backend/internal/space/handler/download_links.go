package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"

	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"taeu.kr/cohesion/internal/auth"
	"taeu.kr/cohesion/internal/config"
	"taeu.kr/cohesion/internal/platform/web"
	store "taeu.kr/cohesion/internal/space/store"
)

func unavailableDownload() *web.Error {
	return &web.Error{Code: http.StatusNotFound, Message: "Shared file unavailable"}
}

func (h *Handler) handleDownloadLinks(w http.ResponseWriter, r *http.Request, spaceID int64) *web.Error {
	w.Header().Set("Cache-Control", "no-store")
	if r.URL.Path != "/api/spaces/"+strconv.FormatInt(spaceID, 10)+"/files/download-links" {
		return unavailableDownload()
	}
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims.UserID <= 0 {
		return &web.Error{Code: 401, Message: "Unauthorized"}
	}
	if h.transferRuntime == nil || h.canShareFiles == nil {
		return &web.Error{Code: 503, Message: "Sharing unavailable"}
	}
	allowed, err := h.canShareFiles(r.Context(), claims.UserID, spaceID)
	if err != nil {
		return uploadIOError(err)
	}
	if !allowed {
		return &web.Error{Code: 403, Message: "Space management and file read access required"}
	}

	rt := h.transferRuntime
	switch r.Method {
	case http.MethodGet:
		list, err := rt.store.DownloadLinks(r.Context(), spaceID, r.URL.Query().Get("path"), time.Now().Unix())
		if err != nil {
			return uploadIOError(err)
		}
		origin := config.PublicShareOrigin()
		for i := range list {
			list[i].PublicOrigin = origin
		}
		return writeJSON(w, 200, list)
	case http.MethodPost:
		return h.createDownloadLink(w, r, claims.UserID, spaceID)
	case http.MethodDelete:
		id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if err != nil || id <= 0 {
			return unavailableDownload()
		}
		rt.shared.mu.Lock()
		defer rt.shared.mu.Unlock()
		snapshot, err := rt.store.DeleteDownloadLink(r.Context(), id, spaceID)
		if errors.Is(err, sql.ErrNoRows) {
			return unavailableDownload()
		}
		if err != nil {
			return uploadIOError(err)
		}
		if validTransferID(snapshot) {
			_ = os.Remove(rt.shared.filename(snapshot))
		} // The janitor retries files held by ongoing downloads.
		w.WriteHeader(204)
		return nil
	default:
		return &web.Error{Code: 405, Message: "Method not allowed"}
	}
}

func (h *Handler) createDownloadLink(w http.ResponseWriter, r *http.Request, owner, spaceID int64) *web.Error {
	var input struct {
		Path  string
		Paths []string
		Days  int
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 512<<10)).Decode(&input); err != nil || input.Days < 1 || input.Days > 365 {
		return &web.Error{Code: 400, Message: "Invalid path or expiry"}
	}
	ctx := r.Context()
	rt := h.transferRuntime
	shared := rt.shared
	select {
	case shared.slots <- struct{}{}:
		defer func() { <-shared.slots }()
	default:
		return &web.Error{Code: 429, Message: "Snapshot creation is busy"}
	}
	spaces, err := h.spaceService.GetAllSpaces(ctx)
	if err != nil {
		return uploadIOError(err)
	}
	for _, s := range spaces {
		overlap, err := h.spaceService.OverlapsDownloadStorage(s.SpacePath)
		if err != nil {
			return uploadIOError(err)
		}
		if overlap {
			return &web.Error{Code: 409, Message: "Space root overlaps private download link storage"}
		}
	}
	spaceInfo, e := h.getSpace(r, spaceID)
	if e != nil {
		return e
	}
	paths := input.Paths
	if paths == nil {
		paths = []string{input.Path}
	} else if input.Path != "" || len(paths) < 2 || len(paths) > 100 {
		return &web.Error{Code: 400, Message: "Select between 2 and 100 files"}
	}
	selection, err := openDownloadSelection(spaceInfo.SpacePath, paths)
	if err != nil {
		return &web.Error{Code: 400, Message: "Select distinct regular files inside the space"}
	}
	defer selection.close()
	token, err := generateDownloadTicketToken()
	if err != nil {
		return uploadIOError(err)
	}
	snapshot, err := generateDownloadTicketToken()
	if err != nil {
		return uploadIOError(err)
	}
	link := store.DownloadLink{Owner: owner, SpaceID: spaceID, Path: input.Path, Name: selection.files[0].item.Name, Size: selection.size, Token: token, Snapshot: snapshot}
	if len(selection.files) > 1 {
		link.Name = store.BundleDownloadName
		link.ItemCount = len(selection.files)
		for _, file := range selection.files {
			link.Items = append(link.Items, file.item)
		}
	}
	if err := shared.reserve(ctx, rt.store, link, spaceInfo.QuotaBytes); err != nil {
		if errors.Is(err, errSnapshotQuota) {
			return &web.Error{Code: 507, Message: "Space snapshot quota exceeded"}
		}
		return uploadIOError(err)
	}
	defer func() { shared.mu.Lock(); delete(shared.pending, snapshot); shared.mu.Unlock() }()
	filename := shared.filename(snapshot)
	dest, err := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return snapshotCopyError(err)
	}
	published := false
	defer func() {
		if !published {
			_ = os.Remove(filename)
		}
	}()
	err = selection.copy(ctx, dest)
	closeErr := dest.Close()
	if err != nil {
		return snapshotCopyError(err)
	}
	if closeErr != nil {
		return snapshotCopyError(closeErr)
	}
	allowed, err := h.canShareFiles(ctx, owner, spaceID)
	if err != nil {
		return uploadIOError(err)
	}
	if !allowed {
		return &web.Error{Code: 403, Message: "Space management and file read access required"}
	}
	link.Expires = time.Now().AddDate(0, 0, input.Days).Unix()
	shared.mu.Lock()
	err = rt.store.CreateDownloadLink(ctx, &link)
	if err == nil {
		published = true
		delete(shared.pending, snapshot)
	}
	shared.mu.Unlock()
	if err != nil {
		return uploadIOError(err)
	}
	link.PublicOrigin = config.PublicShareOrigin()
	return writeJSON(w, 201, link)
}

func snapshotCopyError(err error) *web.Error {
	if errors.Is(err, errSnapshotChanged) {
		return &web.Error{Code: 409, Message: "Source changed while creating the download link"}
	}
	if errors.Is(err, syscall.ENOSPC) || errors.Is(err, syscall.EDQUOT) || runtime.GOOS == "windows" && (errors.Is(err, syscall.Errno(112)) || errors.Is(err, syscall.Errno(39))) {
		return &web.Error{Code: 507, Message: "Insufficient snapshot storage"}
	}
	return storageOperationWebError(err, "Failed to copy shared file")
}

func (h *Handler) handleSharedFile(w http.ResponseWriter, r *http.Request) *web.Error {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if r.URL.RawQuery != "" || h.transferRuntime == nil || h.canShareFiles == nil {
		return unavailableDownload()
	}
	var token string
	switch r.Method {
	case http.MethodGet:
		token = r.Header.Get("X-Download-Link")
	case http.MethodPost:
		media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || media != "application/x-www-form-urlencoded" {
			return unavailableDownload()
		}
		r.Body = http.MaxBytesReader(w, r.Body, 512)
		if err = r.ParseForm(); err != nil || len(r.PostForm["token"]) != 1 {
			return unavailableDownload()
		}
		token = r.PostForm.Get("token")
	default:
		return &web.Error{Code: 405, Message: "Method not allowed"}
	}
	if !validTransferID(token) {
		return unavailableDownload()
	}
	link, file, e := h.openDownloadLink(r, token)
	if e != nil {
		if e.Code == 404 && r.Method == http.MethodPost && r.Header.Get("Sec-Fetch-Mode") == "navigate" && strings.Contains(r.Header.Get("Accept"), "text/html") {
			http.Redirect(w, r, "/share", http.StatusSeeOther)
			return nil
		}
		return e
	}
	defer file.Close()
	if r.Method == http.MethodGet {
		return writeJSON(w, 200, struct {
			Name    string                   `json:"name"`
			Size    int64                    `json:"size"`
			Expires int64                    `json:"expires"`
			Items   []store.DownloadLinkItem `json:"items,omitempty"`
		}{link.Name, link.Size, link.Expires, link.Items})
	}
	name := link.Name
	var content io.ReadSeeker = file
	if ids, exists := r.PostForm["item"]; exists {
		if len(ids) != 1 {
			return unavailableDownload()
		}
		id, err := strconv.Atoi(ids[0])
		if err != nil || id < 1 || id > len(link.Items) {
			return unavailableDownload()
		}
		var offset int64
		for _, item := range link.Items[:id-1] {
			offset += item.Size
		}
		item := link.Items[id-1]
		name = item.Name
		content = io.NewSectionReader(file, offset, item.Size)
	} else if len(link.Items) > 0 {
		return streamDownloadBundle(w, r, file, link.Items)
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeContent(w, r, name, time.Time{}, content)
	return nil
}

func (h *Handler) openDownloadLink(r *http.Request, token string) (*store.DownloadLink, *os.File, *web.Error) {
	rt := h.transferRuntime
	rt.shared.mu.Lock()
	defer rt.shared.mu.Unlock()
	link, err := rt.store.ActiveDownloadLink(r.Context(), token, time.Now().Unix())
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, unavailableDownload()
	}
	if err != nil {
		return nil, nil, uploadIOError(err)
	}
	allowed, err := h.canShareFiles(r.Context(), link.Owner, link.SpaceID)
	if err != nil || !allowed || !validTransferID(link.Snapshot) {
		return nil, nil, unavailableDownload()
	}
	file, info, err := openShareSource(rt.shared.dir, link.Snapshot+".snapshot")
	if err != nil {
		return nil, nil, unavailableDownload()
	}
	if info.Size() != link.Size || link.Expires <= time.Now().Unix() {
		file.Close()
		return nil, nil, unavailableDownload()
	}
	return &link, file, nil
}
