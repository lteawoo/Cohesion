package database

import (
	"context"
	"database/sql"
	_ "embed"
)

//go:embed queries/schema.sql
var schemaDDL string

func Migrate(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, schemaDDL)
	return err
}
