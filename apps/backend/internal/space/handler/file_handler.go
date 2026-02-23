package handler

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"taeu.kr/cohesion/internal/account"
	"taeu.kr/cohesion/internal/auth"
	"taeu.kr/cohesion/internal/platform/web"
	"taeu.kr/cohesion/internal/space"
)

type uploadConflictPolicy string

const (
	uploadConflictPolicyOverwrite uploadConflictPolicy = "overwrite"
	uploadConflictPolicyRename    uploadConflictPolicy = "rename"
	uploadConflictPolicySkip      uploadConflictPolicy = "skip"
)

const (
	fileConflictCodeDestinationExists       = "destination_exists"
	fileConflictCodeSameDestination         = "same_destination"
	fileConflictCodeDestinationTypeMismatch = "destination_type_mismatch"
	fileConflictCodeQuotaExceeded           = "quota_exceeded"
)

const (
	spaceTrashDirectoryName = ".cohesion_trash"
)

// resolveAbsPath는 Space 경로와 상대 경로를 합쳐 절대 경로를 반환하고 트래버셜 방지 검증을 수행합니다.
func resolveAbsPath(spacePath, relativePath string) (string, error) {
	abs := filepath.Join(spacePath, relativePath)
	if !isPathWithinSpace(abs, spacePath) {
		return "", fmt.Errorf("path traversal detected")
	}
	return abs, nil
}

func resolveUploadConflictPolicy(raw string, overwriteLegacy bool) (uploadConflictPolicy, bool, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		if overwriteLegacy {
			return uploadConflictPolicyOverwrite, true, nil
		}
		return "", false, nil
	}

	policy := uploadConflictPolicy(normalized)
	switch policy {
	case uploadConflictPolicyOverwrite, uploadConflictPolicyRename, uploadConflictPolicySkip:
		return policy, true, nil
	default:
		return "", false, fmt.Errorf("invalid conflict policy: %s", raw)
	}
}

func resolveUploadRenamePath(destPath string) (string, string, error) {
	dir := filepath.Dir(destPath)
	base := filepath.Base(destPath)
	nameWithoutExt := base
	ext := ""
	if strings.HasPrefix(base, ".") && strings.Count(base, ".") == 1 {
		nameWithoutExt = base
	} else {
		ext = filepath.Ext(base)
		nameWithoutExt = strings.TrimSuffix(base, ext)
	}

	for i := 1; ; i++ {
		candidateName := fmt.Sprintf("%s (%d)%s", nameWithoutExt, i, ext)
		candidatePath := filepath.Join(dir, candidateName)
		if _, err := os.Stat(candidatePath); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return "", "", err
		}
		return candidatePath, candidateName, nil
	}
}

func resolveUniqueSiblingPath(destPath, purpose string) (string, error) {
	dir := filepath.Dir(destPath)
	base := filepath.Base(destPath)

	for i := 1; ; i++ {
		candidatePath := filepath.Join(dir, fmt.Sprintf(".%s.cohesion-%s-%d", base, purpose, i))
		if _, err := os.Stat(candidatePath); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return "", err
		}
		return candidatePath, nil
	}
}

func moveWithDestinationSwap(srcPath, destPath string) error {
	backupPath, err := resolveUniqueSiblingPath(destPath, "bak")
	if err != nil {
		return fmt.Errorf("failed to allocate backup path: %w", err)
	}

	if err := os.Rename(destPath, backupPath); err != nil {
		return fmt.Errorf("failed to backup destination: %w", err)
	}

	if err := os.Rename(srcPath, destPath); err != nil {
		if restoreErr := os.Rename(backupPath, destPath); restoreErr != nil {
			return fmt.Errorf("failed to move source: %v; additionally failed to restore destination backup %q: %v", err, backupPath, restoreErr)
		}
		return err
	}

	if removeErr := os.RemoveAll(backupPath); removeErr != nil {
		log.Printf("[WARN] move overwrite backup cleanup failed: %s: %v", backupPath, removeErr)
	}

	return nil
}

func copyWithDestinationSwap(srcPath, destPath string, isDir bool) error {
	stagedPath, err := resolveUniqueSiblingPath(destPath, "stage")
	if err != nil {
		return fmt.Errorf("failed to allocate staging path: %w", err)
	}

	var copyErr error
	if isDir {
		copyErr = copyDir(srcPath, stagedPath)
	} else {
		copyErr = copyFile(srcPath, stagedPath)
	}
	if copyErr != nil {
		os.RemoveAll(stagedPath) //nolint:errcheck
		return copyErr
	}

	backupPath, err := resolveUniqueSiblingPath(destPath, "bak")
	if err != nil {
		os.RemoveAll(stagedPath) //nolint:errcheck
		return fmt.Errorf("failed to allocate backup path: %w", err)
	}

	if err := os.Rename(destPath, backupPath); err != nil {
		os.RemoveAll(stagedPath) //nolint:errcheck
		return fmt.Errorf("failed to backup destination: %w", err)
	}

	if err := os.Rename(stagedPath, destPath); err != nil {
		if restoreErr := os.Rename(backupPath, destPath); restoreErr != nil {
			return fmt.Errorf("failed to finalize copied data: %v; additionally failed to restore destination backup %q: %v", err, backupPath, restoreErr)
		}
		if cleanupErr := os.RemoveAll(stagedPath); cleanupErr != nil {
			log.Printf("[WARN] copy overwrite staging cleanup failed: %s: %v", stagedPath, cleanupErr)
		}
		return err
	}

	if removeErr := os.RemoveAll(backupPath); removeErr != nil {
		log.Printf("[WARN] copy overwrite backup cleanup failed: %s: %v", backupPath, removeErr)
	}

	return nil
}

func normalizeRelativePath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	cleaned := filepath.ToSlash(filepath.Clean(trimmed))
	cleaned = strings.TrimPrefix(cleaned, "./")
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "." {
		return ""
	}
	return cleaned
}

func containsTrashDirectorySegment(relativePath string) bool {
	normalized := normalizeRelativePath(relativePath)
	if normalized == "" {
		return false
	}

	segments := strings.Split(normalized, "/")
	for _, segment := range segments {
		if segment == spaceTrashDirectoryName {
			return true
		}
	}
	return false
}

func ensurePathOutsideTrash(relativePath string) error {
	if containsTrashDirectorySegment(relativePath) {
		return fmt.Errorf("access denied: trash path is reserved")
	}
	return nil
}

