package handler

import (
	"context"
	"encoding/json"
	"io"
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
	mux.Handle("/api/browse", web.Handler(h.handleBrowse))
	mux.Handle("/api/browse/base-directories", web.Handler(h.handleBaseDirectories))
	mux.Handle("/api/browse/download", web.Handler(h.handleDownload))
}

// isPathAllowed checks if the given path is within any allowed Space
func (h *Handler) isPathAllowed(ctx context.Context, requestPath string) (bool, error) {
	// Clean the request path
	cleanPath := filepath.Clean(requestPath)

	// Get all spaces
	spaces, err := h.spaceService.GetAllSpaces(ctx)
	if err != nil {
		return false, err
	}

	// If no spaces exist, allow access (for backward compatibility)
	if len(spaces) == 0 {
		return true, nil
	}

	// Check if the path is within any space
	for _, s := range spaces {
		spacePath := filepath.Clean(s.SpacePath)
		// Check if requestPath is within or equal to spacePath
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

	var files []browse.FileInfo
	var err error

	targetPath := path
	if targetPath == "" {
		// path가 비어있으면 초기 디렉토리 사용
		targetPath = h.browseService.GetInitialBrowseRoot()
	}

	// Check if the path is allowed (within a Space)
	// Skip validation in system mode (for Space creation)
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

	files, err = h.browseService.ListDirectory(false, targetPath)

	if err != nil {
		if os.IsNotExist(err) {
			return &web.Error{
				Code:    http.StatusNotFound,
				Message: "Directory not found",
				Err:     err,
			}
		}
		if os.IsPermission(err) {
			return &web.Error{
				Code:    http.StatusForbidden,
				Message: "Permission denied",
				Err:     err,
			}
		}
		return &web.Error{
			Code:    http.StatusInternalServerError,
			Message: "Failed to list directory",
			Err:     err,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(files); err != nil {
		return &web.Error{
			Code:    http.StatusInternalServerError,
			Message: "Failed to encode response",
			Err:     err,
		}
	}
	return nil
}

func (h *Handler) handleDownload(w http.ResponseWriter, r *http.Request) *web.Error {
	path := r.URL.Query().Get("path")
	if path == "" {
		return &web.Error{
			Code:    http.StatusBadRequest,
			Message: "Path parameter is required",
		}
	}

	// Check if the path is allowed (within a Space)
	allowed, err := h.isPathAllowed(r.Context(), path)
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

	// 파일 존재 및 디렉토리 체크
	fileInfo, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &web.Error{
				Code:    http.StatusNotFound,
				Message: "File not found",
				Err:     err,
			}
		}
		if os.IsPermission(err) {
			return &web.Error{
				Code:    http.StatusForbidden,
				Message: "Permission denied",
				Err:     err,
			}
		}
		return &web.Error{
			Code:    http.StatusInternalServerError,
			Message: "Failed to access file",
			Err:     err,
		}
	}

	// 디렉토리는 다운로드 불가
	if fileInfo.IsDir() {
		return &web.Error{
			Code:    http.StatusBadRequest,
			Message: "Cannot download directory",
		}
	}

	// 파일 열기
	file, err := os.Open(path)
	if err != nil {
		return &web.Error{
			Code:    http.StatusInternalServerError,
			Message: "Failed to open file",
			Err:     err,
		}
	}
	defer file.Close()

	// Content-Disposition 헤더 설정 (파일명)
	fileName := filepath.Base(path)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+fileName+"\"")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", string(fileInfo.Size()))

	// 파일 내용 전송
	if _, err := io.Copy(w, file); err != nil {
		return &web.Error{
			Code:    http.StatusInternalServerError,
			Message: "Failed to send file",
			Err:     err,
		}
	}

	return nil
}
