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
	mux.Handle("/api/browse/rename", web.Handler(h.handleRename))
	mux.Handle("/api/browse/delete", web.Handler(h.handleDelete))
	mux.Handle("/api/browse/upload", web.Handler(h.handleUpload))
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

func (h *Handler) handleRename(w http.ResponseWriter, r *http.Request) *web.Error {
	if r.Method != http.MethodPost {
		return &web.Error{
			Code:    http.StatusMethodNotAllowed,
			Message: "Method not allowed",
		}
	}

	var req struct {
		OldPath string `json:"oldPath"`
		NewName string `json:"newName"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return &web.Error{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
			Err:     err,
		}
	}

	if req.OldPath == "" || req.NewName == "" {
		return &web.Error{
			Code:    http.StatusBadRequest,
			Message: "oldPath and newName are required",
		}
	}

	// Check if the path is allowed (within a Space)
	allowed, err := h.isPathAllowed(r.Context(), req.OldPath)
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

	// 새 경로 생성
	dir := filepath.Dir(req.OldPath)
	newPath := filepath.Join(dir, req.NewName)

	// 새 경로도 허용된 Space 내부인지 확인
	allowed, err = h.isPathAllowed(r.Context(), newPath)
	if err != nil {
		return &web.Error{
			Code:    http.StatusInternalServerError,
			Message: "Failed to validate new path",
			Err:     err,
		}
	}
	if !allowed {
		return &web.Error{
			Code:    http.StatusForbidden,
			Message: "Access denied: new path is not within any allowed Space",
		}
	}

	// 파일/폴더 존재 확인
	if _, err := os.Stat(req.OldPath); err != nil {
		if os.IsNotExist(err) {
			return &web.Error{
				Code:    http.StatusNotFound,
				Message: "File or directory not found",
				Err:     err,
			}
		}
		return &web.Error{
			Code:    http.StatusInternalServerError,
			Message: "Failed to access file or directory",
			Err:     err,
		}
	}

	// 이름 변경
	if err := os.Rename(req.OldPath, newPath); err != nil {
		return &web.Error{
			Code:    http.StatusInternalServerError,
			Message: "Failed to rename file or directory",
			Err:     err,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Successfully renamed"})

	return nil
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) *web.Error {
	if r.Method != http.MethodPost {
		return &web.Error{
			Code:    http.StatusMethodNotAllowed,
			Message: "Method not allowed",
		}
	}

	var req struct {
		Path string `json:"path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return &web.Error{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
			Err:     err,
		}
	}

	if req.Path == "" {
		return &web.Error{
			Code:    http.StatusBadRequest,
			Message: "path is required",
		}
	}

	// Check if the path is allowed (within a Space)
	allowed, err := h.isPathAllowed(r.Context(), req.Path)
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

	// 파일/폴더 존재 확인
	fileInfo, err := os.Stat(req.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return &web.Error{
				Code:    http.StatusNotFound,
				Message: "File or directory not found",
				Err:     err,
			}
		}
		return &web.Error{
			Code:    http.StatusInternalServerError,
			Message: "Failed to access file or directory",
			Err:     err,
		}
	}

	// 삭제
	var deleteErr error
	if fileInfo.IsDir() {
		deleteErr = os.RemoveAll(req.Path)
	} else {
		deleteErr = os.Remove(req.Path)
	}

	if deleteErr != nil {
		return &web.Error{
			Code:    http.StatusInternalServerError,
			Message: "Failed to delete file or directory",
			Err:     deleteErr,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Successfully deleted"})

	return nil
}

func (h *Handler) handleUpload(w http.ResponseWriter, r *http.Request) *web.Error {
	if r.Method != http.MethodPost {
		return &web.Error{
			Code:    http.StatusMethodNotAllowed,
			Message: "Method not allowed",
		}
	}

	// Parse multipart form (최대 32MB)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return &web.Error{
			Code:    http.StatusBadRequest,
			Message: "Failed to parse multipart form",
			Err:     err,
		}
	}

	// Get target directory path
	targetPath := r.FormValue("targetPath")
	if targetPath == "" {
		return &web.Error{
			Code:    http.StatusBadRequest,
			Message: "targetPath is required",
		}
	}

	// Check if the target path is allowed (within a Space)
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

	// Check if target directory exists
	fileInfo, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &web.Error{
				Code:    http.StatusNotFound,
				Message: "Target directory not found",
				Err:     err,
			}
		}
		return &web.Error{
			Code:    http.StatusInternalServerError,
			Message: "Failed to access target directory",
			Err:     err,
		}
	}

	// Ensure target is a directory
	if !fileInfo.IsDir() {
		return &web.Error{
			Code:    http.StatusBadRequest,
			Message: "Target path must be a directory",
		}
	}

	// Get uploaded file
	file, header, err := r.FormFile("file")
	if err != nil {
		return &web.Error{
			Code:    http.StatusBadRequest,
			Message: "Failed to get uploaded file",
			Err:     err,
		}
	}
	defer file.Close()

	// Create destination file path
	destPath := filepath.Join(targetPath, header.Filename)

	// Check overwrite parameter
	overwrite := r.FormValue("overwrite") == "true"

	// Check if destination file already exists
	if _, err := os.Stat(destPath); err == nil {
		if !overwrite {
			return &web.Error{
				Code:    http.StatusConflict,
				Message: "File already exists",
			}
		}
		// If overwrite is true, we'll continue and os.Create will overwrite the file
	}

	// Create destination file (overwrites if exists)
	destFile, err := os.Create(destPath)
	if err != nil {
		return &web.Error{
			Code:    http.StatusInternalServerError,
			Message: "Failed to create destination file",
			Err:     err,
		}
	}
	defer destFile.Close()

	// Copy uploaded file to destination
	if _, err := io.Copy(destFile, file); err != nil {
		// Clean up on failure
		os.Remove(destPath)
		return &web.Error{
			Code:    http.StatusInternalServerError,
			Message: "Failed to save uploaded file",
			Err:     err,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message":  "Successfully uploaded",
		"filename": header.Filename,
	})

	return nil
}
