package handler

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"taeu.kr/cohesion/internal/platform/web"
	"taeu.kr/cohesion/internal/space"
)

// resolveAbsPath는 Space 경로와 상대 경로를 합쳐 절대 경로를 반환하고 트래버셜 방지 검증을 수행합니다.
func resolveAbsPath(spacePath, relativePath string) (string, error) {
	abs := filepath.Join(spacePath, relativePath)
	if !isPathWithinSpace(abs, spacePath) {
		return "", fmt.Errorf("path traversal detected")
	}
	return abs, nil
}

// handleSpaceFiles는 /api/spaces/{id}/files/* 요청을 액션별로 분기합니다.
func (h *Handler) handleSpaceFiles(w http.ResponseWriter, r *http.Request, spaceID int64, action string) *web.Error {
	switch action {
	case "download":
		return h.handleFileDownload(w, r, spaceID)
	case "rename":
		return h.handleFileRename(w, r, spaceID)
	case "delete":
		return h.handleFileDelete(w, r, spaceID)
	case "delete-multiple":
		return h.handleFileDeleteMultiple(w, r, spaceID)
	case "create-folder":
		return h.handleFileCreateFolder(w, r, spaceID)
	case "upload":
		return h.handleFileUpload(w, r, spaceID)
	case "move":
		return h.handleFileMove(w, r, spaceID)
	case "copy":
		return h.handleFileCopy(w, r, spaceID)
	case "download-multiple":
		return h.handleFileDownloadMultiple(w, r, spaceID)
	default:
		return &web.Error{
			Code:    http.StatusNotFound,
			Message: fmt.Sprintf("Unknown file action: %s", action),
		}
	}
}

// getSpace는 spaceID로 Space를 조회하고 없으면 에러를 반환합니다.
func (h *Handler) getSpace(r *http.Request, spaceID int64) (*space.Space, *web.Error) {
	spaceData, err := h.spaceService.GetSpaceByID(r.Context(), spaceID)
	if err != nil {
		return nil, &web.Error{
			Code:    http.StatusNotFound,
			Message: fmt.Sprintf("Space not found: %v", err),
			Err:     err,
		}
	}
	return spaceData, nil
}

// handleFileDownload: GET /api/spaces/{id}/files/download?path={relativePath}
func (h *Handler) handleFileDownload(w http.ResponseWriter, r *http.Request, spaceID int64) *web.Error {
	spaceData, webErr := h.getSpace(r, spaceID)
	if webErr != nil {
		return webErr
	}

	relativePath := r.URL.Query().Get("path")
	absPath, err := resolveAbsPath(spaceData.SpacePath, relativePath)
	if err != nil {
		return &web.Error{Code: http.StatusForbidden, Message: "Access denied: invalid path"}
	}

	fileInfo, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &web.Error{Code: http.StatusNotFound, Message: "File not found", Err: err}
		}
		if os.IsPermission(err) {
			return &web.Error{Code: http.StatusForbidden, Message: "Permission denied", Err: err}
		}
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to access file", Err: err}
	}

	if fileInfo.IsDir() {
		return h.downloadFolderAsZip(w, absPath, fileInfo.Name())
	}

	return h.streamFileDownload(w, absPath)
}

func (h *Handler) streamFileDownload(w http.ResponseWriter, absPath string) *web.Error {
	file, err := os.Open(absPath)
	if err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to open file", Err: err}
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to inspect file", Err: err}
	}

	fileName := filepath.Base(absPath)
	w.Header().Set("Content-Disposition", `attachment; filename="`+fileName+`"`)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

	if _, err := io.Copy(w, file); err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to send file", Err: err}
	}
	return nil
}

