package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"taeu.kr/cohesion/internal/browse"
	"taeu.kr/cohesion/internal/platform/web"
	"taeu.kr/cohesion/internal/space"
)

type Handler struct {
	browseService *browse.Service
	spaceService  *space.Service
}

func NewHandler(browseService *browse.Service, spaceService *space.Service) *Handler {
	return &Handler{
		browseService: browseService,
		spaceService:  spaceService,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Space 생성용 시스템 탐색 API
	mux.Handle("/api/browse", web.Handler(h.handleBrowse))
	mux.Handle("/api/browse/base-directories", web.Handler(h.handleBaseDirectories))
}

// isPathAllowed는 경로가 등록된 Space 내부인지 확인합니다.
func (h *Handler) isPathAllowed(ctx context.Context, requestPath string) (bool, error) {
	cleanPath := filepath.Clean(requestPath)

	spaces, err := h.spaceService.GetAllSpaces(ctx)
	if err != nil {
		return false, err
	}

	if len(spaces) == 0 {
		return true, nil
	}

	for _, s := range spaces {
		spacePath := filepath.Clean(s.SpacePath)
		if cleanPath == spacePath || strings.HasPrefix(cleanPath, spacePath+string(filepath.Separator)) {
			return true, nil
		}
	}

	return false, nil
}

func (h *Handler) handleBaseDirectories(w http.ResponseWriter, r *http.Request) *web.Error {
	dirs := h.browseService.GetBaseDirectories()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(dirs); err != nil {
		return &web.Error{
			Code:    http.StatusInternalServerError,
			Message: "Failed to encode response",
			Err:     err,
		}
	}
	return nil
}

func (h *Handler) handleBrowse(w http.ResponseWriter, r *http.Request) *web.Error {
	path := r.URL.Query().Get("path")
	systemMode := r.URL.Query().Get("system") == "true"

	targetPath := path
	if targetPath == "" {
		targetPath = h.browseService.GetInitialBrowseRoot()
	}

	// system=true이면 Space 검증 스킵 (Space 생성용)
	if !systemMode {
		allowed, err := h.isPathAllowed(r.Context(), targetPath)
		if err != nil {
			return &web.Error{
				Code:    http.StatusInternalServerError,
				Message: "Failed to validate path",
				Err:     err,
			}
		}
		if !allowed {
			return &web.Error{
				Code:    http.StatusForbidden,
				Message: "Access denied: path is not within any allowed Space",
			}
		}
	}

	files, err := h.browseService.ListDirectory(false, targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &web.Error{Code: http.StatusNotFound, Message: "Directory not found", Err: err}
		}
		if os.IsPermission(err) {
			return &web.Error{Code: http.StatusForbidden, Message: "Permission denied", Err: err}
		}
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to list directory", Err: err}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(files); err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to encode response", Err: err}
	}
	return nil
}
