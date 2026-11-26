package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
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
	dbPath := resolveSQLiteDBPath(config.Conf.Datasource.URL, config.ConfigDir())
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

// OpenExisting opens only the installation database. It skips migrations and
// leaves directory and file permissions as the server operator configured them.
func OpenExisting(ctx context.Context, datasourceURL, configDir string) (*sql.DB, string, error) {
	dbPath := resolveSQLiteDBPath(datasourceURL, configDir)
	if dbPath == "" || isInMemorySQLiteDB(dbPath) {
		return nil, "", errors.New("an existing on-disk database is required")
	}

	var uri *url.URL
	if strings.HasPrefix(dbPath, "file:") {
		parsed, err := url.Parse(dbPath)
		if err != nil {
			return nil, "", fmt.Errorf("invalid SQLite file URL: %w", err)
		}
		uri = parsed
	} else {
		absPath, err := filepath.Abs(dbPath)
		if err != nil {
			return nil, "", fmt.Errorf("resolve database path: %w", err)
		}
		dbPath = absPath
		info, err := os.Stat(dbPath)
		if err != nil {
			return nil, "", fmt.Errorf("inspect database: %w", err)
		}
		if !info.Mode().IsRegular() {
			return nil, "", fmt.Errorf("database %q is not a regular file", dbPath)
		}
		uriPath := filepath.ToSlash(dbPath)
		if filepath.VolumeName(dbPath) != "" {
			uriPath = "/" + uriPath
		}
		uri = &url.URL{Scheme: "file", Path: uriPath}
	}

	options, err := url.ParseQuery(uri.RawQuery)
	if err != nil {
		return nil, "", fmt.Errorf("invalid SQLite file URL options: %w", err)
	}
	if (uri.Opaque == "" && uri.Path == "") || options.Get("mode") == "memory" || options.Get("mode") == "ro" || options.Get("immutable") == "1" {
		return nil, "", errors.New("an existing writable on-disk database is required")
	}
	options.Set("mode", "rw")
	uri.RawQuery = options.Encode()

	db, err := sql.Open("sqlite3", uri.String())
	if err != nil {
		return nil, "", fmt.Errorf("open existing database %q: %w", dbPath, err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, "", fmt.Errorf("open existing database %q: %w", dbPath, err)
	}
	return db, dbPath, nil
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

func resolveSQLiteDBPath(dbPath, configDir string) string {
	dbPath = strings.TrimSpace(dbPath)
	if dbPath == "" || isInMemorySQLiteDB(dbPath) {
		return dbPath
	}
	if expandedPath, ok := config.ExpandHomePath(dbPath); ok {
		return expandedPath
	}
	if filepath.IsAbs(dbPath) || strings.HasPrefix(dbPath, "file:") {
		return dbPath
	}

	if configDir = strings.TrimSpace(configDir); configDir != "" {
		if config.IsDefaultProductionConfigDir(configDir) && config.IsLegacyProductionDatabaseURL(dbPath) {
			if productionDBPath, err := config.ResolveProductionDatabasePath(); err == nil && strings.TrimSpace(productionDBPath) != "" {
				return productionDBPath
			}
		}
		return filepath.Clean(filepath.Join(configDir, dbPath))
	}

	return dbPath
}
