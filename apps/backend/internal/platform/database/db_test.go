package database

import (
	"os"
	"path/filepath"
	"testing"

	"taeu.kr/cohesion/internal/config"
)

func TestNewDB_ProductionDefaultCreatesDatabaseInStateRoot(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(config.ProductionStateRootEnv, stateRoot)

	config.SetConfig("production")

	db, err := NewDB()
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	expected := filepath.Join(stateRoot, "data", "cohesion.db")
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("expected db file at %q: %v", expected, err)
	}
}

func TestResolveSQLiteDBPath_ExpandsHomeShortcut(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	actual := resolveSQLiteDBPath("~/db/cohesion.db")
	expected := filepath.Join(homeDir, "db", "cohesion.db")
	if actual != expected {
		t.Fatalf("expected %q, got %q", expected, actual)
	}
}
