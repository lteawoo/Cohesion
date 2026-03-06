package audit_test

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"taeu.kr/cohesion/internal/audit"
	auditstore "taeu.kr/cohesion/internal/audit/store"
)

func newAuditHandlerForTest(t *testing.T, retentionDays int) (*audit.Service, *auditstore.Store, *audit.Handler, func()) {
	t.Helper()

	svc, store, db := setupAuditService(t)
	handler := audit.NewHandler(svc)
	handler.SetRetentionDaysProvider(func() int { return retentionDays })
	handler.SetActorResolver(func(*http.Request) string { return "admin" })

	cleanup := func() {
		_ = svc.Close(context.Background())
		_ = db.Close()
	}

	return svc, store, handler, cleanup
}

func TestHandleListLogs_IncludesRetentionDays(t *testing.T) {
	svc, store, handler, cleanup := newAuditHandlerForTest(t, 30)
	defer cleanup()

	if err := store.Create(context.Background(), &audit.Event{
		OccurredAt: time.Now().UTC(),
		Actor:      "admin",
		Action:     "file.delete",
		Result:     audit.ResultSuccess,
		Target:     "docs/report.txt",
		RequestID:  "req_list",
	}); err != nil {
		t.Fatalf("seed audit log: %v", err)
	}

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodGet, "/api/audit/logs", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var response audit.ListResult
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.RetentionDays != 30 {
		t.Fatalf("expected retention days 30, got %d", response.RetentionDays)
	}
	if len(response.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(response.Items))
	}
	_ = svc
}

func TestHandleExportLogs_WritesFilteredCSV(t *testing.T) {
	_, store, handler, cleanup := newAuditHandlerForTest(t, 30)
	defer cleanup()

	for _, event := range []audit.Event{
		{OccurredAt: time.Now().UTC(), Actor: "alice", Action: "file.delete", Result: audit.ResultFailure, Target: "docs/a.txt", RequestID: "req_a"},
		{OccurredAt: time.Now().UTC(), Actor: "bob", Action: "file.copy", Result: audit.ResultSuccess, Target: "docs/b.txt", RequestID: "req_b"},
	} {
		e := event
		if err := store.Create(context.Background(), &e); err != nil {
			t.Fatalf("seed audit log: %v", err)
		}
	}

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodGet, "/api/audit/logs/export?user=alice", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "text/csv") {
		t.Fatalf("expected csv content type, got %q", contentType)
	}

	rows, err := csv.NewReader(strings.NewReader(rec.Body.String())).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 csv rows, got %d", len(rows))
	}
	if rows[1][2] != "alice" {
		t.Fatalf("expected exported actor alice, got %q", rows[1][2])
	}
	if strings.Contains(rec.Body.String(), "bob") {
		t.Fatalf("expected filtered export body, got %s", rec.Body.String())
	}
}

func TestHandleCleanupLogs_DeletesOldLogsAndRecordsAuditEvent(t *testing.T) {
	svc, store, handler, cleanup := newAuditHandlerForTest(t, 30)
	defer cleanup()

	for _, event := range []audit.Event{
		{OccurredAt: time.Now().UTC().AddDate(0, 0, -40), Actor: "admin", Action: "file.delete", Result: audit.ResultSuccess, Target: "docs/old.txt", RequestID: "req_old"},
		{OccurredAt: time.Now().UTC().AddDate(0, 0, -5), Actor: "admin", Action: "file.copy", Result: audit.ResultSuccess, Target: "docs/new.txt", RequestID: "req_new"},
	} {
		e := event
		if err := store.Create(context.Background(), &e); err != nil {
			t.Fatalf("seed audit log: %v", err)
		}
	}

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/api/audit/logs/cleanup", nil)
	req.Header.Set("X-Request-Id", "req_cleanup")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var response audit.CleanupResult
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.DeletedCount != 1 {
		t.Fatalf("expected deleted count 1, got %d", response.DeletedCount)
	}
	if response.RetentionDays != 30 {
		t.Fatalf("expected retention days 30, got %d", response.RetentionDays)
	}

	if err := svc.Close(context.Background()); err != nil {
		t.Fatalf("close service after cleanup: %v", err)
	}

	res, err := svc.List(context.Background(), audit.ListFilter{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list logs after cleanup: %v", err)
	}
	if res.Total != 2 {
		t.Fatalf("expected 2 logs after cleanup (recent + cleanup event), got %d", res.Total)
	}

	foundCleanupEvent := false
	for _, item := range res.Items {
		if item.Action != "audit.logs.cleanup" {
			continue
		}
		foundCleanupEvent = true
		if item.Actor != "admin" {
			t.Fatalf("expected cleanup actor admin, got %q", item.Actor)
		}
		if item.Result != audit.ResultSuccess {
			t.Fatalf("expected cleanup result success, got %q", item.Result)
		}
		if got := item.Metadata["retentionDays"]; got != float64(30) && got != 30 {
			t.Fatalf("expected retentionDays metadata, got %v", got)
		}
		if got := item.Metadata["deletedCount"]; got != float64(1) && got != int64(1) && got != 1 {
			t.Fatalf("expected deletedCount metadata, got %v", got)
		}
	}
	if !foundCleanupEvent {
		t.Fatal("expected cleanup audit event to be recorded")
	}
}

func TestHandleCleanupLogs_RejectsDisabledRetentionPolicy(t *testing.T) {
	svc, _, handler, cleanup := newAuditHandlerForTest(t, 0)
	defer cleanup()
	_ = svc

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/audit/logs/cleanup", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}
