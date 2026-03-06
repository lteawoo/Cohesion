package space_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"taeu.kr/cohesion/internal/platform/database"
	"taeu.kr/cohesion/internal/space"
	spaceStore "taeu.kr/cohesion/internal/space/store"
)

func setupSearchIndexManager(t *testing.T) (*space.SearchIndexManager, *space.Service, *sql.DB) {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	service := space.NewService(spaceStore.NewStore(db))
	manager := space.NewSearchIndexManager(service, spaceStore.NewSearchIndexStore(db))
	return manager, service, db
}

func insertSearchSpace(t *testing.T, db *sql.DB, name, root string) int64 {
	t.Helper()

	result, err := db.ExecContext(context.Background(), `INSERT INTO space (space_name, space_path) VALUES (?, ?)`, name, root)
	if err != nil {
		t.Fatalf("insert space: %v", err)
	}
	spaceID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("read inserted space id: %v", err)
	}
	return spaceID
}

func TestSearchIndexManager_BootstrapAndSearch(t *testing.T) {
	manager, _, db := setupSearchIndexManager(t)
	defer db.Close()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatalf("create docs dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "report.md"), []byte("a"), 0o644); err != nil {
		t.Fatalf("create report.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "report-plan.txt"), []byte("b"), 0o644); err != nil {
		t.Fatalf("create report-plan.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".report-hidden.txt"), []byte("hidden"), 0o644); err != nil {
		t.Fatalf("create hidden report file: %v", err)
	}

	spaceID := insertSearchSpace(t, db, "Design", root)

	if err := manager.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap index: %v", err)
	}

	results, err := manager.Search(context.Background(), []int64{spaceID}, "report")
	if err != nil {
		t.Fatalf("search index: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, item := range results {
		if item.Path == ".report-hidden.txt" {
			t.Fatal("hidden file must be excluded from indexed search results")
		}
	}
}

func TestSearchIndexManager_ReindexesDirtySpacesOnSearch(t *testing.T) {
	manager, _, db := setupSearchIndexManager(t)
	defer db.Close()

	root := t.TempDir()
	spaceID := insertSearchSpace(t, db, "Docs", root)

	if err := manager.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap index: %v", err)
	}

	initialResults, err := manager.Search(context.Background(), []int64{spaceID}, "report")
	if err != nil {
		t.Fatalf("initial search: %v", err)
	}
	if len(initialResults) != 0 {
		t.Fatalf("expected no initial results, got %d", len(initialResults))
	}

	if err := os.WriteFile(filepath.Join(root, "report-new.txt"), []byte("new"), 0o644); err != nil {
		t.Fatalf("create indexed file: %v", err)
	}

	staleResults, err := manager.Search(context.Background(), []int64{spaceID}, "report")
	if err != nil {
		t.Fatalf("stale search: %v", err)
	}
	if len(staleResults) != 0 {
		t.Fatalf("expected stale index to stay unchanged before dirty mark, got %d results", len(staleResults))
	}

	if err := manager.MarkSpaceDirty(context.Background(), spaceID); err != nil {
		t.Fatalf("mark space dirty: %v", err)
	}

	reindexedResults, err := manager.Search(context.Background(), []int64{spaceID}, "report")
	if err != nil {
		t.Fatalf("reindexed search: %v", err)
	}
	if len(reindexedResults) != 1 {
		t.Fatalf("expected 1 result after reindex, got %d", len(reindexedResults))
	}
	if reindexedResults[0].Path != "report-new.txt" {
		t.Fatalf("expected report-new.txt, got %q", reindexedResults[0].Path)
	}
}

func TestSearchIndexManager_SearchSkipsBrokenDirtySpace(t *testing.T) {
	manager, _, db := setupSearchIndexManager(t)
	defer db.Close()

	goodRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(goodRoot, "report-good.txt"), []byte("good"), 0o644); err != nil {
		t.Fatalf("create report-good.txt: %v", err)
	}
	goodSpaceID := insertSearchSpace(t, db, "Good", goodRoot)

	badRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(badRoot, "report-bad.txt"), []byte("bad"), 0o644); err != nil {
		t.Fatalf("create report-bad.txt: %v", err)
	}
	badSpaceID := insertSearchSpace(t, db, "Bad", badRoot)

	if err := manager.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap index: %v", err)
	}

	if err := os.RemoveAll(badRoot); err != nil {
		t.Fatalf("remove bad root: %v", err)
	}
	if err := manager.MarkSpaceDirty(context.Background(), badSpaceID); err != nil {
		t.Fatalf("mark bad space dirty: %v", err)
	}

	results, err := manager.Search(context.Background(), []int64{goodSpaceID, badSpaceID}, "report")
	if err != nil {
		t.Fatalf("search with broken dirty space: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected only clean-space result, got %d", len(results))
	}
	if results[0].SpaceID != goodSpaceID {
		t.Fatalf("expected result from good space %d, got %d", goodSpaceID, results[0].SpaceID)
	}

	var dirty int
	var lastError sql.NullString
	if err := db.QueryRowContext(
		context.Background(),
		`SELECT dirty, last_error FROM file_search_index_state WHERE space_id = ?`,
		badSpaceID,
	).Scan(&dirty, &lastError); err != nil {
		t.Fatalf("query bad space state: %v", err)
	}
	if dirty != 1 {
		t.Fatalf("expected bad space to remain dirty, got %d", dirty)
	}
	if !lastError.Valid || strings.TrimSpace(lastError.String) == "" {
		t.Fatal("expected last_error to be recorded for broken dirty space")
	}
}