// handleFileRename: POST /api/spaces/{id}/files/rename
// body: { path: string, newName: string }  (path는 Space 상대 경로)
func (h *Handler) handleFileRename(w http.ResponseWriter, r *http.Request, spaceID int64) *web.Error {
	if r.Method != http.MethodPost {
		return &web.Error{Code: http.StatusMethodNotAllowed, Message: "Method not allowed"}
	}

	spaceData, webErr := h.getSpace(r, spaceID)
	if webErr != nil {
		return webErr
	}

	var req struct {
		Path    string `json:"path"`
		NewName string `json:"newName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return &web.Error{Code: http.StatusBadRequest, Message: "Invalid request body", Err: err}
	}
	if req.Path == "" || req.NewName == "" {
		return &web.Error{Code: http.StatusBadRequest, Message: "path and newName are required"}
	}

	absPath, err := resolveAbsPath(spaceData.SpacePath, req.Path)
	if err != nil {
		return &web.Error{Code: http.StatusForbidden, Message: "Access denied: invalid path"}
	}

	newAbsPath := filepath.Join(filepath.Dir(absPath), req.NewName)
	if !isPathWithinSpace(newAbsPath, spaceData.SpacePath) {
		return &web.Error{Code: http.StatusForbidden, Message: "Access denied: new path is outside Space"}
	}

	if _, err := os.Stat(absPath); err != nil {
		if os.IsNotExist(err) {
			return &web.Error{Code: http.StatusNotFound, Message: "File or directory not found", Err: err}
		}
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to access file", Err: err}
	}

	if err := os.Rename(absPath, newAbsPath); err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to rename", Err: err}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Successfully renamed"})
	return nil
}

// handleFileDelete: POST /api/spaces/{id}/files/delete
// body: { path: string }
func (h *Handler) handleFileDelete(w http.ResponseWriter, r *http.Request, spaceID int64) *web.Error {
	if r.Method != http.MethodPost {
		return &web.Error{Code: http.StatusMethodNotAllowed, Message: "Method not allowed"}
	}

	spaceData, webErr := h.getSpace(r, spaceID)
	if webErr != nil {
		return webErr
	}

	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return &web.Error{Code: http.StatusBadRequest, Message: "Invalid request body", Err: err}
	}
	if req.Path == "" {
		return &web.Error{Code: http.StatusBadRequest, Message: "path is required"}
	}

	absPath, err := resolveAbsPath(spaceData.SpacePath, req.Path)
	if err != nil {
		return &web.Error{Code: http.StatusForbidden, Message: "Access denied: invalid path"}
	}

	fileInfo, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &web.Error{Code: http.StatusNotFound, Message: "File or directory not found", Err: err}
		}
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to access file", Err: err}
	}

	var deleteErr error
	if fileInfo.IsDir() {
		deleteErr = os.RemoveAll(absPath)
	} else {
		deleteErr = os.Remove(absPath)
	}
	if deleteErr != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to delete", Err: deleteErr}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Successfully deleted"})
	return nil
}

// handleFileDeleteMultiple: POST /api/spaces/{id}/files/delete-multiple
// body: { paths: []string }
func (h *Handler) handleFileDeleteMultiple(w http.ResponseWriter, r *http.Request, spaceID int64) *web.Error {
	if r.Method != http.MethodPost {
		return &web.Error{Code: http.StatusMethodNotAllowed, Message: "Method not allowed"}
	}

	spaceData, webErr := h.getSpace(r, spaceID)
	if webErr != nil {
		return webErr
	}

	var req struct {
		Paths []string `json:"paths"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return &web.Error{Code: http.StatusBadRequest, Message: "Invalid request body", Err: err}
	}
	if len(req.Paths) == 0 {
		return &web.Error{Code: http.StatusBadRequest, Message: "paths array is required and cannot be empty"}
	}

	type deleteResult struct {
		Path   string `json:"path"`
		Reason string `json:"reason,omitempty"`
	}
	succeeded := []string{}
	failed := []deleteResult{}

	for _, relPath := range req.Paths {
		absPath, err := resolveAbsPath(spaceData.SpacePath, relPath)
		if err != nil {
			failed = append(failed, deleteResult{Path: relPath, Reason: "Access denied: invalid path"})
			continue
		}

		fileInfo, err := os.Stat(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				failed = append(failed, deleteResult{Path: relPath, Reason: "File or directory not found"})
			} else {
				failed = append(failed, deleteResult{Path: relPath, Reason: fmt.Sprintf("Failed to access: %v", err)})
			}
			continue
		}

		var deleteErr error
		if fileInfo.IsDir() {
			deleteErr = os.RemoveAll(absPath)
		} else {
			deleteErr = os.Remove(absPath)
		}
		if deleteErr != nil {
			failed = append(failed, deleteResult{Path: relPath, Reason: fmt.Sprintf("Failed to delete: %v", deleteErr)})
		} else {
			succeeded = append(succeeded, relPath)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"succeeded": succeeded, "failed": failed})
	return nil
}

