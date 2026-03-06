package database

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

func TestMigrate_RemovesSpaceDescriptionColumnAndPreservesData(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	statements := []string{
		`CREATE TABLE users (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			username      TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			nickname      TEXT NOT NULL,
			role          TEXT NOT NULL DEFAULT 'user',
			created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE space (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			space_name      TEXT NOT NULL,
			space_desc      TEXT,
			space_path      TEXT NOT NULL,
			icon            TEXT,
			space_category  TEXT,
			quota_bytes     INTEGER,
			created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			created_user_id TEXT,
			updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_user_id TEXT
		)`,
		`CREATE TABLE trash_items (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			space_id      INTEGER NOT NULL,
			original_path TEXT NOT NULL,
			storage_path  TEXT NOT NULL,
			item_name     TEXT NOT NULL,
			is_dir        INTEGER NOT NULL DEFAULT 0,
			item_size     INTEGER NOT NULL DEFAULT 0,
			deleted_by    TEXT NOT NULL,
			deleted_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (space_id, storage_path),
			FOREIGN KEY (space_id) REFERENCES space(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE audit_logs (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			occurred_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			actor         TEXT NOT NULL,
			action        TEXT NOT NULL,
			result        TEXT NOT NULL,
			target        TEXT NOT NULL,
			request_id    TEXT NOT NULL,
			space_id      INTEGER,
			metadata_json TEXT NOT NULL DEFAULT '{}',
			FOREIGN KEY (space_id) REFERENCES space(id) ON DELETE SET NULL
		)`,
		`CREATE TABLE user_space_permissions (
			user_id     INTEGER NOT NULL,
			space_id    INTEGER NOT NULL,
			permission  TEXT NOT NULL,
			created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, space_id),
			CHECK (permission IN ('read', 'write', 'manage')),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (space_id) REFERENCES space(id) ON DELETE CASCADE
		)`,
		`INSERT INTO users(id, username, password_hash, nickname, role) VALUES (1, 'admin', 'hash', 'Admin', 'admin')`,
		`INSERT INTO space(id, space_name, space_desc, space_path, quota_bytes) VALUES (1, 'alpha', 'unused', '/tmp/alpha', 1048576)`,
		`INSERT INTO trash_items(id, space_id, original_path, storage_path, item_name, deleted_by) VALUES (1, 1, '/tmp/alpha/report.txt', '.trash/report.txt', 'report.txt', 'admin')`,
		`INSERT INTO audit_logs(id, actor, action, result, target, request_id, space_id, metadata_json) VALUES (1, 'admin', 'space.update', 'success', 'alpha', 'req-1', 1, '{}')`,
		`INSERT INTO user_space_permissions(user_id, space_id, permission) VALUES (1, 1, 'manage')`,
	}

	for _, stmt := range statements {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("setup statement failed: %v", err)
		}
	}

	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	hasDescriptionColumn, err := tableHasColumn(ctx, db, "space", "space_desc")
	if err != nil {
		t.Fatalf("check space_desc column: %v", err)
	}
	if hasDescriptionColumn {
		t.Fatal("expected space_desc column to be removed")
	}

	var (
		spaceName   string
		spacePath   string
		quotaBytes  int64
		trashCount  int
		auditCount  int
		accessCount int
	)

	if err := db.QueryRowContext(ctx, "SELECT space_name, space_path, quota_bytes FROM space WHERE id = 1").Scan(&spaceName, &spacePath, &quotaBytes); err != nil {
		t.Fatalf("query migrated space: %v", err)
	}
	if spaceName != "alpha" || spacePath != "/tmp/alpha" || quotaBytes != 1048576 {
		t.Fatalf("unexpected migrated space row: %q %q %d", spaceName, spacePath, quotaBytes)
	}

	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM trash_items WHERE space_id = 1").Scan(&trashCount); err != nil {
		t.Fatalf("query trash_items: %v", err)
	}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_logs WHERE space_id = 1").Scan(&auditCount); err != nil {
		t.Fatalf("query audit_logs: %v", err)
	}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_space_permissions WHERE space_id = 1").Scan(&accessCount); err != nil {
		t.Fatalf("query user_space_permissions: %v", err)
	}

	if trashCount != 1 || auditCount != 1 || accessCount != 1 {
		t.Fatalf("expected migrated related rows to be preserved, got trash=%d audit=%d permissions=%d", trashCount, auditCount, accessCount)
	}
}
