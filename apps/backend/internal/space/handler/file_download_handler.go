package handler

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"taeu.kr/cohesion/internal/audit"
	"taeu.kr/cohesion/internal/auth"
	"taeu.kr/cohesion/internal/platform/web"
)

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
		return storageAccessWebError(err, "File not found", "Failed to access file")
	}

	if fileInfo.IsDir() {
		downloadErr := h.downloadFolderAsZip(w, absPath, fileInfo.Name())
		if downloadErr != nil {
			h.recordSpaceAudit(r, audit.Event{
				Action: "file.download",
				Result: audit.ResultFailure,
				Target: relativePath,
				Metadata: map[string]any{
					"path":        relativePath,
					"filename":    fileInfo.Name() + ".zip",
					"format":      "zip",
					"sourceCount": 1,
					"status":      "failed",
					"reason":      "stream_failed",
				},
			}, spaceID)
		} else {
			h.recordSpaceAudit(r, audit.Event{
				Action: "file.download",
				Result: audit.ResultSuccess,
				Target: relativePath,
				Metadata: map[string]any{
					"path":        relativePath,
					"filename":    fileInfo.Name() + ".zip",
					"format":      "zip",
					"sourceCount": 1,
					"status":      "downloaded",
				},
			}, spaceID)
		}
		return downloadErr
	}

	downloadErr := h.streamFileDownload(w, r, absPath)
	if downloadErr != nil {
		h.recordSpaceAudit(r, audit.Event{
			Action: "file.download",
			Result: audit.ResultFailure,
			Target: relativePath,
			Metadata: map[string]any{
				"path":        relativePath,
				"filename":    fileInfo.Name(),
				"size":        fileInfo.Size(),
				"format":      "file",
				"sourceCount": 1,
				"status":      "failed",
				"reason":      "stream_failed",
			},
		}, spaceID)
	} else {
		h.recordSpaceAudit(r, audit.Event{
			Action: "file.download",
			Result: audit.ResultSuccess,
			Target: relativePath,
			Metadata: map[string]any{
				"path":        relativePath,
				"filename":    fileInfo.Name(),
				"size":        fileInfo.Size(),
				"format":      "file",
				"sourceCount": 1,
				"status":      "downloaded",
			},
		}, spaceID)
	}
	return downloadErr
}