func ensureNameIsNotTrashDirectory(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil
	}
	if filepath.Base(trimmed) == spaceTrashDirectoryName {
		return fmt.Errorf("trash directory name is reserved")
	}
	return nil
}

func (h *Handler) ensureSpaceQuotaForWrite(ctx context.Context, spaceID int64, deltaBytes int64) *web.Error {
	if h.quotaService == nil {
		return nil
	}

	if err := h.quotaService.EnsureCanWrite(ctx, spaceID, deltaBytes); err != nil {
		var quotaErr *space.QuotaExceededError
		if errors.As(err, &quotaErr) {
			return &web.Error{Code: http.StatusInsufficientStorage, Message: "Space quota exceeded", Err: err}
		}
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to evaluate space quota", Err: err}
	}
	return nil
}

func quotaFailureReason(err error) string {
	if err == nil {
		return "Space quota exceeded"
	}
	var quotaErr *space.QuotaExceededError
	if errors.As(err, &quotaErr) {
		return fmt.Sprintf("Space quota exceeded (used=%d, quota=%d)", quotaErr.UsedBytes, quotaErr.QuotaBytes)
	}
	return "Space quota exceeded"
}

func trashStorageRelativePath(spacePath string, storageAbsPath string) (string, error) {
	relative, err := filepath.Rel(spacePath, storageAbsPath)
	if err != nil {
		return "", err
	}
	relative = filepath.ToSlash(relative)
	if strings.HasPrefix(relative, "../") || relative == ".." {
		return "", fmt.Errorf("trash path is outside of space")
	}
	return relative, nil
}

func generateTrashStoragePath(spacePath string, baseName string) (string, string, error) {
	trashDir := filepath.Join(spacePath, spaceTrashDirectoryName)
	if !isPathWithinSpace(trashDir, spacePath) {
		return "", "", fmt.Errorf("trash directory is outside of space")
	}
	if err := os.MkdirAll(trashDir, 0o755); err != nil {
		return "", "", err
	}

	safeBaseName := strings.TrimSpace(filepath.Base(baseName))
	if safeBaseName == "" || safeBaseName == "." || safeBaseName == "/" {
		safeBaseName = "item"
	}

	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", "", err
	}
	suffix := hex.EncodeToString(randomBytes)
	candidateName := fmt.Sprintf("%d-%s-%s", time.Now().UnixNano(), suffix, safeBaseName)
	storageAbsPath := filepath.Join(trashDir, candidateName)
	storageRelPath, err := trashStorageRelativePath(spacePath, storageAbsPath)
	if err != nil {
		return "", "", err
	}
	return storageRelPath, storageAbsPath, nil
}

func claimsUsernameFromRequest(r *http.Request) (string, *web.Error) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		return "", &web.Error{Code: http.StatusUnauthorized, Message: "Unauthorized"}
	}
	return claims.Username, nil
}

func (h *Handler) ensureTrashService() *web.Error {
	if h.trashService == nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Trash service is unavailable"}
	}
	return nil
}

func (h *Handler) softDeletePath(r *http.Request, spaceData *space.Space, relPath string) (*space.TrashItem, error) {
	if h.trashService == nil {
		return nil, fmt.Errorf("trash service is unavailable")
	}

	username, webErr := claimsUsernameFromRequest(r)
	if webErr != nil {
		return nil, errors.New(webErr.Message)
	}

	normalizedPath := normalizeRelativePath(relPath)
	if normalizedPath == "" {
		return nil, fmt.Errorf("path is required")
	}
	if err := ensurePathOutsideTrash(normalizedPath); err != nil {
		return nil, err
	}

	absPath, err := resolveAbsPath(spaceData.SpacePath, normalizedPath)
	if err != nil {
		return nil, fmt.Errorf("access denied: invalid path")
	}

	fileInfo, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file or directory not found")
		}
		return nil, fmt.Errorf("failed to access file: %w", err)
	}

	storageRelativePath, storageAbsPath, err := generateTrashStoragePath(spaceData.SpacePath, fileInfo.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to allocate trash storage path: %w", err)
	}

	if err := os.Rename(absPath, storageAbsPath); err != nil {
		return nil, fmt.Errorf("failed to move file into trash: %w", err)
	}

	itemSize := fileInfo.Size()
	if fileInfo.IsDir() {
		itemSize = 0
	}

	item, err := h.trashService.CreateTrashItem(r.Context(), &space.CreateTrashItemRequest{
		SpaceID:      spaceData.ID,
		OriginalPath: normalizedPath,
		StoragePath:  storageRelativePath,
		ItemName:     fileInfo.Name(),
		IsDir:        fileInfo.IsDir(),
		ItemSize:     itemSize,
		DeletedBy:    username,
	})
	if err != nil {
		if rollbackErr := os.Rename(storageAbsPath, absPath); rollbackErr != nil {
			return nil, fmt.Errorf("failed to create trash metadata: %v; rollback failed: %v", err, rollbackErr)
		}
		return nil, fmt.Errorf("failed to create trash metadata: %w", err)
	}

	return item, nil
}

// handleSpaceFiles는 /api/spaces/{id}/files/* 요청을 액션별로 분기합니다.
func (h *Handler) handleSpaceFiles(w http.ResponseWriter, r *http.Request, spaceID int64, action string) *web.Error {
	switch action {
	case "download":
		return h.handleFileDownload(w, r, spaceID)
	case "download-ticket":
		return h.handleFileDownloadTicket(w, r, spaceID)
	case "rename":
		return h.handleFileRename(w, r, spaceID)
	case "delete":
		return h.handleFileDelete(w, r, spaceID)
	case "delete-multiple":
		return h.handleFileDeleteMultiple(w, r, spaceID)
	case "trash":
		return h.handleTrashList(w, r, spaceID)
	case "trash-restore":
		return h.handleTrashRestore(w, r, spaceID)
	case "trash-delete":
		return h.handleTrashDelete(w, r, spaceID)
	case "trash-empty":
		return h.handleTrashEmpty(w, r, spaceID)
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
	case "download-multiple-ticket":
		return h.handleFileDownloadMultipleTicket(w, r, spaceID)
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

func (h *Handler) ensureSpacePermission(r *http.Request, spaceID int64, required account.Permission) *web.Error {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		return &web.Error{Code: http.StatusUnauthorized, Message: "Unauthorized"}
	}

	allowed, err := h.accountService.CanAccessSpaceByID(r.Context(), claims.Username, spaceID, required)
	if err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to evaluate space access", Err: err}
	}
	if !allowed {
		return &web.Error{Code: http.StatusForbidden, Message: "Access denied: insufficient destination space permission"}
	}

	return nil
}

