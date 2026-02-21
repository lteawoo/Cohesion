package handler

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"taeu.kr/cohesion/internal/auth"
	"taeu.kr/cohesion/internal/platform/web"
)

type downloadTicket struct {
	Token          string
	Owner          string
	FilePath       string
	FileName       string
	ContentType    string
	ContentSize    int64
	RemoveAfterUse bool
	ExpiresAt      time.Time
}

func generateDownloadTicketToken() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func (h *Handler) issueDownloadTicket(
	owner string,
	filePath string,
	fileName string,
	contentType string,
	contentSize int64,
	removeAfterUse bool,
) (*downloadTicket, error) {
	token, err := generateDownloadTicketToken()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	ticket := downloadTicket{
		Token:          token,
		Owner:          owner,
		FilePath:       filePath,
		FileName:       fileName,
		ContentType:    contentType,
		ContentSize:    contentSize,
		RemoveAfterUse: removeAfterUse,
		ExpiresAt:      now.Add(h.downloadTicketTTL),
	}

	h.ticketMu.Lock()
	defer h.ticketMu.Unlock()
	h.cleanupExpiredDownloadTicketsLocked(now)
	h.downloadTickets[token] = ticket
	return &ticket, nil
}

func (h *Handler) cleanupExpiredDownloadTicketsLocked(now time.Time) {
	for token, ticket := range h.downloadTickets {
		if ticket.ExpiresAt.After(now) {
			continue
		}
		delete(h.downloadTickets, token)
		if ticket.RemoveAfterUse && ticket.FilePath != "" {
			_ = os.Remove(ticket.FilePath)
		}
	}
}

func (h *Handler) consumeDownloadTicketForOwner(token string, owner string) (*downloadTicket, *web.Error) {
	h.ticketMu.Lock()
	defer h.ticketMu.Unlock()

	now := time.Now()
	h.cleanupExpiredDownloadTicketsLocked(now)

	ticket, ok := h.downloadTickets[token]
	if !ok {
		return nil, &web.Error{Code: http.StatusNotFound, Message: "Download ticket not found"}
	}
	if ticket.Owner != owner {
		return nil, &web.Error{Code: http.StatusForbidden, Message: "Download ticket access denied"}
	}

	delete(h.downloadTickets, token)
	return &ticket, nil
}

func (h *Handler) handleDownloadByTicket(w http.ResponseWriter, r *http.Request) *web.Error {
	if r.Method != http.MethodGet {
		return &web.Error{Code: http.StatusMethodNotAllowed, Message: "Method not allowed"}
	}

	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		return &web.Error{Code: http.StatusUnauthorized, Message: "Unauthorized"}
	}

	token := strings.TrimPrefix(r.URL.Path, "/api/downloads/")
	if token == "" || strings.Contains(token, "/") {
		return &web.Error{Code: http.StatusBadRequest, Message: "Invalid download ticket"}
	}

	ticket, webErr := h.consumeDownloadTicketForOwner(token, claims.Username)
	if webErr != nil {
		return webErr
	}
	if ticket.RemoveAfterUse {
		defer func() {
			_ = os.Remove(ticket.FilePath)
		}()
	}

	file, err := os.Open(ticket.FilePath)
	if err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to open download file", Err: err}
	}
	defer file.Close()

	contentType := ticket.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, ticket.FileName))
	if ticket.ContentSize > 0 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", ticket.ContentSize))
	}

	if _, err := io.Copy(w, file); err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to send download file", Err: err}
	}
	return nil
}
