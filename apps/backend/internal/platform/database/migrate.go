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
	rows, err := db.QueryContext(ctx, "PRAGMA table_info(space)")
	if err != nil {
		return err
	}
	defer rows.Close()

	hasQuotaColumn := false
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
			return err
		}
		if name == "quota_bytes" {
			hasQuotaColumn = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if hasQuotaColumn {
		return nil
	}

	_, err = db.ExecContext(ctx, "ALTER TABLE space ADD COLUMN quota_bytes INTEGER")
	return err
}