// handleFileDownload: GET /api/spaces/{id}/files/download?path={relativePath}
func (h *Handler) handleFileDownload(w http.ResponseWriter, r *http.Request, spaceID int64) *web.Error {
	spaceData, webErr := h.getSpace(r, spaceID)
	if webErr != nil {
		return webErr
	}

	relativePath := r.URL.Query().Get("path")
	if err := ensurePathOutsideTrash(relativePath); err != nil {
		return &web.Error{Code: http.StatusForbidden, Message: "Access denied: invalid path", Err: err}
	}
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

// handleFileDownloadTicket: POST /api/spaces/{id}/files/download-ticket
// body: { path: string }
func (h *Handler) handleFileDownloadTicket(w http.ResponseWriter, r *http.Request, spaceID int64) *web.Error {
	if r.Method != http.MethodPost {
		return &web.Error{Code: http.StatusMethodNotAllowed, Message: "Method not allowed"}
	}

	spaceData, webErr := h.getSpace(r, spaceID)
	if webErr != nil {
		return webErr
	}

	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		return &web.Error{Code: http.StatusUnauthorized, Message: "Unauthorized"}
	}

	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return &web.Error{Code: http.StatusBadRequest, Message: "Invalid request body", Err: err}
	}
	if strings.TrimSpace(req.Path) == "" {
		return &web.Error{Code: http.StatusBadRequest, Message: "path is required"}
	}
	if err := ensurePathOutsideTrash(req.Path); err != nil {
		return &web.Error{Code: http.StatusForbidden, Message: "Access denied: invalid path", Err: err}
	}

	absPath, err := resolveAbsPath(spaceData.SpacePath, req.Path)
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

	downloadFilePath := absPath
	downloadFileName := fileInfo.Name()
	contentType := "application/octet-stream"
	contentSize := fileInfo.Size()
	removeAfterUse := false

	if fileInfo.IsDir() {
		zipFileName := fileInfo.Name() + ".zip"
		zipTempPath, zipSize, zipErr := h.buildZipTempArchive(func(zipWriter *zip.Writer) *web.Error {
			return h.writeFolderToZip(absPath, zipWriter)
		})
		if zipErr != nil {
			return zipErr
		}
		downloadFilePath = zipTempPath
		downloadFileName = zipFileName
		contentType = "application/zip"
		contentSize = zipSize
		removeAfterUse = true
	}

	ticket, err := h.issueDownloadTicket(
		claims.Username,
		downloadFilePath,
		downloadFileName,
		contentType,
		contentSize,
		removeAfterUse,
	)
	if err != nil {
		if removeAfterUse {
			os.Remove(downloadFilePath) //nolint:errcheck
		}
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to issue download ticket", Err: err}
	}

	type downloadTicketResponse struct {
		DownloadURL string `json:"downloadUrl"`
		FileName    string `json:"fileName"`
		ExpiresAt   string `json:"expiresAt"`
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(downloadTicketResponse{
		DownloadURL: fmt.Sprintf("/api/downloads/%s", ticket.Token),
		FileName:    ticket.FileName,
		ExpiresAt:   ticket.ExpiresAt.Format(time.RFC3339),
	})
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
	if strings.ContainsAny(req.NewName, `/\`) {
		return &web.Error{Code: http.StatusBadRequest, Message: "newName must not contain path separators"}
	}
	if err := ensurePathOutsideTrash(req.Path); err != nil {
		return &web.Error{Code: http.StatusForbidden, Message: "Access denied: invalid path", Err: err}
	}
	if err := ensureNameIsNotTrashDirectory(req.NewName); err != nil {
		return &web.Error{Code: http.StatusBadRequest, Message: "newName is reserved", Err: err}
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
	if webErr := h.ensureTrashService(); webErr != nil {
		return webErr
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

	item, err := h.softDeletePath(r, spaceData, req.Path)
	if err != nil {
		message := err.Error()
		switch {
		case strings.Contains(message, "path is required"):
			return &web.Error{Code: http.StatusBadRequest, Message: "path is required", Err: err}
		case strings.Contains(message, "access denied"):
			return &web.Error{Code: http.StatusForbidden, Message: "Access denied: invalid path", Err: err}
		case strings.Contains(message, "file or directory not found"):
			return &web.Error{Code: http.StatusNotFound, Message: "File or directory not found", Err: err}
		case strings.Contains(message, "Unauthorized"):
			return &web.Error{Code: http.StatusUnauthorized, Message: "Unauthorized", Err: err}
		default:
			return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to move item into trash", Err: err}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":      "Moved to trash",
		"trashItemId":  item.ID,
		"originalPath": item.OriginalPath,
	})
	return nil
}

// handleFileDeleteMultiple: POST /api/spaces/{id}/files/delete-multiple
// body: { paths: []string }
func (h *Handler) handleFileDeleteMultiple(w http.ResponseWriter, r *http.Request, spaceID int64) *web.Error {
	if r.Method != http.MethodPost {
		return &web.Error{Code: http.StatusMethodNotAllowed, Message: "Method not allowed"}
	}
	if webErr := h.ensureTrashService(); webErr != nil {
		return webErr
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
		item, err := h.softDeletePath(r, spaceData, relPath)
		if err != nil {
			failed = append(failed, deleteResult{Path: relPath, Reason: err.Error()})
			continue
		}
		succeeded = append(succeeded, item.OriginalPath)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"succeeded": succeeded, "failed": failed})
	return nil
}

// handleTrashList: GET /api/spaces/{id}/files/trash
func (h *Handler) handleTrashList(w http.ResponseWriter, r *http.Request, spaceID int64) *web.Error {
	if r.Method != http.MethodGet {
		return &web.Error{Code: http.StatusMethodNotAllowed, Message: "Method not allowed"}
	}
	if webErr := h.ensureTrashService(); webErr != nil {
		return webErr
	}

	spaceData, webErr := h.getSpace(r, spaceID)
	if webErr != nil {
		return webErr
	}

	items, err := h.trashService.ListTrashItems(r.Context(), spaceID)
	if err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to list trash items", Err: err}
	}

	type trashItemResponse struct {
		ID           int64     `json:"id"`
		OriginalPath string    `json:"originalPath"`
		ItemName     string    `json:"itemName"`
		IsDir        bool      `json:"isDir"`
		ItemSize     int64     `json:"itemSize"`
		DeletedBy    string    `json:"deletedBy"`
		DeletedAt    time.Time `json:"deletedAt"`
	}

	response := make([]trashItemResponse, 0, len(items))
	for _, item := range items {
		absStoragePath, pathErr := resolveAbsPath(spaceData.SpacePath, item.StoragePath)
		if pathErr != nil {
			_ = h.trashService.DeleteTrashItem(r.Context(), item.ID)
			continue
		}

		if _, statErr := os.Stat(absStoragePath); statErr != nil {
			if os.IsNotExist(statErr) {
				_ = h.trashService.DeleteTrashItem(r.Context(), item.ID)
				continue
			}
			return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to inspect trash item", Err: statErr}
		}

		response = append(response, trashItemResponse{
			ID:           item.ID,
			OriginalPath: item.OriginalPath,
			ItemName:     item.ItemName,
			IsDir:        item.IsDir,
			ItemSize:     item.ItemSize,
			DeletedBy:    item.DeletedBy,
			DeletedAt:    item.DeletedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"items": response,
	})
	return nil
}

// handleTrashRestore: POST /api/spaces/{id}/files/trash-restore
// body: { ids: []int64, conflictPolicy?: "overwrite"|"rename"|"skip" }
func (h *Handler) handleTrashRestore(w http.ResponseWriter, r *http.Request, spaceID int64) *web.Error {
	if r.Method != http.MethodPost {
		return &web.Error{Code: http.StatusMethodNotAllowed, Message: "Method not allowed"}
	}
	if webErr := h.ensureTrashService(); webErr != nil {
		return webErr
	}

	spaceData, webErr := h.getSpace(r, spaceID)
	if webErr != nil {
		return webErr
	}

	var req struct {
		IDs            []int64 `json:"ids"`
		ConflictPolicy string  `json:"conflictPolicy,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return &web.Error{Code: http.StatusBadRequest, Message: "Invalid request body", Err: err}
	}
	if len(req.IDs) == 0 {
		return &web.Error{Code: http.StatusBadRequest, Message: "ids array is required and cannot be empty"}
	}

	conflictPolicy, hasConflictPolicy, err := resolveUploadConflictPolicy(req.ConflictPolicy, false)
	if err != nil {
		return &web.Error{Code: http.StatusBadRequest, Message: "Invalid conflict policy", Err: err}
	}

	type restoreSuccess struct {
		ID           int64  `json:"id"`
		OriginalPath string `json:"originalPath"`
	}
	type restoreFailed struct {
		ID           int64  `json:"id"`
		OriginalPath string `json:"originalPath,omitempty"`
		Reason       string `json:"reason"`
		Code         string `json:"code,omitempty"`
	}

	succeeded := make([]restoreSuccess, 0)
	skipped := make([]restoreSuccess, 0)
	failed := make([]restoreFailed, 0)

	for _, id := range req.IDs {
		item, getErr := h.trashService.GetTrashItem(r.Context(), id)
		if getErr != nil || item.SpaceID != spaceID {
			failed = append(failed, restoreFailed{ID: id, Reason: "Trash item not found"})
			continue
		}

		absStoragePath, storagePathErr := resolveAbsPath(spaceData.SpacePath, item.StoragePath)
		if storagePathErr != nil {
			_ = h.trashService.DeleteTrashItem(r.Context(), item.ID)
			failed = append(failed, restoreFailed{
				ID:           item.ID,
				OriginalPath: item.OriginalPath,
				Reason:       "Invalid trash storage path",
			})
			continue
		}

		storageInfo, storageStatErr := os.Stat(absStoragePath)
		if storageStatErr != nil {
			if os.IsNotExist(storageStatErr) {
				_ = h.trashService.DeleteTrashItem(r.Context(), item.ID)
				failed = append(failed, restoreFailed{
					ID:           item.ID,
					OriginalPath: item.OriginalPath,
					Reason:       "Trash item no longer exists",
				})
				continue
			}
			failed = append(failed, restoreFailed{
				ID:           item.ID,
				OriginalPath: item.OriginalPath,
				Reason:       fmt.Sprintf("Failed to access trash item: %v", storageStatErr),
			})
			continue
		}

		destAbsPath, destPathErr := resolveAbsPath(spaceData.SpacePath, item.OriginalPath)
		if destPathErr != nil {
			failed = append(failed, restoreFailed{
				ID:           item.ID,
				OriginalPath: item.OriginalPath,
				Reason:       "Invalid original path",
			})
			continue
		}

		if mkdirErr := os.MkdirAll(filepath.Dir(destAbsPath), 0o755); mkdirErr != nil {
			failed = append(failed, restoreFailed{
				ID:           item.ID,
				OriginalPath: item.OriginalPath,
				Reason:       fmt.Sprintf("Failed to prepare restore directory: %v", mkdirErr),
			})
			continue
		}

		finalDestAbsPath := destAbsPath
		if destInfo, destStatErr := os.Stat(destAbsPath); destStatErr == nil {
			if !hasConflictPolicy {
				failed = append(failed, restoreFailed{
					ID:           item.ID,
					OriginalPath: item.OriginalPath,
					Reason:       "Destination path already exists",
					Code:         fileConflictCodeDestinationExists,
				})
				continue
			}

			switch conflictPolicy {
			case uploadConflictPolicyOverwrite:
				if storageInfo.IsDir() != destInfo.IsDir() {
					failed = append(failed, restoreFailed{
						ID:           item.ID,
						OriginalPath: item.OriginalPath,
						Reason:       "Cannot overwrite destination with different type",
						Code:         fileConflictCodeDestinationTypeMismatch,
					})
					continue
				}
				if overwriteErr := moveWithDestinationSwap(absStoragePath, finalDestAbsPath); overwriteErr != nil {
					failed = append(failed, restoreFailed{
						ID:           item.ID,
						OriginalPath: item.OriginalPath,
						Reason:       fmt.Sprintf("Failed to restore with overwrite: %v", overwriteErr),
					})
					continue
				}
				if deleteErr := h.trashService.DeleteTrashItem(r.Context(), item.ID); deleteErr != nil {
					log.Printf("[WARN] restore metadata cleanup failed (id=%d): %v", item.ID, deleteErr)
				}
				succeeded = append(succeeded, restoreSuccess{ID: item.ID, OriginalPath: item.OriginalPath})
				continue
			case uploadConflictPolicyRename:
				renamedAbsPath, _, renameErr := resolveUploadRenamePath(destAbsPath)
				if renameErr != nil {
					failed = append(failed, restoreFailed{
						ID:           item.ID,
						OriginalPath: item.OriginalPath,
						Reason:       fmt.Sprintf("Failed to resolve restore rename destination: %v", renameErr),
					})
					continue
				}
				finalDestAbsPath = renamedAbsPath
			case uploadConflictPolicySkip:
				skipped = append(skipped, restoreSuccess{ID: item.ID, OriginalPath: item.OriginalPath})
				continue
			}
		} else if !os.IsNotExist(destStatErr) {
			failed = append(failed, restoreFailed{
				ID:           item.ID,
				OriginalPath: item.OriginalPath,
				Reason:       fmt.Sprintf("Failed to access destination path: %v", destStatErr),
			})
			continue
		}

		if moveErr := os.Rename(absStoragePath, finalDestAbsPath); moveErr != nil {
			failed = append(failed, restoreFailed{
				ID:           item.ID,
				OriginalPath: item.OriginalPath,
				Reason:       fmt.Sprintf("Failed to restore item: %v", moveErr),
			})
			continue
		}
		if deleteErr := h.trashService.DeleteTrashItem(r.Context(), item.ID); deleteErr != nil {
			log.Printf("[WARN] restore metadata cleanup failed (id=%d): %v", item.ID, deleteErr)
		}
		succeeded = append(succeeded, restoreSuccess{ID: item.ID, OriginalPath: item.OriginalPath})
	}
	if h.quotaService != nil && len(succeeded) > 0 {
		h.quotaService.Invalidate(spaceID)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"succeeded": succeeded,
		"skipped":   skipped,
		"failed":    failed,
	})
	return nil
}

