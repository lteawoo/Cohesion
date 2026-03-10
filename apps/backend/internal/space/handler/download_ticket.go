package handler

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"os"
	"strings"
	"time"

	"taeu.kr/cohesion/internal/audit"
	"taeu.kr/cohesion/internal/auth"
	"taeu.kr/cohesion/internal/platform/web"
)

type downloadTicket struct {
	Token          string
	Owner          string
	FilePath       string
	FileName       string
	Action         string
	SpaceID        *int64
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
	action string,
	spaceID *int64,
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
		Action:         strings.TrimSpace(action),
		SpaceID:        spaceID,
		ContentType:    contentType,
		ContentSize:    contentSize,
		RemoveAfterUse: removeAfterUse,
		ExpiresAt:      now.Add(h.downloadTicketTTL),
	}
	if ticket.Action == "" {
		ticket.Action = "file.download-ticket"
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

func (h *Handler) getDownloadTicketForOwner(token string, owner string) (*downloadTicket, *web.Error) {
	h.ticketMu.Lock()
	defer h.ticketMu.Unlock()

	now := time.Now()
	h.cleanupExpiredDownloadTicketsLocked(now)

	ticket, ok := h.downloadTickets[token]
	if !ok {
		return nil, &web.Error{Code: http.StatusNotFound, Message: "Download ticket not found"}
	}
	if ticket.Owner != owner {
		ticketCopy := ticket
		return &ticketCopy, &web.Error{Code: http.StatusForbidden, Message: "Download ticket access denied"}
	}

	ticketCopy := ticket
	return &ticketCopy, nil
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

	ticket, webErr := h.getDownloadTicketForOwner(token, claims.Username)
	if webErr != nil {
		if webErr.Code == http.StatusForbidden {
			r = h.recordDownloadTicketDeniedAudit(r, ticket)
		}
		return webErr
	}
	file, err := os.Open(ticket.FilePath)
	if err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to open download file", Err: err}
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to inspect download file", Err: err}
	}

	serveAttachmentContent(w, r, file, fileInfo, ticket.FileName, ticket.ContentType)
	return nil
}

func (h *Handler) recordDownloadTicketDeniedAudit(r *http.Request, ticket *downloadTicket) *http.Request {
	if h.auditRecorder == nil || auth.DeniedAuditRecorded(r.Context()) {
		return r
	}

	action := "file.download-ticket"
	target := "download-ticket"
	var spaceID *int64
	if ticket != nil {
		if strings.TrimSpace(ticket.Action) != "" {
			action = strings.TrimSpace(ticket.Action)
		}
		if strings.TrimSpace(ticket.FileName) != "" {
			target = strings.TrimSpace(ticket.FileName)
		}
		spaceID = ticket.SpaceID
	}

	event := audit.Event{
		Action:    action,
		Result:    audit.ResultDenied,
		Target:    target,
		RequestID: strings.TrimSpace(r.Header.Get("X-Request-Id")),
		SpaceID:   spaceID,
		Metadata: map[string]any{
			"reason": "ticket_access_denied",
			"code":   "download.ticket_access_denied",
			"status": http.StatusForbidden,
		},
	}
	if claims, ok := auth.ClaimsFromContext(r.Context()); ok {
		event.Actor = claims.Username
	}

	h.auditRecorder.RecordBestEffort(event)
	return r.WithContext(auth.WithDeniedAuditRecorded(r.Context()))
}
