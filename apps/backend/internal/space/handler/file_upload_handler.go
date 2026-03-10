package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"taeu.kr/cohesion/internal/audit"
	"taeu.kr/cohesion/internal/platform/web"
)

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

	reader, err := r.MultipartReader()
	if err != nil {
		return &web.Error{Code: http.StatusBadRequest, Message: "Failed to parse multipart form", Err: err}
	}

	var (
		targetRelPath       string
		rawConflictPolicy   string
		overwriteLegacy     bool
		pathProvided        bool
		declaredUploadSize  int64 = -1
		fileName            string
		resultFileName      string
		stageFile           *os.File
		stagePath           string
		fileSize            int64
		plan                *uploadPlan
		uploadReservationID string
	)

	defer func() {
		if uploadReservationID != "" && h.quotaService != nil {
			h.quotaService.ReleaseWriteReservation(uploadReservationID)
		}
		if stageFile != nil {
			_ = stageFile.Close()
		}
		if stagePath != "" {
			_ = os.Remove(stagePath)
		}
	}()

	for {
		part, nextErr := reader.NextPart()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			return &web.Error{Code: http.StatusBadRequest, Message: "Failed to read multipart form", Err: nextErr}
		}

		if part.FileName() == "" {
			value, readErr := readMultipartFieldValue(part)
			if readErr != nil {
				return &web.Error{Code: http.StatusBadRequest, Message: "Failed to read multipart field", Err: readErr}
			}
			switch part.FormName() {
			case "path":
				targetRelPath = value
				pathProvided = true
			case "conflictPolicy":
				rawConflictPolicy = value
			case "overwrite":
				overwriteLegacy = strings.EqualFold(strings.TrimSpace(value), "true")
			case "size":
				if strings.TrimSpace(value) == "" {
					declaredUploadSize = -1
					break
				}
				parsedSize, parseErr := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
				if parseErr != nil || parsedSize < 0 {
					return &web.Error{Code: http.StatusBadRequest, Message: "Invalid upload size", Err: parseErr}
				}
				declaredUploadSize = parsedSize
			}
			continue
		}

		if fileName != "" {
			return &web.Error{Code: http.StatusBadRequest, Message: "Only one file upload is supported"}
		}

		fileName = part.FileName()
		if strings.TrimSpace(fileName) == "" {
			return &web.Error{Code: http.StatusBadRequest, Message: "Uploaded file name is required"}
		}

		if pathProvided {
			plan, webErr = h.buildUploadPlan(r.Context(), spaceID, spaceData.SpacePath, targetRelPath, fileName, declaredUploadSize, rawConflictPolicy, overwriteLegacy)
			if webErr != nil {
				return webErr
			}
			if uploadReservationID == "" {
				uploadReservationID, webErr = h.acquireUploadReservation(r.Context(), spaceID, plan)
				if webErr != nil {
					return webErr
				}
			}
		}

		if plan != nil && plan.skip {
			if _, err := io.Copy(io.Discard, part); err != nil {
				return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to discard skipped upload", Err: err}
			}
			resultFileName = plan.resultFileName
			fileSize = declaredUploadSize
			continue
		}

		stageDestPath := ""
		if plan != nil {
			stageDestPath = plan.destPath
		}
		stageFile, stagePath, err = createUploadStageFile(stageDestPath)
		if err != nil {
			return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to prepare upload staging file", Err: err}
		}

		var writer io.Writer = stageFile
		if plan != nil && plan.quotaWindow.enabled {
			writer = &quotaEnforcingWriter{writer: stageFile, remaining: plan.quotaWindow.maxBytes}
		}
		fileSize, err = io.Copy(writer, part)
		if err != nil {
			if errors.Is(err, errUploadQuotaExceeded) {
				return createQuotaExceededWebError(spaceID, plan.quotaWindow.usedBytes, plan.quotaWindow.quotaBytes, fileSize-plan.existingBytes)
			}
			return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to save uploaded file", Err: err}
		}
	}

	if strings.TrimSpace(fileName) == "" {
		return &web.Error{Code: http.StatusBadRequest, Message: "Failed to get uploaded file"}
	}

	if plan == nil {
		plan, webErr = h.buildUploadPlan(r.Context(), spaceID, spaceData.SpacePath, targetRelPath, fileName, fileSize, rawConflictPolicy, overwriteLegacy)
		if webErr != nil {
			return webErr
		}
	}

	if plan.skip {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]string{
			"message":  "Skipped existing file",
			"filename": plan.resultFileName,
			"status":   "skipped",
		}); err != nil {
			return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to encode upload response", Err: err}
		}
		h.recordSpaceAudit(r, audit.Event{
			Action: "file.upload",
			Result: audit.ResultPartial,
			Target: filepath.ToSlash(filepath.Join(targetRelPath, plan.resultFileName)),
			Metadata: map[string]any{
				"filename":       plan.resultFileName,
				"status":         "skipped",
				"conflictPolicy": string(plan.conflictPolicy),
			},
		}, spaceID)
		return nil
	}

	if stageFile == nil {
		return &web.Error{Code: http.StatusBadRequest, Message: "Failed to stage uploaded file"}
	}
	if err := stageFile.Close(); err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to finalize upload staging file", Err: err}
	}
	stageFile = nil

	projectedDelta := fileSize - plan.existingBytes
	if projectedDelta < 0 {
		projectedDelta = 0
	}
	h.invalidateQuotaForSpaces(spaceID)
	if webErr := h.ensureSpaceQuotaForWrite(r.Context(), spaceID, projectedDelta); webErr != nil {
		return webErr
	}

	if err := finalizeUploadedFile(stagePath, plan.destPath, plan.existingBytes > 0); err != nil {
		return storageOperationWebError(err, "Failed to finalize uploaded file")
	}
	stagePath = ""
	resultFileName = plan.resultFileName

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"message":  "Successfully uploaded",
		"filename": resultFileName,
		"status":   "uploaded",
	}); err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to encode upload response", Err: err}
	}
	h.recordSpaceAudit(r, audit.Event{
		Action: "file.upload",
		Result: audit.ResultSuccess,
		Target: filepath.ToSlash(filepath.Join(targetRelPath, resultFileName)),
		Metadata: map[string]any{
			"filename":       resultFileName,
			"size":           fileSize,
			"status":         "uploaded",
			"conflictPolicy": string(plan.conflictPolicy),
		},
	}, spaceID)
	h.invalidateQuotaForSpaces(spaceID)
	return nil
}