// handleTrashDelete: POST /api/spaces/{id}/files/trash-delete
// body: { ids: []int64 }
func (h *Handler) handleTrashDelete(w http.ResponseWriter, r *http.Request, spaceID int64) *web.Error {
	if r.Method != http.MethodPost {
		return &web.Error{Code: http.StatusMethodNotAllowed, Message: "Method not allowed"}
	}
	if webErr := h.ensureTrashService(); webErr != nil {
		return webErr
	}

	spaceData, webErr := h.getSpace(r, spaceID)
	if webErr != nil {
		return webErr
	}

	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return &web.Error{Code: http.StatusBadRequest, Message: "Invalid request body", Err: err}
	}
	if len(req.IDs) == 0 {
		return &web.Error{Code: http.StatusBadRequest, Message: "ids array is required and cannot be empty"}
	}

	type deleteSuccess struct {
		ID int64 `json:"id"`
	}
	type deleteFailed struct {
		ID     int64  `json:"id"`
		Reason string `json:"reason"`
	}

	succeeded := make([]deleteSuccess, 0)
	failed := make([]deleteFailed, 0)

	for _, id := range req.IDs {
		item, getErr := h.trashService.GetTrashItem(r.Context(), id)
		if getErr != nil || item.SpaceID != spaceID {
			failed = append(failed, deleteFailed{ID: id, Reason: "Trash item not found"})
			continue
		}

		absStoragePath, storagePathErr := resolveAbsPath(spaceData.SpacePath, item.StoragePath)
		if storagePathErr != nil {
			failed = append(failed, deleteFailed{ID: item.ID, Reason: "Invalid trash storage path"})
			continue
		}

		if removeErr := os.RemoveAll(absStoragePath); removeErr != nil && !os.IsNotExist(removeErr) {
			failed = append(failed, deleteFailed{ID: item.ID, Reason: fmt.Sprintf("Failed to delete trash file: %v", removeErr)})
			continue
		}
		if deleteErr := h.trashService.DeleteTrashItem(r.Context(), item.ID); deleteErr != nil {
			failed = append(failed, deleteFailed{ID: item.ID, Reason: fmt.Sprintf("Failed to delete trash metadata: %v", deleteErr)})
			continue
		}
		succeeded = append(succeeded, deleteSuccess{ID: item.ID})
	}
	if h.quotaService != nil && len(succeeded) > 0 {
		h.quotaService.Invalidate(spaceID)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"succeeded": succeeded,
		"failed":    failed,
	})
	return nil
}