// handleFileCreateFolder: POST /api/spaces/{id}/files/create-folder
// body: { parentPath: string, folderName: string }
func (h *Handler) handleFileCreateFolder(w http.ResponseWriter, r *http.Request, spaceID int64) *web.Error {
	if r.Method != http.MethodPost {
		return &web.Error{Code: http.StatusMethodNotAllowed, Message: "Method not allowed"}
	}

	spaceData, webErr := h.getSpace(r, spaceID)
	if webErr != nil {
		return webErr
	}

	var req struct {
		ParentPath string `json:"parentPath"`
		FolderName string `json:"folderName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return &web.Error{Code: http.StatusBadRequest, Message: "Invalid request body", Err: err}
	}
	if req.FolderName == "" {
		return &web.Error{Code: http.StatusBadRequest, Message: "folderName is required"}
	}

	absParent, err := resolveAbsPath(spaceData.SpacePath, req.ParentPath)
	if err != nil {
		return &web.Error{Code: http.StatusForbidden, Message: "Access denied: invalid path"}
	}

	parentInfo, err := os.Stat(absParent)
	if err != nil {
		if os.IsNotExist(err) {
			return &web.Error{Code: http.StatusNotFound, Message: "Parent directory not found", Err: err}
		}
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to access parent directory", Err: err}
	}
	if !parentInfo.IsDir() {
		return &web.Error{Code: http.StatusBadRequest, Message: "Parent path is not a directory"}
	}

	if strings.ContainsAny(req.FolderName, "/\\:*?\"<>|") {
		return &web.Error{Code: http.StatusBadRequest, Message: "Folder name contains invalid characters"}
	}

	folderPath := filepath.Join(absParent, req.FolderName)
	if _, err := os.Stat(folderPath); err == nil {
		return &web.Error{Code: http.StatusConflict, Message: "Folder already exists"}
	}

	if err := os.Mkdir(folderPath, 0755); err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to create folder", Err: err}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Successfully created",
		"name":    req.FolderName,
	})
	return nil
}

// handleFileUpload: POST /api/spaces/{id}/files/upload
// multipart form: file, path (상대 경로), overwrite (optional)
func (h *Handler) handleFileUpload(w http.ResponseWriter, r *http.Request, spaceID int64) *web.Error {
	if r.Method != http.MethodPost {
		return &web.Error{Code: http.StatusMethodNotAllowed, Message: "Method not allowed"}
	}

	spaceData, webErr := h.getSpace(r, spaceID)
	if webErr != nil {
		return webErr
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return &web.Error{Code: http.StatusBadRequest, Message: "Failed to parse multipart form", Err: err}
	}

	targetRelPath := r.FormValue("path")
	absTarget, err := resolveAbsPath(spaceData.SpacePath, targetRelPath)
	if err != nil {
		return &web.Error{Code: http.StatusForbidden, Message: "Access denied: invalid path"}
	}

	targetInfo, err := os.Stat(absTarget)
	if err != nil {
		if os.IsNotExist(err) {
			return &web.Error{Code: http.StatusNotFound, Message: "Target directory not found", Err: err}
		}
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to access target directory", Err: err}
	}
	if !targetInfo.IsDir() {
		return &web.Error{Code: http.StatusBadRequest, Message: "Target path must be a directory"}
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		return &web.Error{Code: http.StatusBadRequest, Message: "Failed to get uploaded file", Err: err}
	}
	defer file.Close()

	destPath := filepath.Join(absTarget, header.Filename)
	overwrite := r.FormValue("overwrite") == "true"

	if _, err := os.Stat(destPath); err == nil {
		if !overwrite {
			return &web.Error{Code: http.StatusConflict, Message: "File already exists"}
		}
	}

	destFile, err := os.Create(destPath)
	if err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to create destination file", Err: err}
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, file); err != nil {
		os.Remove(destPath)
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to save uploaded file", Err: err}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Successfully uploaded", "filename": header.Filename})
	return nil
}

// handleFileMove: POST /api/spaces/{id}/files/move
// body: { sources: []string, destination: { spaceId: int64, path: string } }
func (h *Handler) handleFileMove(w http.ResponseWriter, r *http.Request, spaceID int64) *web.Error {
	if r.Method != http.MethodPost {
		return &web.Error{Code: http.StatusMethodNotAllowed, Message: "Method not allowed"}
	}

	srcSpace, webErr := h.getSpace(r, spaceID)
	if webErr != nil {
		return webErr
	}

	var req struct {
		Sources     []string `json:"sources"`
		Destination struct {
			SpaceID int64  `json:"spaceId"`
			Path    string `json:"path"`
		} `json:"destination"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return &web.Error{Code: http.StatusBadRequest, Message: "Invalid request body", Err: err}
	}
	if len(req.Sources) == 0 {
		return &web.Error{Code: http.StatusBadRequest, Message: "sources array is required and cannot be empty"}
	}

	// 대상 Space 조회 (cross-Space 지원)
	dstSpaceID := req.Destination.SpaceID
	if dstSpaceID == 0 {
		dstSpaceID = spaceID
	}
	dstSpace, webErr := h.getSpace(r, dstSpaceID)
	if webErr != nil {
		return webErr
	}

	absDestDir, err := resolveAbsPath(dstSpace.SpacePath, req.Destination.Path)
	if err != nil {
		return &web.Error{Code: http.StatusForbidden, Message: "Access denied: invalid destination path"}
	}

	destDirInfo, err := os.Stat(absDestDir)
	if err != nil {
		if os.IsNotExist(err) {
			return &web.Error{Code: http.StatusNotFound, Message: "Destination directory not found", Err: err}
		}
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to access destination directory", Err: err}
	}
	if !destDirInfo.IsDir() {
		return &web.Error{Code: http.StatusBadRequest, Message: "Destination must be a directory"}
	}

	type moveResult struct {
		Path   string `json:"path"`
		Reason string `json:"reason,omitempty"`
	}
	succeeded := []string{}
	failed := []moveResult{}

	for _, relSrc := range req.Sources {
		absSrc, err := resolveAbsPath(srcSpace.SpacePath, relSrc)
		if err != nil {
			failed = append(failed, moveResult{Path: relSrc, Reason: "Access denied: invalid source path"})
			continue
		}

		if _, err := os.Stat(absSrc); err != nil {
			if os.IsNotExist(err) {
				failed = append(failed, moveResult{Path: relSrc, Reason: "Source not found"})
			} else {
				failed = append(failed, moveResult{Path: relSrc, Reason: fmt.Sprintf("Failed to access source: %v", err)})
			}
			continue
		}

		cleanSrc := filepath.Clean(absSrc)
		cleanDst := filepath.Clean(absDestDir)
		if strings.HasPrefix(cleanDst, cleanSrc+string(filepath.Separator)) {
			failed = append(failed, moveResult{Path: relSrc, Reason: "Cannot move to a subdirectory of itself"})
			continue
		}

		destPath := filepath.Join(absDestDir, filepath.Base(absSrc))
		if _, err := os.Stat(destPath); err == nil {
			failed = append(failed, moveResult{Path: relSrc, Reason: "Destination path already exists"})
			continue
		}

		if err := os.Rename(absSrc, destPath); err != nil {
			failed = append(failed, moveResult{Path: relSrc, Reason: fmt.Sprintf("Failed to move: %v", err)})
		} else {
			succeeded = append(succeeded, relSrc)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"succeeded": succeeded, "failed": failed})
	return nil
}

