package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"taeu.kr/cohesion/internal/platform/web"
	"taeu.kr/cohesion/internal/space"
)

const maxMultipartFieldValueBytes = 1 << 20

var errUploadQuotaExceeded = errors.New("upload quota exceeded during stream")

type uploadQuotaWindow struct {
	enabled    bool
	maxBytes   int64
	usedBytes  int64
	quotaBytes int64
}

type uploadPlan struct {
	targetRelPath    string
	destPath         string
	resultFileName   string
	conflictPolicy   uploadConflictPolicy
	existingBytes    int64
	estimatedSize    int64
	quotaWindow      uploadQuotaWindow
	skip             bool
	originalFileName string
}

type quotaEnforcingWriter struct {
	writer    io.Writer
	remaining int64
}

func (w *quotaEnforcingWriter) Write(p []byte) (int, error) {
	if w.remaining <= 0 {
		return 0, errUploadQuotaExceeded
	}
	if int64(len(p)) <= w.remaining {
		n, err := w.writer.Write(p)
		w.remaining -= int64(n)
		return n, err
	}

	allowed := int(w.remaining)
	n, err := w.writer.Write(p[:allowed])
	w.remaining -= int64(n)
	if err != nil {
		return n, err
	}
	return n, errUploadQuotaExceeded
}

func createUploadStageFile(destPath string) (*os.File, string, error) {
	if strings.TrimSpace(destPath) == "" {
		file, err := os.CreateTemp("", "cohesion-upload-*")
		if err != nil {
			return nil, "", err
		}
		return file, file.Name(), nil
	}

	stagePath, err := resolveUniqueSiblingPath(destPath, "upload")
	if err != nil {
		return nil, "", err
	}
	file, err := os.Create(stagePath)
	if err != nil {
		return nil, "", err
	}
	return file, stagePath, nil
}

func finalizeUploadedFile(stagePath, destPath string, replaceExisting bool) error {
	if replaceExisting {
		if err := moveWithDestinationSwap(stagePath, destPath); err == nil {
			return nil
		}
	} else if err := os.Rename(stagePath, destPath); err == nil {
		return nil
	}

	fallbackStagePath, err := resolveUniqueSiblingPath(destPath, "upload-finalize")
	if err != nil {
		return err
	}
	defer os.Remove(fallbackStagePath) //nolint:errcheck

	if err := copyFile(stagePath, fallbackStagePath); err != nil {
		return err
	}
	if replaceExisting {
		if err := moveWithDestinationSwap(fallbackStagePath, destPath); err != nil {
			return err
		}
	} else {
		if err := os.Rename(fallbackStagePath, destPath); err != nil {
			return err
		}
	}
	return os.Remove(stagePath)
}

func createQuotaExceededWebError(spaceID, usedBytes, quotaBytes, deltaBytes int64) *web.Error {
	return &web.Error{
		Code:    http.StatusInsufficientStorage,
		Message: "Space quota exceeded",
		Err: &space.QuotaExceededError{
			SpaceID:    spaceID,
			UsedBytes:  usedBytes,
			QuotaBytes: quotaBytes,
			DeltaBytes: deltaBytes,
		},
	}
}

func (h *Handler) calculateUploadQuotaWindow(ctx context.Context, spaceID int64, existingBytes int64, estimatedSize int64) (uploadQuotaWindow, *web.Error) {
	if h.quotaService == nil {
		return uploadQuotaWindow{}, nil
	}

	usage, err := h.quotaService.GetSpaceUsage(ctx, spaceID)
	if err != nil {
		return uploadQuotaWindow{}, &web.Error{Code: http.StatusInternalServerError, Message: "Failed to evaluate space quota", Err: err}
	}
	if usage.QuotaBytes == nil {
		return uploadQuotaWindow{}, nil
	}

	quotaBytes := *usage.QuotaBytes
	if estimatedSize >= 0 {
		delta := estimatedSize - existingBytes
		if delta < 0 {
			delta = 0
		}
		if err := h.quotaService.EnsureCanWrite(ctx, spaceID, delta); err != nil {
			var quotaErr *space.QuotaExceededError
			if errors.As(err, &quotaErr) {
				return uploadQuotaWindow{}, &web.Error{Code: http.StatusInsufficientStorage, Message: "Space quota exceeded", Err: err}
			}
			return uploadQuotaWindow{}, &web.Error{Code: http.StatusInternalServerError, Message: "Failed to evaluate space quota", Err: err}
		}
	}

	maxBytes := quotaBytes - (usage.UsedBytes - existingBytes)
	if maxBytes < 0 {
		maxBytes = 0
	}

	return uploadQuotaWindow{
		enabled:    true,
		maxBytes:   maxBytes,
		usedBytes:  usage.UsedBytes,
		quotaBytes: quotaBytes,
	}, nil
}

