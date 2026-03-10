package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"taeu.kr/cohesion/internal/account"
	"taeu.kr/cohesion/internal/audit"
	"taeu.kr/cohesion/internal/platform/logging"
	"taeu.kr/cohesion/internal/platform/web"
)

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
		return storageAccessWebError(err, "File or directory not found", "Failed to access file")
	}

	if err := os.Rename(absPath, newAbsPath); err != nil {
		h.recordSpaceAudit(r, audit.Event{
			Action: "file.rename",
			Result: audit.ResultFailure,
			Target: req.Path,
			Metadata: map[string]any{
				"path":    req.Path,
				"newName": req.NewName,
				"reason":  "rename_failed",
			},
		}, spaceID)
		return storageOperationWebError(err, "Failed to rename")
	}
	h.recordSpaceAudit(r, audit.Event{
		Action: "file.rename",
		Result: audit.ResultSuccess,
		Target: req.Path,
		Metadata: map[string]any{
			"path":    req.Path,
			"newName": req.NewName,
		},
	}, spaceID)

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
		message := strings.TrimSpace(err.Error())
		lowerMessage := strings.ToLower(message)
		switch {
		case strings.Contains(lowerMessage, "path is required"):
			return &web.Error{Code: http.StatusBadRequest, Message: "path is required", Err: err}
		case strings.Contains(lowerMessage, "access denied"):
			return &web.Error{Code: http.StatusForbidden, Message: "Access denied: invalid path", Err: err}
		case strings.Contains(lowerMessage, "file or directory not found"):
			return &web.Error{Code: http.StatusNotFound, Message: "File or directory not found", Err: err}
		case strings.Contains(lowerMessage, "unauthorized"):
			return &web.Error{Code: http.StatusUnauthorized, Message: "Unauthorized", Err: err}
		case strings.Contains(lowerMessage, "permission denied"):
			return &web.Error{Code: http.StatusForbidden, Message: "Permission denied", Err: err}
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
	h.recordSpaceAudit(r, audit.Event{
		Action: "file.delete",
		Result: audit.ResultSuccess,
		Target: item.OriginalPath,
		Metadata: map[string]any{
			"path":        item.OriginalPath,
			"trashItemId": item.ID,
		},
	}, spaceID)
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
	result := audit.ResultSuccess
	if len(succeeded) == 0 && len(failed) > 0 {
		result = audit.ResultFailure
	} else if len(failed) > 0 {
		result = audit.ResultPartial
	}
	h.recordSpaceAudit(r, audit.Event{
		Action: "file.delete-multiple",
		Result: result,
		Target: fmt.Sprintf("%d items", len(req.Paths)),
		Metadata: map[string]any{
			"total":     len(req.Paths),
			"succeeded": len(succeeded),
			"failed":    len(failed),
		},
	}, spaceID)
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
			return storageAccessWebError(statErr, "", "Failed to inspect trash item")
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
				Reason:       safeFilesystemReason("Failed to access trash item", storageStatErr),
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
				Reason:       safeFilesystemReason("Failed to prepare restore directory", mkdirErr),
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
						Reason:       safeFilesystemReason("Failed to restore with overwrite", overwriteErr),
					})
					continue
				}
				if deleteErr := h.trashService.DeleteTrashItem(r.Context(), item.ID); deleteErr != nil {
					logging.Event(log.Warn(), logging.ComponentStorage, "warn.trash.metadata_cleanup_failed").
						Int64("trash_id", item.ID).
						Err(deleteErr).
						Msg("trash metadata cleanup failed")
				}
				succeeded = append(succeeded, restoreSuccess{ID: item.ID, OriginalPath: item.OriginalPath})
				continue
			case uploadConflictPolicyRename:
				renamedAbsPath, _, renameErr := resolveUploadRenamePath(destAbsPath)
				if renameErr != nil {
					failed = append(failed, restoreFailed{
						ID:           item.ID,
						OriginalPath: item.OriginalPath,
						Reason:       safeFilesystemReason("Failed to resolve restore rename destination", renameErr),
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
				Reason:       safeFilesystemReason("Failed to access destination path", destStatErr),
			})
			continue
		}

		if moveErr := os.Rename(absStoragePath, finalDestAbsPath); moveErr != nil {
			failed = append(failed, restoreFailed{
				ID:           item.ID,
				OriginalPath: item.OriginalPath,
				Reason:       safeFilesystemReason("Failed to restore item", moveErr),
			})
			continue
		}
		if deleteErr := h.trashService.DeleteTrashItem(r.Context(), item.ID); deleteErr != nil {
			logging.Event(log.Warn(), logging.ComponentStorage, "warn.trash.metadata_cleanup_failed").
				Int64("trash_id", item.ID).
				Err(deleteErr).
				Msg("trash metadata cleanup failed")
		}
		succeeded = append(succeeded, restoreSuccess{ID: item.ID, OriginalPath: item.OriginalPath})
	}
	if len(succeeded) > 0 {
		h.invalidateQuotaForSpaces(spaceID)
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
			failed = append(failed, deleteFailed{ID: item.ID, Reason: safeFilesystemReason("Failed to delete trash file", removeErr)})
			continue
		}
		if deleteErr := h.trashService.DeleteTrashItem(r.Context(), item.ID); deleteErr != nil {
			failed = append(failed, deleteFailed{ID: item.ID, Reason: "Failed to delete trash metadata"})
			continue
		}
		succeeded = append(succeeded, deleteSuccess{ID: item.ID})
	}
	if len(succeeded) > 0 {
		h.invalidateQuotaForSpaces(spaceID)
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
			failed = append(failed, emptyFailed{ID: item.ID, Reason: safeFilesystemReason("Failed to delete trash file", removeErr)})
			continue
		}
		if deleteErr := h.trashService.DeleteTrashItem(r.Context(), item.ID); deleteErr != nil {
			failed = append(failed, emptyFailed{ID: item.ID, Reason: "Failed to delete trash metadata"})
			continue
		}
		removed++
	}
	if removed > 0 {
		h.invalidateQuotaForSpaces(spaceID)
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
		return storageAccessWebError(err, "Parent directory not found", "Failed to access parent directory")
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

	if err := os.Mkdir(folderPath, 0o755); err != nil {
		return storageOperationWebError(err, "Failed to create folder")
	}
	h.invalidateQuotaForSpaces(spaceID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Successfully created",
		"name":    req.FolderName,
	})
	h.recordSpaceAudit(r, audit.Event{
		Action: "file.mkdir",
		Result: audit.ResultSuccess,
		Target: filepath.ToSlash(filepath.Join(req.ParentPath, req.FolderName)),
		Metadata: map[string]any{
			"path": req.ParentPath,
			"name": req.FolderName,
		},
	}, spaceID)
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
		return storageAccessWebError(err, "Destination directory not found", "Failed to access destination directory")
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
				failed = append(failed, moveResult{Path: relSrc, Reason: safeFilesystemReason("Failed to access source", err)})
			}
			continue
		}
		sourceSize, sizeErr := h.quotaService.CalculatePathSize(r.Context(), absSrc)
		if sizeErr != nil {
			failed = append(failed, moveResult{Path: relSrc, Reason: safeFilesystemReason("Failed to evaluate source size", sizeErr)})
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
					failed = append(failed, moveResult{Path: relSrc, Reason: safeFilesystemReason("Failed to evaluate destination size", existingSizeErr)})
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
					failed = append(failed, moveResult{Path: relSrc, Reason: safeFilesystemReason("Failed to overwrite destination", overwriteErr)})
					continue
				}
				succeeded = append(succeeded, relSrc)
				quotaInvalidationTargets[dstSpaceID] = struct{}{}
				quotaInvalidationTargets[spaceID] = struct{}{}
				continue
			case uploadConflictPolicyRename:
				renamedPath, _, renameErr := resolveUploadRenamePath(destPath)
				if renameErr != nil {
					failed = append(failed, moveResult{Path: relSrc, Reason: safeFilesystemReason("Failed to resolve rename destination", renameErr)})
					continue
				}
				destPath = renamedPath
			case uploadConflictPolicySkip:
				skipped = append(skipped, relSrc)
				continue
			}
		} else if !os.IsNotExist(statErr) {
			failed = append(failed, moveResult{Path: relSrc, Reason: safeFilesystemReason("Failed to access destination", statErr)})
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
			failed = append(failed, moveResult{Path: relSrc, Reason: safeFilesystemReason("Failed to move", err)})
		} else {
			succeeded = append(succeeded, relSrc)
			quotaInvalidationTargets[dstSpaceID] = struct{}{}
			quotaInvalidationTargets[spaceID] = struct{}{}
		}
	}
	if len(quotaInvalidationTargets) > 0 {
		spaceIDs := make([]int64, 0, len(quotaInvalidationTargets))
		for targetID := range quotaInvalidationTargets {
			spaceIDs = append(spaceIDs, targetID)
		}
		h.invalidateQuotaForSpaces(spaceIDs...)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"succeeded": succeeded,
		"failed":    failed,
		"skipped":   skipped,
	})
	result := audit.ResultSuccess
	if len(succeeded) == 0 && len(failed) > 0 {
		result = audit.ResultFailure
	} else if len(failed) > 0 || len(skipped) > 0 {
		result = audit.ResultPartial
	}
	h.recordSpaceAudit(r, audit.Event{
		Action: "file.move",
		Result: result,
		Target: req.Destination.Path,
		Metadata: map[string]any{
			"sourceCount": len(req.Sources),
			"succeeded":   len(succeeded),
			"failed":      len(failed),
			"skipped":     len(skipped),
			"fromSpaceId": spaceID,
			"toSpaceId":   dstSpaceID,
		},
	}, spaceID)
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
		return storageAccessWebError(err, "Destination directory not found", "Failed to access destination directory")
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
				failed = append(failed, copyResult{Path: relSrc, Reason: safeFilesystemReason("Failed to access source", err)})
			}
			continue
		}
		sourceSize, sizeErr := h.quotaService.CalculatePathSize(r.Context(), absSrc)
		if sizeErr != nil {
			failed = append(failed, copyResult{Path: relSrc, Reason: safeFilesystemReason("Failed to evaluate source size", sizeErr)})
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
					failed = append(failed, copyResult{Path: relSrc, Reason: safeFilesystemReason("Failed to evaluate destination size", existingSizeErr)})
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
					failed = append(failed, copyResult{Path: relSrc, Reason: safeFilesystemReason("Failed to overwrite destination", overwriteErr)})
					continue
				}
				succeeded = append(succeeded, relSrc)
				quotaInvalidationTargets[dstSpaceID] = struct{}{}
				continue
			case uploadConflictPolicyRename:
				renamedPath, _, renameErr := resolveUploadRenamePath(destPath)
				if renameErr != nil {
					failed = append(failed, copyResult{Path: relSrc, Reason: safeFilesystemReason("Failed to resolve rename destination", renameErr)})
					continue
				}
				destPath = renamedPath
			case uploadConflictPolicySkip:
				skipped = append(skipped, relSrc)
				continue
			}
		} else if !os.IsNotExist(statErr) {
			failed = append(failed, copyResult{Path: relSrc, Reason: safeFilesystemReason("Failed to access destination", statErr)})
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
			failed = append(failed, copyResult{Path: relSrc, Reason: safeFilesystemReason("Failed to copy", copyErr)})
		} else {
			succeeded = append(succeeded, relSrc)
			quotaInvalidationTargets[dstSpaceID] = struct{}{}
		}
	}
	if len(quotaInvalidationTargets) > 0 {
		spaceIDs := make([]int64, 0, len(quotaInvalidationTargets))
		for targetID := range quotaInvalidationTargets {
			spaceIDs = append(spaceIDs, targetID)
		}
		h.invalidateQuotaForSpaces(spaceIDs...)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"succeeded": succeeded,
		"failed":    failed,
		"skipped":   skipped,
	})
	result := audit.ResultSuccess
	if len(succeeded) == 0 && len(failed) > 0 {
		result = audit.ResultFailure
	} else if len(failed) > 0 || len(skipped) > 0 {
		result = audit.ResultPartial
	}
	h.recordSpaceAudit(r, audit.Event{
		Action: "file.copy",
		Result: result,
		Target: req.Destination.Path,
		Metadata: map[string]any{
			"sourceCount": len(req.Sources),
			"succeeded":   len(succeeded),
			"failed":      len(failed),
			"skipped":     len(skipped),
			"fromSpaceId": spaceID,
			"toSpaceId":   dstSpaceID,
		},
	}, spaceID)
	return nil
}

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