// handleFileCopy: POST /api/spaces/{id}/files/copy
// body: { sources: []string, destination: { spaceId: int64, path: string } }
func (h *Handler) handleFileCopy(w http.ResponseWriter, r *http.Request, spaceID int64) *web.Error {
	if r.Method != http.MethodPost {
		return &web.Error{Code: http.StatusMethodNotAllowed, Message: "Method not allowed"}
	}

	srcSpace, webErr := h.getSpace(r, spaceID)
	if webErr != nil {
		return webErr
	}

	var req struct {
		Sources     []string `json:"sources"`
		Destination struct {
			SpaceID int64  `json:"spaceId"`
			Path    string `json:"path"`
		} `json:"destination"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return &web.Error{Code: http.StatusBadRequest, Message: "Invalid request body", Err: err}
	}
	if len(req.Sources) == 0 {
		return &web.Error{Code: http.StatusBadRequest, Message: "sources array is required and cannot be empty"}
	}

	dstSpaceID := req.Destination.SpaceID
	if dstSpaceID == 0 {
		dstSpaceID = spaceID
	}
	dstSpace, webErr := h.getSpace(r, dstSpaceID)
	if webErr != nil {
		return webErr
	}

	absDestDir, err := resolveAbsPath(dstSpace.SpacePath, req.Destination.Path)
	if err != nil {
		return &web.Error{Code: http.StatusForbidden, Message: "Access denied: invalid destination path"}
	}

	destDirInfo, err := os.Stat(absDestDir)
	if err != nil {
		if os.IsNotExist(err) {
			return &web.Error{Code: http.StatusNotFound, Message: "Destination directory not found", Err: err}
		}
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to access destination directory", Err: err}
	}
	if !destDirInfo.IsDir() {
		return &web.Error{Code: http.StatusBadRequest, Message: "Destination must be a directory"}
	}

	type copyResult struct {
		Path   string `json:"path"`
		Reason string `json:"reason,omitempty"`
	}
	succeeded := []string{}
	failed := []copyResult{}

	for _, relSrc := range req.Sources {
		absSrc, err := resolveAbsPath(srcSpace.SpacePath, relSrc)
		if err != nil {
			failed = append(failed, copyResult{Path: relSrc, Reason: "Access denied: invalid source path"})
			continue
		}

		sourceInfo, err := os.Stat(absSrc)
		if err != nil {
			if os.IsNotExist(err) {
				failed = append(failed, copyResult{Path: relSrc, Reason: "Source not found"})
			} else {
				failed = append(failed, copyResult{Path: relSrc, Reason: fmt.Sprintf("Failed to access source: %v", err)})
			}
			continue
		}

		cleanSrc := filepath.Clean(absSrc)
		cleanDst := filepath.Clean(absDestDir)
		if strings.HasPrefix(cleanDst, cleanSrc+string(filepath.Separator)) {
			failed = append(failed, copyResult{Path: relSrc, Reason: "Cannot copy to a subdirectory of itself"})
			continue
		}

		destPath := filepath.Join(absDestDir, filepath.Base(absSrc))
		if _, err := os.Stat(destPath); err == nil {
			failed = append(failed, copyResult{Path: relSrc, Reason: "Destination path already exists"})
			continue
		}

		var copyErr error
		if sourceInfo.IsDir() {
			copyErr = copyDir(absSrc, destPath)
		} else {
			copyErr = copyFile(absSrc, destPath)
		}
		if copyErr != nil {
			failed = append(failed, copyResult{Path: relSrc, Reason: fmt.Sprintf("Failed to copy: %v", copyErr)})
		} else {
			succeeded = append(succeeded, relSrc)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"succeeded": succeeded, "failed": failed})
	return nil
}

// handleFileDownloadMultiple: POST /api/spaces/{id}/files/download-multiple
// body: { paths: []string }
func (h *Handler) handleFileDownloadMultiple(w http.ResponseWriter, r *http.Request, spaceID int64) *web.Error {
	if r.Method != http.MethodPost {
		return &web.Error{Code: http.StatusMethodNotAllowed, Message: "Method not allowed"}
	}

	spaceData, webErr := h.getSpace(r, spaceID)
	if webErr != nil {
		return webErr
	}

	var req struct {
		Paths []string `json:"paths"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return &web.Error{Code: http.StatusBadRequest, Message: "Invalid request body", Err: err}
	}
	if len(req.Paths) == 0 {
		return &web.Error{Code: http.StatusBadRequest, Message: "paths array is required and cannot be empty"}
	}

	// 절대 경로 변환 및 검증
	absPaths := make([]string, 0, len(req.Paths))
	for _, relPath := range req.Paths {
		absPath, err := resolveAbsPath(spaceData.SpacePath, relPath)
		if err != nil {
			return &web.Error{Code: http.StatusForbidden, Message: fmt.Sprintf("Access denied: invalid path %s", relPath)}
		}
		if _, err := os.Stat(absPath); err != nil {
			if os.IsNotExist(err) {
				return &web.Error{Code: http.StatusNotFound, Message: fmt.Sprintf("Path not found: %s", relPath), Err: err}
			}
			return &web.Error{Code: http.StatusInternalServerError, Message: fmt.Sprintf("Failed to access path: %s", relPath), Err: err}
		}
		absPaths = append(absPaths, absPath)
	}

	if len(absPaths) == 1 {
		fileInfo, err := os.Stat(absPaths[0])
		if err != nil {
			if os.IsNotExist(err) {
				return &web.Error{Code: http.StatusNotFound, Message: "File not found", Err: err}
			}
			if os.IsPermission(err) {
				return &web.Error{Code: http.StatusForbidden, Message: "Permission denied", Err: err}
			}
			return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to access file", Err: err}
		}

		if fileInfo.IsDir() {
			return h.downloadFolderAsZip(w, absPaths[0], fileInfo.Name())
		}

		return h.streamFileDownload(w, absPaths[0])
	}

	zipFileName := fmt.Sprintf("download-%d.zip", os.Getpid())
	return h.streamZipDownload(w, zipFileName, func(zipWriter *zip.Writer) *web.Error {
		for i, absPath := range absPaths {
			if err := addToZip(zipWriter, absPath, filepath.Base(req.Paths[i])); err != nil {
				log.Printf("Failed to add %s to ZIP: %v", absPath, err)
			}
		}
		return nil
	})
}

