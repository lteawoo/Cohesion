package handler

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"taeu.kr/cohesion/internal/platform/database"
	"taeu.kr/cohesion/internal/space"
	spaceStore "taeu.kr/cohesion/internal/space/store"
)

func setupIndexedSearchHandler(t *testing.T) (*Handler, *space.SearchIndexManager, *sql.DB) {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	spaceService := space.NewService(spaceStore.NewStore(db))
	indexManager := space.NewSearchIndexManager(spaceService, spaceStore.NewSearchIndexStore(db))
	handler := NewHandler(spaceService, nil, &fakeSearchSpaceAccessService{})
	handler.SetSearchIndexer(indexManager)
	return handler, indexManager, db
}

func TestHandleSearchFiles_UsesIndexedResultsAndDirtyRecovery(t *testing.T) {
	handler, indexManager, db := setupIndexedSearchHandler(t)
	defer db.Close()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "report-a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("create report-a.txt: %v", err)
	}

	result, err := db.ExecContext(context.Background(), `INSERT INTO space (space_name, space_path) VALUES (?, ?)`, "Docs", root)
	if err != nil {
		t.Fatalf("insert space: %v", err)
	}
	spaceID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}

	handler.accountService = &fakeSearchSpaceAccessService{
		allowedBySpace: map[int64]bool{spaceID: true},
	}

	if err := indexManager.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap index: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/search/files?q=report&limit=10", nil)
	req = withClaims(req, "tester")
	rec := httptest.NewRecorder()

	webErr := handler.handleSearchFiles(rec, req)
	if webErr != nil {
		t.Fatalf("expected no error, got %+v", webErr)
	}
	got := decodeSearchResponse(t, rec)
	if len(got.Items) != 1 {
		t.Fatalf("expected 1 indexed result, got %d", len(got.Items))
	}

	if err := os.WriteFile(filepath.Join(root, "report-b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatalf("create report-b.txt: %v", err)
	}
	if err := indexManager.MarkSpaceDirty(context.Background(), spaceID); err != nil {
		t.Fatalf("mark dirty: %v", err)
	}

	rec = httptest.NewRecorder()
	webErr = handler.handleSearchFiles(rec, req)
	if webErr != nil {
		t.Fatalf("expected no error after dirty reindex, got %+v", webErr)
	}
	got = decodeSearchResponse(t, rec)
	if len(got.Items) != 2 {
		t.Fatalf("expected 2 indexed results after dirty reindex, got %d", len(got.Items))
	}
}

func TestHandleSearchFiles_SkipsBrokenDirtySpace(t *testing.T) {
	handler, indexManager, db := setupIndexedSearchHandler(t)
	defer db.Close()

	goodRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(goodRoot, "report-good.txt"), []byte("good"), 0o644); err != nil {
		t.Fatalf("create report-good.txt: %v", err)
	}
	goodResult, err := db.ExecContext(context.Background(), `INSERT INTO space (space_name, space_path) VALUES (?, ?)`, "Good", goodRoot)
	if err != nil {
		t.Fatalf("insert good space: %v", err)
	}
	goodSpaceID, err := goodResult.LastInsertId()
	if err != nil {
		t.Fatalf("good last insert id: %v", err)
	}

	badRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(badRoot, "report-bad.txt"), []byte("bad"), 0o644); err != nil {
		t.Fatalf("create report-bad.txt: %v", err)
	}
	badResult, err := db.ExecContext(context.Background(), `INSERT INTO space (space_name, space_path) VALUES (?, ?)`, "Bad", badRoot)
	if err != nil {
		t.Fatalf("insert bad space: %v", err)
	}
	badSpaceID, err := badResult.LastInsertId()
	if err != nil {
		t.Fatalf("bad last insert id: %v", err)
	}

	handler.accountService = &fakeSearchSpaceAccessService{
		allowedBySpace: map[int64]bool{
			goodSpaceID: true,
			badSpaceID:  true,
		},
	}

	if err := indexManager.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap index: %v", err)
	}

	if err := os.RemoveAll(badRoot); err != nil {
		t.Fatalf("remove bad root: %v", err)
	}
	if err := indexManager.MarkSpaceDirty(context.Background(), badSpaceID); err != nil {
		t.Fatalf("mark bad dirty: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/search/files?q=report&limit=10", nil)
	req = withClaims(req, "tester")
	rec := httptest.NewRecorder()

	webErr := handler.handleSearchFiles(rec, req)
	if webErr != nil {
		t.Fatalf("expected partial indexed search success, got %+v", webErr)
	}
	got := decodeSearchResponse(t, rec)
	if len(got.Items) != 1 {
		t.Fatalf("expected only one clean-space result, got %d", len(got.Items))
	}
	if got.Items[0].SpaceID != goodSpaceID {
		t.Fatalf("expected result from good space %d, got %d", goodSpaceID, got.Items[0].SpaceID)
	}
}
