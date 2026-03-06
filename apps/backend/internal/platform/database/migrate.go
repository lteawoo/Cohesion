package database

import (
	"context"
	"database/sql"
	_ "embed"
	"strings"
)

//go:embed queries/schema.sql
var schemaDDL string

func Migrate(ctx context.Context, db *sql.DB) error {
	if err := migrateUsersRoleConstraint(ctx, db); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, schemaDDL); err != nil {
		return err
	}
	if err := migrateSpaceQuotaColumn(ctx, db); err != nil {
		return err
	}
	if err := migrateSpaceDescriptionColumn(ctx, db); err != nil {
		return err
	}
	return nil
}

func migrateUsersRoleConstraint(ctx context.Context, db *sql.DB) error {
	var schemaSQL string
	if err := db.QueryRowContext(ctx, "SELECT sql FROM sqlite_master WHERE type='table' AND name='users'").Scan(&schemaSQL); err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}

	if !strings.Contains(schemaSQL, "CHECK (role IN ('admin', 'user'))") {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	statements := []string{
		"PRAGMA foreign_keys = OFF",
		"ALTER TABLE users RENAME TO users_legacy",
		`CREATE TABLE users (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			username      TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			nickname      TEXT NOT NULL,
			role          TEXT NOT NULL DEFAULT 'user',
			created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		"INSERT INTO users(id, username, password_hash, nickname, role, created_at, updated_at) SELECT id, username, password_hash, nickname, role, created_at, updated_at FROM users_legacy",
		"DROP TABLE users_legacy",
		"ALTER TABLE user_space_permissions RENAME TO user_space_permissions_legacy",
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
		"INSERT INTO user_space_permissions(user_id, space_id, permission, created_at, updated_at) SELECT user_id, space_id, permission, created_at, updated_at FROM user_space_permissions_legacy",
		"DROP TABLE user_space_permissions_legacy",
		"PRAGMA foreign_keys = ON",
	}

	for _, stmt := range statements {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func migrateSpaceQuotaColumn(ctx context.Context, db *sql.DB) error {
	hasQuotaColumn, err := tableHasColumn(ctx, db, "space", "quota_bytes")
	if err != nil {
		return err
	}
	if hasQuotaColumn {
		return nil
	}

	_, err = db.ExecContext(ctx, "ALTER TABLE space ADD COLUMN quota_bytes INTEGER")
	return err
}

func migrateSpaceDescriptionColumn(ctx context.Context, db *sql.DB) error {
	hasDescriptionColumn, err := tableHasColumn(ctx, db, "space", "space_desc")
	if err != nil {
		return err
	}
	if !hasDescriptionColumn {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	statements := []string{
		"PRAGMA foreign_keys = OFF",
		"ALTER TABLE trash_items RENAME TO trash_items_legacy",
		"ALTER TABLE audit_logs RENAME TO audit_logs_legacy",
		"ALTER TABLE user_space_permissions RENAME TO user_space_permissions_legacy",
		"ALTER TABLE space RENAME TO space_legacy",
		"DROP INDEX IF EXISTS idx_trash_items_space_deleted_at",
		"DROP INDEX IF EXISTS idx_audit_logs_occurred_id",
		"DROP INDEX IF EXISTS idx_audit_logs_filters",
		`CREATE TABLE space (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			space_name      TEXT NOT NULL,
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
		`CREATE INDEX idx_trash_items_space_deleted_at
			ON trash_items(space_id, deleted_at DESC)`,
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
		`CREATE INDEX idx_audit_logs_occurred_id
			ON audit_logs(occurred_at DESC, id DESC)`,
		`CREATE INDEX idx_audit_logs_filters
			ON audit_logs(actor, action, result, space_id)`,
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
		`INSERT INTO space(id, space_name, space_path, icon, space_category, quota_bytes, created_at, created_user_id, updated_at, updated_user_id)
			SELECT id, space_name, space_path, icon, space_category, quota_bytes, created_at, created_user_id, updated_at, updated_user_id
			FROM space_legacy`,
		`INSERT INTO trash_items(id, space_id, original_path, storage_path, item_name, is_dir, item_size, deleted_by, deleted_at)
			SELECT id, space_id, original_path, storage_path, item_name, is_dir, item_size, deleted_by, deleted_at
			FROM trash_items_legacy`,
		`INSERT INTO audit_logs(id, occurred_at, actor, action, result, target, request_id, space_id, metadata_json)
			SELECT id, occurred_at, actor, action, result, target, request_id, space_id, metadata_json
			FROM audit_logs_legacy`,
		`INSERT INTO user_space_permissions(user_id, space_id, permission, created_at, updated_at)
			SELECT user_id, space_id, permission, created_at, updated_at
			FROM user_space_permissions_legacy`,
		"DROP TABLE space_legacy",
		"DROP TABLE trash_items_legacy",
		"DROP TABLE audit_logs_legacy",
		"DROP TABLE user_space_permissions_legacy",
		"PRAGMA foreign_keys = ON",
	}

	for _, stmt := range statements {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func tableHasColumn(ctx context.Context, db *sql.DB, tableName string, columnName string) (bool, error) {
	rows, err := db.QueryContext(ctx, "PRAGMA table_info("+tableName+")")
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultVal sql.NullString
			pk         int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &pk); err != nil {
			return false, err
		}
		if name == columnName {
			return true, nil
		}
	}

	if err := rows.Err(); err != nil {
		return false, err
	}
	return false, nil
}