// downloadFolderAsZip은 폴더를 zip으로 압축하여 스트리밍합니다.
func (h *Handler) downloadFolderAsZip(w http.ResponseWriter, folderPath string, folderName string) *web.Error {
	zipFileName := folderName + ".zip"
	return h.streamZipDownload(w, zipFileName, func(zipWriter *zip.Writer) *web.Error {
		err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				log.Printf("Skipping file due to error: %s - %v", path, err)
				return nil
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return nil
			}
			relPath, err := filepath.Rel(folderPath, path)
			if err != nil {
				return nil
			}
			if relPath == "." {
				return nil
			}

			header, err := zip.FileInfoHeader(info)
			if err != nil {
				return nil
			}
			header.Name = filepath.ToSlash(relPath)
			header.Method = zip.Deflate

			if info.IsDir() {
				header.Name += "/"
				zipWriter.CreateHeader(header) //nolint:errcheck
				return nil
			}

			writer, err := zipWriter.CreateHeader(header)
			if err != nil {
				return nil
			}
			file, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer file.Close()
			io.Copy(writer, file) //nolint:errcheck
			return nil
		})

		if err != nil {
			return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to create zip archive", Err: err}
		}
		return nil
	})
}

func (h *Handler) streamZipDownload(
	w http.ResponseWriter,
	zipFileName string,
	writeZip func(zipWriter *zip.Writer) *web.Error,
) *web.Error {
	tempFile, err := os.CreateTemp("", "cohesion-download-*.zip")
	if err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to prepare zip archive", Err: err}
	}
	tempFilePath := tempFile.Name()
	defer func() {
		tempFile.Close() //nolint:errcheck
		os.Remove(tempFilePath)
	}()

	zipWriter := zip.NewWriter(tempFile)
	if webErr := writeZip(zipWriter); webErr != nil {
		zipWriter.Close() //nolint:errcheck
		return webErr
	}
	if err := zipWriter.Close(); err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to finalize zip archive", Err: err}
	}

	zipInfo, err := tempFile.Stat()
	if err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to inspect zip archive", Err: err}
	}
	if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to rewind zip archive", Err: err}
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, zipFileName))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", zipInfo.Size()))

	if _, err := io.Copy(w, tempFile); err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to send zip archive", Err: err}
	}
	return nil
}

