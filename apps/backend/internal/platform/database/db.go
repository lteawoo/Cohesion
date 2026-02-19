package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"taeu.kr/cohesion/internal/config"
)

const (
	dbDirPermission  = 0700
	dbFilePermission = 0600
)

func NewDB() (*sql.DB, error) {
	dbPath := resolveSQLiteDBPath(strings.TrimSpace(config.Conf.Datasource.URL))
	if dbPath == "" {
		return nil, errors.New("database url is required")
	}

	if !isInMemorySQLiteDB(dbPath) {
		// Ensure the directory and database file exist with owner-only permissions.
		if err := ensureDatabasePath(dbPath); err != nil {
			return nil, err
		}
	}

	// Open the database connection
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set a timeout context for initial operations
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Verify the connection
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Run migrations (Schema setup.. etc)
	if err := Migrate(ctx, db); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	if !isInMemorySQLiteDB(dbPath) {
		if err := enforceOwnerOnlyPermissions(dbPath, dbFilePermission); err != nil {
			return nil, fmt.Errorf("failed to enforce database file permission: %w", err)
		}
	}

	return db, nil
}

func ensureDatabasePath(dbPath string) error {
	if dir := filepath.Dir(dbPath); dir != "." {
		if err := os.MkdirAll(dir, dbDirPermission); err != nil {
			return fmt.Errorf("failed to create data directory: %w", err)
		}
		if err := enforceOwnerOnlyPermissions(dir, dbDirPermission); err != nil {
			return fmt.Errorf("failed to enforce data directory permission: %w", err)
		}
	}

	if _, err := os.Stat(dbPath); errors.Is(err, os.ErrNotExist) {
		file, createErr := os.OpenFile(dbPath, os.O_CREATE|os.O_EXCL|os.O_RDWR, dbFilePermission)
		if createErr != nil && !errors.Is(createErr, os.ErrExist) {
			return fmt.Errorf("failed to create database file: %w", createErr)
		}
		if createErr == nil {
			_ = file.Close()
		}
	} else if err != nil {
		return fmt.Errorf("failed to inspect database file: %w", err)
	}

	if err := enforceOwnerOnlyPermissions(dbPath, dbFilePermission); err != nil {
		return fmt.Errorf("failed to enforce database file permission: %w", err)
	}

	return nil
}

func enforceOwnerOnlyPermissions(path string, perm os.FileMode) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	return os.Chmod(path, perm)
}

func isInMemorySQLiteDB(dbPath string) bool {
	return dbPath == ":memory:" || strings.HasPrefix(dbPath, "file::memory:")
}

func resolveSQLiteDBPath(dbPath string) string {
	if dbPath == "" || isInMemorySQLiteDB(dbPath) {
		return dbPath
	}
	if filepath.IsAbs(dbPath) || strings.HasPrefix(dbPath, "file:") {
		return dbPath
	}

	if configDir := strings.TrimSpace(config.ConfigDir()); configDir != "" {
		return filepath.Clean(filepath.Join(configDir, dbPath))
	}

	return dbPath
}
