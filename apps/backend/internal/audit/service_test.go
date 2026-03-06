package audit_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"taeu.kr/cohesion/internal/audit"
	auditstore "taeu.kr/cohesion/internal/audit/store"
	"taeu.kr/cohesion/internal/platform/database"
)

func setupAuditService(t *testing.T) (*audit.Service, *auditstore.Store, *sql.DB) {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	store := auditstore.NewStore(db)
	svc := audit.NewService(store, audit.Config{BufferSize: 16})
	return svc, store, db
}

func TestService_RecordBestEffort_RedactsSensitiveMetadata(t *testing.T) {
	svc, _, db := setupAuditService(t)
	defer db.Close()

	now := time.Now().UTC().Add(-1 * time.Minute)
	svc.RecordBestEffort(audit.Event{
		OccurredAt: now,
		Actor:      "admin",
		Action:     "config.update",
		Result:     audit.ResultSuccess,
		Target:     "server",
		RequestID:  "req_test",
		Metadata: map[string]any{
			"before": map[string]any{
				"port":     "3000",
				"password": "should-not-be-stored",
			},
			"after": map[string]any{
				"port": "38080",
				"path": "/Users/secret/path",
			},
			"reason": "validation_failed",
			"token":  "should-not-be-stored",
		},
	})

	if err := svc.Close(context.Background()); err != nil {
		t.Fatalf("close service: %v", err)
	}

	res, err := svc.List(context.Background(), audit.ListFilter{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(res.Items))
	}

	item := res.Items[0]
	before, ok := item.Metadata["before"].(map[string]any)
	if !ok {
		t.Fatalf("expected before metadata map")
	}
	if _, exists := before["password"]; exists {
		t.Fatal("expected password to be redacted from metadata")
	}

	after, ok := item.Metadata["after"].(map[string]any)
	if !ok {
		t.Fatalf("expected after metadata map")
	}
	if got, _ := after["path"].(string); got != "[REDACTED_PATH]" {
		t.Fatalf("expected redacted absolute path, got %q", got)
	}

	if _, exists := item.Metadata["token"]; exists {
		t.Fatal("expected token to be dropped from metadata")
	}
	if reason, ok := item.Metadata["reason"].(string); !ok || reason != "validation_failed" {
		t.Fatalf("expected reason to be preserved, got %v", item.Metadata["reason"])
	}
}

func TestService_RecordBestEffort_DeniedMetadataUsesAllowlistAndMasking(t *testing.T) {
	svc, _, db := setupAuditService(t)
	defer db.Close()

	svc.RecordBestEffort(audit.Event{
		Actor:     "member",
		Action:    "file.download-ticket",
		Result:    audit.ResultDenied,
		Target:    "docs/report.zip",
		RequestID: "req_denied",
		Metadata: map[string]any{
			"reason": "permission_denied",
			"code":   "auth.permission_denied",
			"path":   "/Users/private/report.zip",
			"token":  "must-not-be-stored",
		},
	})

	if err := svc.Close(context.Background()); err != nil {
		t.Fatalf("close service: %v", err)
	}

	res, err := svc.List(context.Background(), audit.ListFilter{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(res.Items))
	}

	item := res.Items[0]
	if item.Result != audit.ResultDenied {
		t.Fatalf("expected result denied, got %s", item.Result)
	}
	if _, exists := item.Metadata["token"]; exists {
		t.Fatal("expected token to be removed from denied metadata")
	}
	if got, _ := item.Metadata["path"].(string); got != "[REDACTED_PATH]" {
		t.Fatalf("expected denied path to be redacted, got %q", got)
	}
	if got, _ := item.Metadata["reason"].(string); got != "permission_denied" {
		t.Fatalf("expected reason to be preserved, got %v", item.Metadata["reason"])
	}
	if got, _ := item.Metadata["code"].(string); got != "auth.permission_denied" {
		t.Fatalf("expected code to be preserved, got %v", item.Metadata["code"])
	}
}

type failingStore struct{}

func (f *failingStore) Create(_ context.Context, _ *audit.Event) error {
	return context.DeadlineExceeded
}

func (f *failingStore) List(_ context.Context, _ audit.ListFilter) ([]*audit.Log, int64, error) {
	return []*audit.Log{}, 0, nil
}

func (f *failingStore) GetByID(_ context.Context, _ int64) (*audit.Log, error) {
	return nil, sql.ErrNoRows
}

func TestService_RecordBestEffort_DoesNotPropagateStoreFailure(t *testing.T) {
	svc := audit.NewService(&failingStore{}, audit.Config{BufferSize: 1})

	svc.RecordBestEffort(audit.Event{
		Actor:     "tester",
		Action:    "file.delete",
		Result:    audit.ResultSuccess,
		Target:    "docs/a.txt",
		RequestID: "req_fail",
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := svc.Close(ctx); err != nil {
		t.Fatalf("close service: %v", err)
	}
}

func TestService_List_FilterAndPagination(t *testing.T) {
	svc, store, db := setupAuditService(t)
	defer db.Close()
	defer func() {
		_ = svc.Close(context.Background())
	}()

	base := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	spaceInsert, err := db.ExecContext(
		context.Background(),
		"INSERT INTO space(space_name, space_path) VALUES (?, ?)",
		"audit-test-space",
		"/tmp/audit-test-space",
	)
	if err != nil {
		t.Fatalf("insert space: %v", err)
	}
	spaceID, err := spaceInsert.LastInsertId()
	if err != nil {
		t.Fatalf("read space id: %v", err)
	}

	events := []audit.Event{
		{OccurredAt: base.Add(1 * time.Minute), Actor: "alice", Action: "file.upload", Result: audit.ResultSuccess, Target: "docs/a.txt", RequestID: "req_1", SpaceID: &spaceID, Metadata: map[string]any{"filename": "a.txt", "size": 1}},
		{OccurredAt: base.Add(2 * time.Minute), Actor: "alice", Action: "file.upload", Result: audit.ResultFailure, Target: "docs/b.txt", RequestID: "req_2", SpaceID: &spaceID, Metadata: map[string]any{"filename": "b.txt", "status": "failed"}},
		{OccurredAt: base.Add(3 * time.Minute), Actor: "bob", Action: "file.copy", Result: audit.ResultSuccess, Target: "docs/c.txt", RequestID: "req_3", SpaceID: &spaceID, Metadata: map[string]any{"sourceCount": 1}},
	}

	for _, event := range events {
		e := event
		if err := store.Create(context.Background(), &e); err != nil {
			t.Fatalf("create log: %v", err)
		}
	}

	from := base.Add(90 * time.Second)
	to := base.Add(4 * time.Minute)
	res, err := svc.List(context.Background(), audit.ListFilter{
		Page:     1,
		PageSize: 1,
		From:     &from,
		To:       &to,
		User:     "alice",
		Action:   "file.upload",
		SpaceID:  &spaceID,
		Result:   audit.ResultFailure,
	})
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}
	if res.Total != 1 {
		t.Fatalf("expected total=1, got %d", res.Total)
	}
	if len(res.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(res.Items))
	}
	if res.Items[0].RequestID != "req_2" {
		t.Fatalf("expected req_2, got %s", res.Items[0].RequestID)
	}
}