// addToZip은 파일 또는 디렉토리를 zip에 추가합니다.
func addToZip(zipWriter *zip.Writer, sourcePath string, baseName string) error {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return nil
			}
			relPath, err := filepath.Rel(sourcePath, path)
			if err != nil {
				return nil
			}
			zipPath := filepath.Join(baseName, relPath)
			if relPath == "." {
				zipPath = baseName
			}

			header, err := zip.FileInfoHeader(info)
			if err != nil {
				return nil
			}
			header.Name = filepath.ToSlash(zipPath)
			header.Method = zip.Deflate

			if info.IsDir() {
				if relPath != "." {
					header.Name += "/"
					zipWriter.CreateHeader(header) //nolint:errcheck
				}
				return nil
			}

			writer, err := zipWriter.CreateHeader(header)
			if err != nil {
				return nil
			}
			file, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer file.Close()
			io.Copy(writer, file) //nolint:errcheck
			return nil
		})
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(baseName)
	header.Method = zip.Deflate

	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}

	file, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(writer, file)
	return err
}

// copyFile은 단일 파일을 복사합니다.
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err = io.Copy(destFile, sourceFile); err != nil {
		return err
	}

	sourceInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, sourceInfo.Mode())
}

// copyDir은 디렉토리를 재귀적으로 복사합니다.
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}