func (h *Handler) buildUploadPlan(
	ctx context.Context,
	spaceID int64,
	spaceRoot string,
	targetRelPath string,
	fileName string,
	estimatedSize int64,
	rawConflictPolicy string,
	overwriteLegacy bool,
) (*uploadPlan, *web.Error) {
	if err := ensurePathOutsideTrash(targetRelPath); err != nil {
		return nil, &web.Error{Code: http.StatusForbidden, Message: "Access denied: invalid path", Err: err}
	}
	absTarget, err := resolveAbsPath(spaceRoot, targetRelPath)
	if err != nil {
		return nil, &web.Error{Code: http.StatusForbidden, Message: "Access denied: invalid path"}
	}

	targetInfo, err := os.Stat(absTarget)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &web.Error{Code: http.StatusNotFound, Message: "Target directory not found", Err: err}
		}
		return nil, &web.Error{Code: http.StatusInternalServerError, Message: "Failed to access target directory", Err: err}
	}
	if !targetInfo.IsDir() {
		return nil, &web.Error{Code: http.StatusBadRequest, Message: "Target path must be a directory"}
	}

	conflictPolicy, hasConflictPolicy, err := resolveUploadConflictPolicy(rawConflictPolicy, overwriteLegacy)
	if err != nil {
		return nil, &web.Error{Code: http.StatusBadRequest, Message: "Invalid conflict policy", Err: err}
	}

	destPath := filepath.Join(absTarget, fileName)
	resultFileName := fileName
	existingBytes := int64(0)

	if existingInfo, err := os.Stat(destPath); err == nil {
		if !hasConflictPolicy {
			return nil, &web.Error{Code: http.StatusConflict, Message: "File already exists"}
		}
		switch conflictPolicy {
		case uploadConflictPolicyOverwrite:
			if existingInfo.IsDir() {
				return nil, &web.Error{Code: http.StatusConflict, Message: "Directory already exists"}
			}
			existingBytes = existingInfo.Size()
		case uploadConflictPolicyRename:
			renamedPath, renamedFileName, resolveErr := resolveUploadRenamePath(destPath)
			if resolveErr != nil {
				return nil, &web.Error{Code: http.StatusInternalServerError, Message: "Failed to resolve rename destination", Err: resolveErr}
			}
			destPath = renamedPath
			resultFileName = renamedFileName
		case uploadConflictPolicySkip:
			return &uploadPlan{
				targetRelPath:    targetRelPath,
				destPath:         destPath,
				resultFileName:   fileName,
				conflictPolicy:   conflictPolicy,
				skip:             true,
				estimatedSize:    estimatedSize,
				originalFileName: fileName,
			}, nil
		}
	} else if !os.IsNotExist(err) {
		return nil, &web.Error{Code: http.StatusInternalServerError, Message: "Failed to inspect destination path", Err: err}
	}

	quotaWindow, webErr := h.calculateUploadQuotaWindow(ctx, spaceID, existingBytes, estimatedSize)
	if webErr != nil {
		return nil, webErr
	}

	return &uploadPlan{
		targetRelPath:    targetRelPath,
		destPath:         destPath,
		resultFileName:   resultFileName,
		conflictPolicy:   conflictPolicy,
		existingBytes:    existingBytes,
		estimatedSize:    estimatedSize,
		quotaWindow:      quotaWindow,
		originalFileName: fileName,
	}, nil
}

func (h *Handler) acquireUploadReservation(ctx context.Context, spaceID int64, plan *uploadPlan) (string, *web.Error) {
	if h.quotaService == nil || plan == nil || !plan.quotaWindow.enabled || plan.estimatedSize < 0 {
		return "", nil
	}

	deltaBytes := plan.estimatedSize - plan.existingBytes
	if deltaBytes <= 0 {
		return "", nil
	}

	reservationID, err := generateDownloadTicketToken()
	if err != nil {
		return "", &web.Error{Code: http.StatusInternalServerError, Message: "Failed to prepare upload reservation", Err: err}
	}

	if _, err := h.quotaService.AcquireWriteReservation(ctx, spaceID, reservationID, deltaBytes); err != nil {
		var quotaErr *space.QuotaExceededError
		if errors.As(err, &quotaErr) {
			return "", &web.Error{Code: http.StatusInsufficientStorage, Message: "Space quota exceeded", Err: err}
		}
		return "", &web.Error{Code: http.StatusInternalServerError, Message: "Failed to reserve upload quota", Err: err}
	}

	plan.quotaWindow.maxBytes = plan.existingBytes + deltaBytes
	return reservationID, nil
}

func readMultipartFieldValue(part *multipart.Part) (string, error) {
	value, err := io.ReadAll(io.LimitReader(part, maxMultipartFieldValueBytes))
	if err != nil {
		return "", err
	}
	if len(value) >= maxMultipartFieldValueBytes {
		return "", fmt.Errorf("multipart field too large")
	}
	return string(value), nil
}

func serveAttachmentContent(w http.ResponseWriter, r *http.Request, file *os.File, fileInfo os.FileInfo, fileName string, contentType string) {
	downloadName := strings.TrimSpace(fileName)
	if downloadName == "" {
		downloadName = fileInfo.Name()
	}

	resolvedContentType := strings.TrimSpace(contentType)
	if resolvedContentType == "" {
		resolvedContentType = mime.TypeByExtension(filepath.Ext(downloadName))
	}
	if resolvedContentType == "" {
		resolvedContentType = "application/octet-stream"
	}

	safeFileName := strings.ReplaceAll(downloadName, `"`, "")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, safeFileName))
	w.Header().Set("Content-Type", resolvedContentType)
	http.ServeContent(w, r, downloadName, fileInfo.ModTime(), file)
}