// handleTrashEmpty: POST /api/spaces/{id}/files/trash-empty
func (h *Handler) handleTrashEmpty(w http.ResponseWriter, r *http.Request, spaceID int64) *web.Error {
	if r.Method != http.MethodPost {
		return &web.Error{Code: http.StatusMethodNotAllowed, Message: "Method not allowed"}
	}
	if webErr := h.ensureTrashService(); webErr != nil {
		return webErr
	}

	spaceData, webErr := h.getSpace(r, spaceID)
	if webErr != nil {
		return webErr
	}

	items, err := h.trashService.ListTrashItems(r.Context(), spaceID)
	if err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to list trash items", Err: err}
	}

	type emptyFailed struct {
		ID     int64  `json:"id"`
		Reason string `json:"reason"`
	}

	removed := 0
	failed := make([]emptyFailed, 0)
	for _, item := range items {
		absStoragePath, storagePathErr := resolveAbsPath(spaceData.SpacePath, item.StoragePath)
		if storagePathErr != nil {
			failed = append(failed, emptyFailed{ID: item.ID, Reason: "Invalid trash storage path"})
			continue
		}

		if removeErr := os.RemoveAll(absStoragePath); removeErr != nil && !os.IsNotExist(removeErr) {
			failed = append(failed, emptyFailed{ID: item.ID, Reason: fmt.Sprintf("Failed to delete trash file: %v", removeErr)})
			continue
		}
		if deleteErr := h.trashService.DeleteTrashItem(r.Context(), item.ID); deleteErr != nil {
			failed = append(failed, emptyFailed{ID: item.ID, Reason: fmt.Sprintf("Failed to delete trash metadata: %v", deleteErr)})
			continue
		}
		removed++
	}
	if h.quotaService != nil && removed > 0 {
		h.quotaService.Invalidate(spaceID)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"removed": removed,
		"failed":  failed,
	})
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
	if err := ensurePathOutsideTrash(req.ParentPath); err != nil {
		return &web.Error{Code: http.StatusForbidden, Message: "Access denied: invalid path", Err: err}
	}
	if err := ensureNameIsNotTrashDirectory(req.FolderName); err != nil {
		return &web.Error{Code: http.StatusBadRequest, Message: "folderName is reserved", Err: err}
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

	if webErr := h.ensureSpaceQuotaForWrite(r.Context(), spaceID, 0); webErr != nil {
		return webErr
	}

	if err := os.Mkdir(folderPath, 0755); err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to create folder", Err: err}
	}
	if h.quotaService != nil {
		h.quotaService.Invalidate(spaceID)
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
// multipart form: file, path (상대 경로), conflictPolicy (optional: overwrite|rename|skip), overwrite (legacy optional)
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
	if err := ensurePathOutsideTrash(targetRelPath); err != nil {
		return &web.Error{Code: http.StatusForbidden, Message: "Access denied: invalid path", Err: err}
	}
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
	resultFileName := header.Filename
	projectedDelta := header.Size
	overwriteLegacy := r.FormValue("overwrite") == "true"
	conflictPolicy, hasConflictPolicy, err := resolveUploadConflictPolicy(r.FormValue("conflictPolicy"), overwriteLegacy)
	if err != nil {
		return &web.Error{Code: http.StatusBadRequest, Message: "Invalid conflict policy", Err: err}
	}

	if existingInfo, err := os.Stat(destPath); err == nil {
		if !hasConflictPolicy {
			return &web.Error{Code: http.StatusConflict, Message: "File already exists"}
		}
		switch conflictPolicy {
		case uploadConflictPolicyOverwrite:
			if existingInfo.IsDir() {
				return &web.Error{Code: http.StatusConflict, Message: "Directory already exists"}
			}
			projectedDelta = header.Size - existingInfo.Size()
		case uploadConflictPolicyRename:
			renamedPath, renamedFileName, resolveErr := resolveUploadRenamePath(destPath)
			if resolveErr != nil {
				return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to resolve rename destination", Err: resolveErr}
			}
			destPath = renamedPath
			resultFileName = renamedFileName
		case uploadConflictPolicySkip:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{
				"message":  "Skipped existing file",
				"filename": header.Filename,
				"status":   "skipped",
			})
			return nil
		}
	} else if !os.IsNotExist(err) {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to inspect destination path", Err: err}
	}

	if webErr := h.ensureSpaceQuotaForWrite(r.Context(), spaceID, projectedDelta); webErr != nil {
		return webErr
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
	json.NewEncoder(w).Encode(map[string]string{
		"message":  "Successfully uploaded",
		"filename": resultFileName,
		"status":   "uploaded",
	})
	if h.quotaService != nil {
		h.quotaService.Invalidate(spaceID)
	}
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
		Sources        []string `json:"sources"`
		ConflictPolicy string   `json:"conflictPolicy,omitempty"`
		Destination    struct {
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
	conflictPolicy, hasConflictPolicy, err := resolveUploadConflictPolicy(req.ConflictPolicy, false)
	if err != nil {
		return &web.Error{Code: http.StatusBadRequest, Message: "Invalid conflict policy", Err: err}
	}

	// 대상 Space 조회 (cross-Space 지원)
	dstSpaceID := req.Destination.SpaceID
	if dstSpaceID == 0 {
		dstSpaceID = spaceID
	}
	if webErr := h.ensureSpacePermission(r, dstSpaceID, account.PermissionWrite); webErr != nil {
		return webErr
	}
	if err := ensurePathOutsideTrash(req.Destination.Path); err != nil {
		return &web.Error{Code: http.StatusForbidden, Message: "Access denied: invalid destination path", Err: err}
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
		Code   string `json:"code,omitempty"`
	}
	succeeded := []string{}
	skipped := []string{}
	failed := []moveResult{}
	quotaInvalidationTargets := map[int64]struct{}{}

	for _, relSrc := range req.Sources {
		if err := ensurePathOutsideTrash(relSrc); err != nil {
			failed = append(failed, moveResult{Path: relSrc, Reason: "Access denied: invalid source path"})
			continue
		}
		absSrc, err := resolveAbsPath(srcSpace.SpacePath, relSrc)
		if err != nil {
			failed = append(failed, moveResult{Path: relSrc, Reason: "Access denied: invalid source path"})
			continue
		}

		srcInfo, err := os.Stat(absSrc)
		if err != nil {
			if os.IsNotExist(err) {
				failed = append(failed, moveResult{Path: relSrc, Reason: "Source not found"})
			} else {
				failed = append(failed, moveResult{Path: relSrc, Reason: fmt.Sprintf("Failed to access source: %v", err)})
			}
			continue
		}
		sourceSize, sizeErr := h.quotaService.CalculatePathSize(r.Context(), absSrc)
		if sizeErr != nil {
			failed = append(failed, moveResult{Path: relSrc, Reason: fmt.Sprintf("Failed to evaluate source size: %v", sizeErr)})
			continue
		}

		cleanSrc := filepath.Clean(absSrc)
		cleanDst := filepath.Clean(absDestDir)
		if strings.HasPrefix(cleanDst, cleanSrc+string(filepath.Separator)) {
			failed = append(failed, moveResult{Path: relSrc, Reason: "Cannot move to a subdirectory of itself"})
			continue
		}

		destPath := filepath.Join(absDestDir, filepath.Base(absSrc))
		cleanDestPath := filepath.Clean(destPath)
		if cleanSrc == cleanDestPath {
			failed = append(failed, moveResult{
				Path:   relSrc,
				Reason: "Cannot move to the same destination",
				Code:   fileConflictCodeSameDestination,
			})
			continue
		}

		projectedDelta := sourceSize
		if dstSpaceID == spaceID {
			projectedDelta = 0
		}

		if destInfo, statErr := os.Stat(destPath); statErr == nil {
			if !hasConflictPolicy {
				failed = append(failed, moveResult{
					Path:   relSrc,
					Reason: "Destination path already exists",
					Code:   fileConflictCodeDestinationExists,
				})
				continue
			}

			switch conflictPolicy {
			case uploadConflictPolicyOverwrite:
				if srcInfo.IsDir() != destInfo.IsDir() {
					failed = append(failed, moveResult{
						Path:   relSrc,
						Reason: "Cannot overwrite destination with different type",
						Code:   fileConflictCodeDestinationTypeMismatch,
					})
					continue
				}
				existingSize, existingSizeErr := h.quotaService.CalculatePathSize(r.Context(), destPath)
				if existingSizeErr != nil {
					failed = append(failed, moveResult{Path: relSrc, Reason: fmt.Sprintf("Failed to evaluate destination size: %v", existingSizeErr)})
					continue
				}
				projectedDelta -= existingSize
				if webErr := h.ensureSpaceQuotaForWrite(r.Context(), dstSpaceID, projectedDelta); webErr != nil {
					failed = append(failed, moveResult{
						Path:   relSrc,
						Reason: quotaFailureReason(webErr.Err),
						Code:   fileConflictCodeQuotaExceeded,
					})
					continue
				}
				if overwriteErr := moveWithDestinationSwap(absSrc, destPath); overwriteErr != nil {
					failed = append(failed, moveResult{Path: relSrc, Reason: fmt.Sprintf("Failed to overwrite destination: %v", overwriteErr)})
					continue
				}
				succeeded = append(succeeded, relSrc)
				quotaInvalidationTargets[dstSpaceID] = struct{}{}
				quotaInvalidationTargets[spaceID] = struct{}{}
				continue
			case uploadConflictPolicyRename:
				renamedPath, _, renameErr := resolveUploadRenamePath(destPath)
				if renameErr != nil {
					failed = append(failed, moveResult{Path: relSrc, Reason: fmt.Sprintf("Failed to resolve rename destination: %v", renameErr)})
					continue
				}
				destPath = renamedPath
			case uploadConflictPolicySkip:
				skipped = append(skipped, relSrc)
				continue
			}
		} else if !os.IsNotExist(statErr) {
			failed = append(failed, moveResult{Path: relSrc, Reason: fmt.Sprintf("Failed to access destination: %v", statErr)})
			continue
		}

		if webErr := h.ensureSpaceQuotaForWrite(r.Context(), dstSpaceID, projectedDelta); webErr != nil {
			failed = append(failed, moveResult{
				Path:   relSrc,
				Reason: quotaFailureReason(webErr.Err),
				Code:   fileConflictCodeQuotaExceeded,
			})
			continue
		}

		if err := os.Rename(absSrc, destPath); err != nil {
			failed = append(failed, moveResult{Path: relSrc, Reason: fmt.Sprintf("Failed to move: %v", err)})
		} else {
			succeeded = append(succeeded, relSrc)
			quotaInvalidationTargets[dstSpaceID] = struct{}{}
			quotaInvalidationTargets[spaceID] = struct{}{}
		}
	}
	if h.quotaService != nil && len(quotaInvalidationTargets) > 0 {
		spaceIDs := make([]int64, 0, len(quotaInvalidationTargets))
		for targetID := range quotaInvalidationTargets {
			spaceIDs = append(spaceIDs, targetID)
		}
		h.quotaService.InvalidateMany(spaceIDs...)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"succeeded": succeeded,
		"failed":    failed,
		"skipped":   skipped,
	})
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
		Sources        []string `json:"sources"`
		ConflictPolicy string   `json:"conflictPolicy,omitempty"`
		Destination    struct {
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
	conflictPolicy, hasConflictPolicy, err := resolveUploadConflictPolicy(req.ConflictPolicy, false)
	if err != nil {
		return &web.Error{Code: http.StatusBadRequest, Message: "Invalid conflict policy", Err: err}
	}

	dstSpaceID := req.Destination.SpaceID
	if dstSpaceID == 0 {
		dstSpaceID = spaceID
	}
	if webErr := h.ensureSpacePermission(r, dstSpaceID, account.PermissionWrite); webErr != nil {
		return webErr
	}
	if err := ensurePathOutsideTrash(req.Destination.Path); err != nil {
		return &web.Error{Code: http.StatusForbidden, Message: "Access denied: invalid destination path", Err: err}
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
		Code   string `json:"code,omitempty"`
	}
	succeeded := []string{}
	skipped := []string{}
	failed := []copyResult{}
	quotaInvalidationTargets := map[int64]struct{}{}

	for _, relSrc := range req.Sources {
		if err := ensurePathOutsideTrash(relSrc); err != nil {
			failed = append(failed, copyResult{Path: relSrc, Reason: "Access denied: invalid source path"})
			continue
		}
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
		sourceSize, sizeErr := h.quotaService.CalculatePathSize(r.Context(), absSrc)
		if sizeErr != nil {
			failed = append(failed, copyResult{Path: relSrc, Reason: fmt.Sprintf("Failed to evaluate source size: %v", sizeErr)})
			continue
		}

		cleanSrc := filepath.Clean(absSrc)
		cleanDst := filepath.Clean(absDestDir)
		if strings.HasPrefix(cleanDst, cleanSrc+string(filepath.Separator)) {
			failed = append(failed, copyResult{Path: relSrc, Reason: "Cannot copy to a subdirectory of itself"})
			continue
		}

		destPath := filepath.Join(absDestDir, filepath.Base(absSrc))
		cleanDestPath := filepath.Clean(destPath)
		if cleanSrc == cleanDestPath {
			failed = append(failed, copyResult{
				Path:   relSrc,
				Reason: "Cannot copy to the same destination",
				Code:   fileConflictCodeSameDestination,
			})
			continue
		}

		projectedDelta := sourceSize
		if destInfo, statErr := os.Stat(destPath); statErr == nil {
			if !hasConflictPolicy {
				failed = append(failed, copyResult{
					Path:   relSrc,
					Reason: "Destination path already exists",
					Code:   fileConflictCodeDestinationExists,
				})
				continue
			}

			switch conflictPolicy {
			case uploadConflictPolicyOverwrite:
				if sourceInfo.IsDir() != destInfo.IsDir() {
					failed = append(failed, copyResult{
						Path:   relSrc,
						Reason: "Cannot overwrite destination with different type",
						Code:   fileConflictCodeDestinationTypeMismatch,
					})
					continue
				}
				existingSize, existingSizeErr := h.quotaService.CalculatePathSize(r.Context(), destPath)
				if existingSizeErr != nil {
					failed = append(failed, copyResult{Path: relSrc, Reason: fmt.Sprintf("Failed to evaluate destination size: %v", existingSizeErr)})
					continue
				}
				projectedDelta -= existingSize
				if webErr := h.ensureSpaceQuotaForWrite(r.Context(), dstSpaceID, projectedDelta); webErr != nil {
					failed = append(failed, copyResult{
						Path:   relSrc,
						Reason: quotaFailureReason(webErr.Err),
						Code:   fileConflictCodeQuotaExceeded,
					})
					continue
				}
				if overwriteErr := copyWithDestinationSwap(absSrc, destPath, sourceInfo.IsDir()); overwriteErr != nil {
					failed = append(failed, copyResult{Path: relSrc, Reason: fmt.Sprintf("Failed to overwrite destination: %v", overwriteErr)})
					continue
				}
				succeeded = append(succeeded, relSrc)
				quotaInvalidationTargets[dstSpaceID] = struct{}{}
				continue
			case uploadConflictPolicyRename:
				renamedPath, _, renameErr := resolveUploadRenamePath(destPath)
				if renameErr != nil {
					failed = append(failed, copyResult{Path: relSrc, Reason: fmt.Sprintf("Failed to resolve rename destination: %v", renameErr)})
					continue
				}
				destPath = renamedPath
			case uploadConflictPolicySkip:
				skipped = append(skipped, relSrc)
				continue
			}
		} else if !os.IsNotExist(statErr) {
			failed = append(failed, copyResult{Path: relSrc, Reason: fmt.Sprintf("Failed to access destination: %v", statErr)})
			continue
		}

		if webErr := h.ensureSpaceQuotaForWrite(r.Context(), dstSpaceID, projectedDelta); webErr != nil {
			failed = append(failed, copyResult{
				Path:   relSrc,
				Reason: quotaFailureReason(webErr.Err),
				Code:   fileConflictCodeQuotaExceeded,
			})
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
			quotaInvalidationTargets[dstSpaceID] = struct{}{}
		}
	}
	if h.quotaService != nil && len(quotaInvalidationTargets) > 0 {
		spaceIDs := make([]int64, 0, len(quotaInvalidationTargets))
		for targetID := range quotaInvalidationTargets {
			spaceIDs = append(spaceIDs, targetID)
		}
		h.quotaService.InvalidateMany(spaceIDs...)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"succeeded": succeeded,
		"failed":    failed,
		"skipped":   skipped,
	})
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
		if err := ensurePathOutsideTrash(relPath); err != nil {
			return &web.Error{Code: http.StatusForbidden, Message: fmt.Sprintf("Access denied: invalid path %s", relPath), Err: err}
		}
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

// handleFileDownloadMultipleTicket: POST /api/spaces/{id}/files/download-multiple-ticket
// body: { paths: []string }
func (h *Handler) handleFileDownloadMultipleTicket(w http.ResponseWriter, r *http.Request, spaceID int64) *web.Error {
	if r.Method != http.MethodPost {
		return &web.Error{Code: http.StatusMethodNotAllowed, Message: "Method not allowed"}
	}

	spaceData, webErr := h.getSpace(r, spaceID)
	if webErr != nil {
		return webErr
	}

	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		return &web.Error{Code: http.StatusUnauthorized, Message: "Unauthorized"}
	}

	var req struct {
		Paths []string `json:"paths"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return &web.Error{Code: http.StatusBadRequest, Message: "Invalid request body", Err: err}
	}
	if len(req.Paths) < 2 {
		return &web.Error{Code: http.StatusBadRequest, Message: "at least 2 paths are required"}
	}

	absPaths := make([]string, 0, len(req.Paths))
	for _, relPath := range req.Paths {
		if err := ensurePathOutsideTrash(relPath); err != nil {
			return &web.Error{Code: http.StatusForbidden, Message: fmt.Sprintf("Access denied: invalid path %s", relPath), Err: err}
		}
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

	zipFileName := fmt.Sprintf("download-%d.zip", os.Getpid())
	zipTempPath, zipSize, zipErr := h.buildZipTempArchive(func(zipWriter *zip.Writer) *web.Error {
		for i, absPath := range absPaths {
			if err := addToZip(zipWriter, absPath, filepath.Base(req.Paths[i])); err != nil {
				log.Printf("Failed to add %s to ZIP: %v", absPath, err)
			}
		}
		return nil
	})
	if zipErr != nil {
		return zipErr
	}

	ticket, err := h.issueDownloadTicket(claims.Username, zipTempPath, zipFileName, "application/zip", zipSize, true)
	if err != nil {
		os.Remove(zipTempPath) //nolint:errcheck
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to issue download ticket", Err: err}
	}

	type downloadTicketResponse struct {
		DownloadURL string `json:"downloadUrl"`
		FileName    string `json:"fileName"`
		ExpiresAt   string `json:"expiresAt"`
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(downloadTicketResponse{
		DownloadURL: fmt.Sprintf("/api/downloads/%s", ticket.Token),
		FileName:    ticket.FileName,
		ExpiresAt:   ticket.ExpiresAt.Format(time.RFC3339),
	})
	return nil
}

// downloadFolderAsZip은 폴더를 zip으로 압축하여 스트리밍합니다.
func (h *Handler) downloadFolderAsZip(w http.ResponseWriter, folderPath string, folderName string) *web.Error {
	zipFileName := folderName + ".zip"
	return h.streamZipDownload(w, zipFileName, func(zipWriter *zip.Writer) *web.Error {
		return h.writeFolderToZip(folderPath, zipWriter)
	})
}

func (h *Handler) writeFolderToZip(folderPath string, zipWriter *zip.Writer) *web.Error {
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
}

func (h *Handler) streamZipDownload(
	w http.ResponseWriter,
	zipFileName string,
	writeZip func(zipWriter *zip.Writer) *web.Error,
) *web.Error {
	zipTempPath, zipSize, zipErr := h.buildZipTempArchive(writeZip)
	if zipErr != nil {
		return zipErr
	}
	defer os.Remove(zipTempPath) //nolint:errcheck

	zipFile, err := os.Open(zipTempPath)
	if err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to open zip archive", Err: err}
	}
	defer zipFile.Close()

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, zipFileName))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", zipSize))

	if _, err := io.Copy(w, zipFile); err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to send zip archive", Err: err}
	}
	return nil
}

func (h *Handler) buildZipTempArchive(writeZip func(zipWriter *zip.Writer) *web.Error) (string, int64, *web.Error) {
	tempFile, err := os.CreateTemp("", "cohesion-download-*.zip")
	if err != nil {
		return "", 0, &web.Error{Code: http.StatusInternalServerError, Message: "Failed to prepare zip archive", Err: err}
	}
	tempFilePath := tempFile.Name()
	cleanup := true
	defer func() {
		tempFile.Close() //nolint:errcheck
		if cleanup {
			os.Remove(tempFilePath) //nolint:errcheck
		}
	}()

	zipWriter := zip.NewWriter(tempFile)
	if webErr := writeZip(zipWriter); webErr != nil {
		zipWriter.Close() //nolint:errcheck
		return "", 0, webErr
	}
	if err := zipWriter.Close(); err != nil {
		return "", 0, &web.Error{Code: http.StatusInternalServerError, Message: "Failed to finalize zip archive", Err: err}
	}

	zipInfo, err := tempFile.Stat()
	if err != nil {
		return "", 0, &web.Error{Code: http.StatusInternalServerError, Message: "Failed to inspect zip archive", Err: err}
	}
	cleanup = false
	return tempFilePath, zipInfo.Size(), nil
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
