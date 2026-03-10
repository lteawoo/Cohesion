package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"taeu.kr/cohesion/internal/account"
	"taeu.kr/cohesion/internal/audit"
	"taeu.kr/cohesion/internal/auth"
	"taeu.kr/cohesion/internal/browse"
	"taeu.kr/cohesion/internal/platform/logging"
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
		logging.Event(log.Warn(), logging.ComponentStorage, "warn.storage.cleanup_failed").
			Str("operation", "move-overwrite-backup-cleanup").
			Str("path", backupPath).
			Err(removeErr).
			Msg("cleanup failed")
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
			logging.Event(log.Warn(), logging.ComponentStorage, "warn.storage.cleanup_failed").
				Str("operation", "copy-overwrite-stage-cleanup").
				Str("path", stagedPath).
				Err(cleanupErr).
				Msg("cleanup failed")
		}
		return err
	}

	if removeErr := os.RemoveAll(backupPath); removeErr != nil {
		logging.Event(log.Warn(), logging.ComponentStorage, "warn.storage.cleanup_failed").
			Str("operation", "copy-overwrite-backup-cleanup").
			Str("path", backupPath).
			Err(removeErr).
			Msg("cleanup failed")
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

func storageAccessWebError(err error, notFoundMessage string, fallbackMessage string) *web.Error {
	if err == nil {
		return nil
	}
	if os.IsNotExist(err) && strings.TrimSpace(notFoundMessage) != "" {
		return &web.Error{Code: http.StatusNotFound, Message: notFoundMessage, Err: err}
	}
	if browse.IsPermissionError(err) {
		return &web.Error{Code: http.StatusForbidden, Message: "Permission denied", Err: err}
	}
	return &web.Error{Code: http.StatusInternalServerError, Message: fallbackMessage, Err: err}
}

func storageOperationWebError(err error, fallbackMessage string) *web.Error {
	if err == nil {
		return nil
	}
	if browse.IsPermissionError(err) {
		return &web.Error{Code: http.StatusForbidden, Message: "Permission denied", Err: err}
	}
	return &web.Error{Code: http.StatusInternalServerError, Message: fallbackMessage, Err: err}
}

func safeFilesystemReason(base string, err error) string {
	trimmedBase := strings.TrimSpace(base)
	if err == nil {
		return trimmedBase
	}
	switch {
	case browse.IsPermissionError(err):
		return fmt.Sprintf("%s: permission denied", trimmedBase)
	case os.IsNotExist(err):
		return fmt.Sprintf("%s: path not found", trimmedBase)
	default:
		return trimmedBase
	}
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

func (h *Handler) invalidateQuotaForSpaces(spaceIDs ...int64) {
	if h.quotaService == nil || len(spaceIDs) == 0 {
		return
	}

	seen := make(map[int64]struct{}, len(spaceIDs))
	unique := make([]int64, 0, len(spaceIDs))
	for _, spaceID := range spaceIDs {
		if spaceID == 0 {
			continue
		}
		if _, ok := seen[spaceID]; ok {
			continue
		}
		seen[spaceID] = struct{}{}
		unique = append(unique, spaceID)
	}

	switch len(unique) {
	case 0:
		return
	case 1:
		h.quotaService.Invalidate(unique[0])
	default:
		h.quotaService.InvalidateMany(unique...)
	}
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
		return nil, errors.New("Trash service is unavailable")
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
			return nil, errors.New("File or directory not found")
		}
		if browse.IsPermissionError(err) {
			return nil, errors.New("Permission denied")
		}
		return nil, errors.New("Failed to access file")
	}

	storageRelativePath, storageAbsPath, err := generateTrashStoragePath(spaceData.SpacePath, fileInfo.Name())
	if err != nil {
		if browse.IsPermissionError(err) {
			return nil, errors.New("Permission denied")
		}
		return nil, errors.New("Failed to allocate trash storage path")
	}

	if err := os.Rename(absPath, storageAbsPath); err != nil {
		if browse.IsPermissionError(err) {
			return nil, errors.New("Permission denied")
		}
		return nil, errors.New("Failed to move item into trash")
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
			return nil, errors.New("Failed to create trash metadata")
		}
		return nil, errors.New("Failed to create trash metadata")
	}

	return item, nil
}

func (h *Handler) markSearchIndexDirty(ctx context.Context, spaceID int64, action string) {
	if h.searchIndexer == nil {
		return
	}

	var err error
	switch action {
	case "rename", "delete", "delete-multiple", "trash", "trash-restore", "trash-delete", "trash-empty", "create-folder", "upload":
		err = h.searchIndexer.MarkSpaceDirty(ctx, spaceID)
	case "move", "copy":
		err = h.searchIndexer.MarkAllDirty(ctx)
	default:
		return
	}
	if err != nil {
		log.Warn().Err(err).Int64("space_id", spaceID).Str("action", action).Msg("failed to mark search index dirty")
	}
}

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

func (h *Handler) recordSpaceAudit(r *http.Request, event audit.Event, spaceID int64) {
	if h.auditRecorder == nil {
		return
	}
	if event.SpaceID == nil {
		event.SpaceID = &spaceID
	}
	if claims, ok := auth.ClaimsFromContext(r.Context()); ok {
		event.Actor = claims.Username
	}
	if event.RequestID == "" {
		event.RequestID = strings.TrimSpace(r.Header.Get("X-Request-Id"))
	}
	h.auditRecorder.RecordBestEffort(event)
}

func (h *Handler) recordDeniedFileActionAudit(r *http.Request, action string, spaceID int64, webErr *web.Error) *http.Request {
	if h.auditRecorder == nil || webErr == nil || auth.DeniedAuditRecorded(r.Context()) {
		return r
	}

	auditAction, ok := auth.DeniedAuditActionForSpaceFileAction(action)
	if !ok {
		return r
	}

	reason := "access_denied"
	code := "space.access_denied"
	switch webErr.Code {
	case http.StatusUnauthorized:
		reason = "unauthorized"
		code = "auth.unauthorized"
	case http.StatusForbidden:
		msg := strings.ToLower(strings.TrimSpace(webErr.Message))
		switch {
		case strings.Contains(msg, "invalid path"), strings.Contains(msg, "outside"), strings.Contains(msg, "reserved"):
			reason = "invalid_path"
			code = "space.invalid_path"
		case strings.Contains(msg, "permission denied"), strings.Contains(msg, "insufficient destination space permission"):
			reason = "permission_denied"
			code = "space.permission_denied"
		}
	}

	target := strings.TrimSpace(r.URL.Query().Get("path"))
	if target == "" {
		target = strings.TrimPrefix(strings.TrimSpace(r.URL.Path), "/api/spaces/")
	}
	if target == "" {
		target = fmt.Sprintf("space:%d", spaceID)
	}

	event := audit.Event{
		Action:    auditAction,
		Result:    audit.ResultDenied,
		Target:    target,
		RequestID: strings.TrimSpace(r.Header.Get("X-Request-Id")),
		SpaceID:   &spaceID,
		Metadata: map[string]any{
			"reason": reason,
			"code":   code,
			"status": webErr.Code,
		},
	}
	if claims, ok := auth.ClaimsFromContext(r.Context()); ok {
		event.Actor = claims.Username
	}

	h.auditRecorder.RecordBestEffort(event)
	return r.WithContext(auth.WithDeniedAuditRecorded(r.Context()))
}
