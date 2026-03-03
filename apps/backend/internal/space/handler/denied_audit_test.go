package handler

import (
	"net/http"
	"testing"

	"taeu.kr/cohesion/internal/audit"
	"taeu.kr/cohesion/internal/auth"
	"taeu.kr/cohesion/internal/platform/web"
)

type recordingAuditSink struct {
	events []audit.Event
}

func (s *recordingAuditSink) RecordBestEffort(event audit.Event) {
	s.events = append(s.events, event)
}

func TestRecordDeniedFileActionAudit_RecordsMappedEvent(t *testing.T) {
	sink := &recordingAuditSink{}
	h := &Handler{auditRecorder: sink}

	req, err := http.NewRequest(http.MethodPost, "/api/spaces/12/files/download-multiple?path=docs/a.txt", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("X-Request-Id", "req_denied_file")
	req = req.WithContext(auth.WithClaims(req.Context(), &auth.Claims{Username: "member"}))

	nextReq := h.recordDeniedFileActionAudit(req, "download-multiple", 12, &web.Error{
		Code:    http.StatusForbidden,
		Message: "Access denied: invalid path",
	})

	if len(sink.events) != 1 {
		t.Fatalf("expected 1 denied audit event, got %d", len(sink.events))
	}
	event := sink.events[0]
	expectedAction, mapped := auth.DeniedAuditActionForSpaceFileAction("download-multiple")
	if !mapped {
		t.Fatal("expected download-multiple to be mapped")
	}
	if event.Action != expectedAction {
		t.Fatalf("expected action %s, got %s", expectedAction, event.Action)
	}
	if event.Result != audit.ResultDenied {
		t.Fatalf("expected denied result, got %s", event.Result)
	}
	if event.Actor != "member" {
		t.Fatalf("expected actor member, got %s", event.Actor)
	}
	if event.SpaceID == nil || *event.SpaceID != 12 {
		t.Fatalf("expected space id 12, got %v", event.SpaceID)
	}
	if got, _ := event.Metadata["code"].(string); got != "space.invalid_path" {
		t.Fatalf("expected code space.invalid_path, got %v", event.Metadata["code"])
	}
	if got, _ := event.Metadata["reason"].(string); got != "invalid_path" {
		t.Fatalf("expected reason invalid_path, got %v", event.Metadata["reason"])
	}
	if !auth.DeniedAuditRecorded(nextReq.Context()) {
		t.Fatal("expected request context to be marked as denied audit recorded")
	}
}

func TestRecordDeniedFileActionAudit_SkipsWhenAlreadyRecorded(t *testing.T) {
	sink := &recordingAuditSink{}
	h := &Handler{auditRecorder: sink}

	req, err := http.NewRequest(http.MethodPost, "/api/spaces/9/files/delete", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req = req.WithContext(auth.WithDeniedAuditRecorded(req.Context()))

	_ = h.recordDeniedFileActionAudit(req, "delete", 9, &web.Error{
		Code:    http.StatusForbidden,
		Message: "Permission denied",
	})

	if len(sink.events) != 0 {
		t.Fatalf("expected no additional denied audit event, got %d", len(sink.events))
	}
}

func TestRecordDeniedFileActionAudit_UsesSharedResolverForAction(t *testing.T) {
	tests := []string{"move", "download-multiple-ticket"}

	for _, action := range tests {
		action := action
		t.Run(action, func(t *testing.T) {
			sink := &recordingAuditSink{}
			h := &Handler{auditRecorder: sink}

			req, err := http.NewRequest(http.MethodPost, "/api/spaces/5/files/"+action+"?path=docs/a.txt", nil)
			if err != nil {
				t.Fatalf("new request: %v", err)
			}
			req = req.WithContext(auth.WithClaims(req.Context(), &auth.Claims{Username: "member"}))

			_ = h.recordDeniedFileActionAudit(req, action, 5, &web.Error{
				Code:    http.StatusForbidden,
				Message: "Permission denied",
			})

			if len(sink.events) != 1 {
				t.Fatalf("expected 1 denied audit event, got %d", len(sink.events))
			}
			expectedAction, mapped := auth.DeniedAuditActionForSpaceFileAction(action)
			if !mapped {
				t.Fatalf("expected %s to be mapped", action)
			}
			if sink.events[0].Action != expectedAction {
				t.Fatalf("expected action %s, got %s", expectedAction, sink.events[0].Action)
			}
		})
	}
}

func TestRecordDownloadTicketDeniedAudit_UsesTicketAction(t *testing.T) {
	sink := &recordingAuditSink{}
	h := &Handler{auditRecorder: sink}
	spaceID := int64(3)

	req, err := http.NewRequest(http.MethodGet, "/api/downloads/token-1", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("X-Request-Id", "req_ticket_denied")
	req = req.WithContext(auth.WithClaims(req.Context(), &auth.Claims{Username: "member"}))

	nextReq := h.recordDownloadTicketDeniedAudit(req, &downloadTicket{
		Action:   "file.download-multiple-ticket",
		FileName: "bundle.zip",
		SpaceID:  &spaceID,
	})

	if len(sink.events) != 1 {
		t.Fatalf("expected 1 denied audit event, got %d", len(sink.events))
	}
	event := sink.events[0]
	if event.Action != "file.download-multiple-ticket" {
		t.Fatalf("expected action file.download-multiple-ticket, got %s", event.Action)
	}
	if event.Result != audit.ResultDenied {
		t.Fatalf("expected denied result, got %s", event.Result)
	}
	if event.SpaceID == nil || *event.SpaceID != 3 {
		t.Fatalf("expected space id 3, got %v", event.SpaceID)
	}
	if event.Target != "bundle.zip" {
		t.Fatalf("expected target bundle.zip, got %s", event.Target)
	}
	if !auth.DeniedAuditRecorded(nextReq.Context()) {
		t.Fatal("expected request context to be marked as denied audit recorded")
	}
}