func (h *Handler) streamFileDownload(w http.ResponseWriter, r *http.Request, absPath string) *web.Error {
	file, err := os.Open(absPath)
	if err != nil {
		return storageAccessWebError(err, "File not found", "Failed to open file")
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return storageAccessWebError(err, "File not found", "Failed to inspect file")
	}

	serveAttachmentContent(w, r, file, fileInfo, filepath.Base(absPath), "")
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
		return storageAccessWebError(err, "File not found", "Failed to access file")
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
		"file.download-ticket",
		&spaceID,
		contentType,
		contentSize,
		removeAfterUse,
	)
	if err != nil {
		if removeAfterUse {
			os.Remove(downloadFilePath) //nolint:errcheck
		}
		h.recordSpaceAudit(r, audit.Event{
			Action: "file.download-ticket",
			Result: audit.ResultFailure,
			Target: req.Path,
			Metadata: map[string]any{
				"path":     req.Path,
				"filename": downloadFileName,
				"size":     contentSize,
				"format":   contentType,
				"status":   "failed",
				"reason":   "issue_ticket_failed",
			},
		}, spaceID)
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to issue download ticket", Err: err}
	}
	h.recordSpaceAudit(r, audit.Event{
		Action: "file.download-ticket",
		Result: audit.ResultSuccess,
		Target: req.Path,
		Metadata: map[string]any{
			"path":     req.Path,
			"filename": ticket.FileName,
			"size":     contentSize,
			"format":   contentType,
			"status":   "ticket_issued",
		},
	}, spaceID)

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
			if storageErr := storageAccessWebError(err, "", fmt.Sprintf("Failed to access path: %s", relPath)); storageErr != nil {
				return storageErr
			}
		}
		absPaths = append(absPaths, absPath)
	}

	if len(absPaths) == 1 {
		fileInfo, err := os.Stat(absPaths[0])
		if err != nil {
			return storageAccessWebError(err, "File not found", "Failed to access file")
		}

		if fileInfo.IsDir() {
			downloadErr := h.downloadFolderAsZip(w, absPaths[0], fileInfo.Name())
			if downloadErr != nil {
				h.recordSpaceAudit(r, audit.Event{
					Action: "file.download",
					Result: audit.ResultFailure,
					Target: req.Paths[0],
					Metadata: map[string]any{
						"path":        req.Paths[0],
						"filename":    fileInfo.Name() + ".zip",
						"format":      "zip",
						"sourceCount": 1,
						"status":      "failed",
						"reason":      "stream_failed",
					},
				}, spaceID)
			} else {
				h.recordSpaceAudit(r, audit.Event{
					Action: "file.download",
					Result: audit.ResultSuccess,
					Target: req.Paths[0],
					Metadata: map[string]any{
						"path":        req.Paths[0],
						"filename":    fileInfo.Name() + ".zip",
						"format":      "zip",
						"sourceCount": 1,
						"status":      "downloaded",
					},
				}, spaceID)
			}
			return downloadErr
		}

		downloadErr := h.streamFileDownload(w, r, absPaths[0])
		if downloadErr != nil {
			h.recordSpaceAudit(r, audit.Event{
				Action: "file.download",
				Result: audit.ResultFailure,
				Target: req.Paths[0],
				Metadata: map[string]any{
					"path":        req.Paths[0],
					"filename":    fileInfo.Name(),
					"size":        fileInfo.Size(),
					"format":      "file",
					"sourceCount": 1,
					"status":      "failed",
					"reason":      "stream_failed",
				},
			}, spaceID)
		} else {
			h.recordSpaceAudit(r, audit.Event{
				Action: "file.download",
				Result: audit.ResultSuccess,
				Target: req.Paths[0],
				Metadata: map[string]any{
					"path":        req.Paths[0],
					"filename":    fileInfo.Name(),
					"size":        fileInfo.Size(),
					"format":      "file",
					"sourceCount": 1,
					"status":      "downloaded",
				},
			}, spaceID)
		}
		return downloadErr
	}

	zipFileName := fmt.Sprintf("download-%d.zip", os.Getpid())
	downloadErr := h.streamZipDownload(w, zipFileName, func(zipWriter *zip.Writer) *web.Error {
		for i, absPath := range absPaths {
			if err := addToZip(zipWriter, absPath, filepath.Base(req.Paths[i])); err != nil {
				return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to create zip archive", Err: err}
			}
		}
		return nil
	})
	if downloadErr != nil {
		h.recordSpaceAudit(r, audit.Event{
			Action: "file.download-multiple",
			Result: audit.ResultFailure,
			Target: fmt.Sprintf("%d items", len(req.Paths)),
			Metadata: map[string]any{
				"sourceCount": len(req.Paths),
				"filename":    zipFileName,
				"format":      "zip",
				"status":      "failed",
				"reason":      "stream_failed",
			},
		}, spaceID)
	} else {
		h.recordSpaceAudit(r, audit.Event{
			Action: "file.download-multiple",
			Result: audit.ResultSuccess,
			Target: fmt.Sprintf("%d items", len(req.Paths)),
			Metadata: map[string]any{
				"sourceCount": len(req.Paths),
				"filename":    zipFileName,
				"format":      "zip",
				"status":      "downloaded",
			},
		}, spaceID)
	}
	return downloadErr
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
			if storageErr := storageAccessWebError(err, "", fmt.Sprintf("Failed to access path: %s", relPath)); storageErr != nil {
				return storageErr
			}
		}
		absPaths = append(absPaths, absPath)
	}

	zipFileName := fmt.Sprintf("download-%d.zip", os.Getpid())
	zipTempPath, zipSize, zipErr := h.buildZipTempArchive(func(zipWriter *zip.Writer) *web.Error {
		for i, absPath := range absPaths {
			if err := addToZip(zipWriter, absPath, filepath.Base(req.Paths[i])); err != nil {
				return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to create zip archive", Err: err}
			}
		}
		return nil
	})
	if zipErr != nil {
		return zipErr
	}

	ticket, err := h.issueDownloadTicket(claims.Username, zipTempPath, zipFileName, "file.download-multiple-ticket", &spaceID, "application/zip", zipSize, true)
	if err != nil {
		os.Remove(zipTempPath) //nolint:errcheck
		h.recordSpaceAudit(r, audit.Event{
			Action: "file.download-multiple-ticket",
			Result: audit.ResultFailure,
			Target: fmt.Sprintf("%d items", len(req.Paths)),
			Metadata: map[string]any{
				"sourceCount": len(req.Paths),
				"filename":    zipFileName,
				"size":        zipSize,
				"format":      "application/zip",
				"status":      "failed",
				"reason":      "issue_ticket_failed",
			},
		}, spaceID)
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to issue download ticket", Err: err}
	}
	h.recordSpaceAudit(r, audit.Event{
		Action: "file.download-multiple-ticket",
		Result: audit.ResultSuccess,
		Target: fmt.Sprintf("%d items", len(req.Paths)),
		Metadata: map[string]any{
			"sourceCount": len(req.Paths),
			"filename":    ticket.FileName,
			"size":        zipSize,
			"format":      "application/zip",
			"status":      "ticket_issued",
		},
	}, spaceID)

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

func (h *Handler) downloadFolderAsZip(w http.ResponseWriter, folderPath string, folderName string) *web.Error {
	zipFileName := folderName + ".zip"
	return h.streamZipDownload(w, zipFileName, func(zipWriter *zip.Writer) *web.Error {
		return h.writeFolderToZip(folderPath, zipWriter)
	})
}

func (h *Handler) writeFolderToZip(folderPath string, zipWriter *zip.Writer) *web.Error {
	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		relPath, err := filepath.Rel(folderPath, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)
		header.Method = zip.Deflate

		if info.IsDir() {
			header.Name += "/"
			_, err = zipWriter.CreateHeader(header)
			return err
		}

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(writer, file)
		return err
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

func addToZip(zipWriter *zip.Writer, sourcePath string, baseName string) error {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return nil
			}
			relPath, err := filepath.Rel(sourcePath, path)
			if err != nil {
				return err
			}
			zipPath := filepath.Join(baseName, relPath)
			if relPath == "." {
				zipPath = baseName
			}

			header, err := zip.FileInfoHeader(info)
			if err != nil {
				return err
			}
			header.Name = filepath.ToSlash(zipPath)
			header.Method = zip.Deflate

			if info.IsDir() {
				if relPath != "." {
					header.Name += "/"
					if _, err := zipWriter.CreateHeader(header); err != nil {
						return err
					}
				}
				return nil
			}

			writer, err := zipWriter.CreateHeader(header)
			if err != nil {
				return err
			}
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = io.Copy(writer, file)
			return err
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
