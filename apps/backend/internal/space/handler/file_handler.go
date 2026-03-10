package handler

import (
	"fmt"
	"net/http"

	"taeu.kr/cohesion/internal/platform/web"
)

// handleSpaceFiles는 /api/spaces/{id}/files/* 요청을 액션별로 분기합니다.
func (h *Handler) handleSpaceFiles(w http.ResponseWriter, r *http.Request, spaceID int64, action string) *web.Error {
	var webErr *web.Error

	switch action {
	case "download":
		webErr = h.handleFileDownload(w, r, spaceID)
	case "download-ticket":
		webErr = h.handleFileDownloadTicket(w, r, spaceID)
	case "archive-downloads":
		webErr = h.handleArchiveDownloads(w, r, spaceID)
	case "archive-download-ticket":
		webErr = h.handleArchiveDownloadTicket(w, r, spaceID)
	case "rename":
		webErr = h.handleFileRename(w, r, spaceID)
	case "delete":
		webErr = h.handleFileDelete(w, r, spaceID)
	case "delete-multiple":
		webErr = h.handleFileDeleteMultiple(w, r, spaceID)
	case "trash":
		webErr = h.handleTrashList(w, r, spaceID)
	case "trash-restore":
		webErr = h.handleTrashRestore(w, r, spaceID)
	case "trash-delete":
		webErr = h.handleTrashDelete(w, r, spaceID)
	case "trash-empty":
		webErr = h.handleTrashEmpty(w, r, spaceID)
	case "create-folder":
		webErr = h.handleFileCreateFolder(w, r, spaceID)
	case "upload":
		webErr = h.handleFileUpload(w, r, spaceID)
	case "move":
		webErr = h.handleFileMove(w, r, spaceID)
	case "copy":
		webErr = h.handleFileCopy(w, r, spaceID)
	case "download-multiple":
		webErr = h.handleFileDownloadMultiple(w, r, spaceID)
	case "download-multiple-ticket":
		webErr = h.handleFileDownloadMultipleTicket(w, r, spaceID)
	default:
		return &web.Error{
			Code:    http.StatusNotFound,
			Message: fmt.Sprintf("Unknown file action: %s", action),
		}
	}

	if webErr != nil && (webErr.Code == http.StatusForbidden || webErr.Code == http.StatusUnauthorized) {
		r = h.recordDeniedFileActionAudit(r, action, spaceID, webErr)
	}
	if webErr == nil {
		h.markSearchIndexDirty(r.Context(), spaceID, action)
	}
	return webErr
}
