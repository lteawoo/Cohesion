package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"taeu.kr/cohesion/internal/auth"
	"taeu.kr/cohesion/internal/space"
)

type fakeTrashStore struct {
	mu     sync.Mutex
	nextID int64
	items  map[int64]*space.TrashItem
}

func newFakeTrashStore() *fakeTrashStore {
	return &fakeTrashStore{
		nextID: 1,
		items:  make(map[int64]*space.TrashItem),
	}
}

func (f *fakeTrashStore) CreateTrashItem(_ context.Context, req *space.CreateTrashItemRequest) (*space.TrashItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	item := &space.TrashItem{
		ID:           f.nextID,
		SpaceID:      req.SpaceID,
		OriginalPath: req.OriginalPath,
		StoragePath:  req.StoragePath,
		ItemName:     req.ItemName,
		IsDir:        req.IsDir,
		ItemSize:     req.ItemSize,
		DeletedBy:    req.DeletedBy,
		DeletedAt:    time.Now(),
	}
	f.nextID++
	f.items[item.ID] = item
	return item, nil
}

func (f *fakeTrashStore) ListTrashItemsBySpace(_ context.Context, spaceID int64) ([]*space.TrashItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	items := make([]*space.TrashItem, 0)
	for _, item := range f.items {
		if item.SpaceID != spaceID {
			continue
		}
		copied := *item
		items = append(items, &copied)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].DeletedAt.After(items[j].DeletedAt)
	})
	return items, nil
}

func (f *fakeTrashStore) GetTrashItemByID(_ context.Context, id int64) (*space.TrashItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	item, ok := f.items[id]
	if !ok {
		return nil, errors.New("not found")
	}
	copied := *item
	return &copied, nil
}

func (f *fakeTrashStore) DeleteTrashItemByID(_ context.Context, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.items, id)
	return nil
}

func (f *fakeTrashStore) DeleteTrashItemsBySpace(_ context.Context, spaceID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for id, item := range f.items {
		if item.SpaceID == spaceID {
			delete(f.items, id)
		}
	}
	return nil
}

func newJSONRequestWithClaims(t *testing.T, method, target string, payload interface{}) *http.Request {
	t.Helper()

	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			t.Fatalf("failed to encode payload: %v", err)
		}
	}

	req := httptest.NewRequest(method, target, &body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(auth.WithClaims(req.Context(), &auth.Claims{Username: "tester"}))
	return req
}

func decodeJSONBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return payload
}

func setupTrashHandler(t *testing.T) (*Handler, string) {
	t.Helper()

	spaceRoot := t.TempDir()
	store := &fakeUploadSpaceStore{
		spacesByID: map[int64]*space.Space{
			1: {
				ID:        1,
				SpaceName: "Trash",
				SpacePath: spaceRoot,
			},
		},
	}
	trashStore := newFakeTrashStore()
	handler := NewHandler(space.NewService(store), nil, nil, space.NewTrashService(trashStore))
	return handler, spaceRoot
}

func TestTrash_DeleteAndRestore(t *testing.T) {
	handler, root := setupTrashHandler(t)

	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	deleteReq := newJSONRequestWithClaims(t, http.MethodPost, "/api/spaces/1/files/delete", map[string]string{
		"path": "note.txt",
	})
	deleteRec := httptest.NewRecorder()
	if webErr := handler.handleFileDelete(deleteRec, deleteReq, 1); webErr != nil {
		t.Fatalf("delete failed: %+v", webErr)
	}

	if _, err := os.Stat(filepath.Join(root, "note.txt")); !os.IsNotExist(err) {
		t.Fatalf("source file should be moved into trash, err=%v", err)
	}

	listReq := newJSONRequestWithClaims(t, http.MethodGet, "/api/spaces/1/files/trash", nil)
	listRec := httptest.NewRecorder()
	if webErr := handler.handleTrashList(listRec, listReq, 1); webErr != nil {
		t.Fatalf("trash list failed: %+v", webErr)
	}
	listPayload := decodeJSONBody(t, listRec)
	items := listPayload["items"].([]interface{})
	if len(items) != 1 {
		t.Fatalf("expected one trash item, got %d", len(items))
	}
	trashItem := items[0].(map[string]interface{})
	trashID := int64(trashItem["id"].(float64))

	restoreReq := newJSONRequestWithClaims(t, http.MethodPost, "/api/spaces/1/files/trash-restore", map[string]interface{}{
		"ids": []int64{trashID},
	})
	restoreRec := httptest.NewRecorder()
	if webErr := handler.handleTrashRestore(restoreRec, restoreReq, 1); webErr != nil {
		t.Fatalf("trash restore failed: %+v", webErr)
	}

	content, err := os.ReadFile(filepath.Join(root, "note.txt"))
	if err != nil {
		t.Fatalf("restored file should exist: %v", err)
	}
	if string(content) != "hello" {
		t.Fatalf("unexpected restored file content: %q", string(content))
	}

	listRecAfterRestore := httptest.NewRecorder()
	if webErr := handler.handleTrashList(listRecAfterRestore, listReq, 1); webErr != nil {
		t.Fatalf("trash list after restore failed: %+v", webErr)
	}
	listPayloadAfterRestore := decodeJSONBody(t, listRecAfterRestore)
	itemsAfterRestore := listPayloadAfterRestore["items"].([]interface{})
	if len(itemsAfterRestore) != 0 {
		t.Fatalf("expected empty trash after restore, got %d items", len(itemsAfterRestore))
	}
}

func TestTrash_RestoreConflictPolicy(t *testing.T) {
	handler, root := setupTrashHandler(t)

	if err := os.WriteFile(filepath.Join(root, "conflict.txt"), []byte("from-trash"), 0o644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}
	deleteReq := newJSONRequestWithClaims(t, http.MethodPost, "/api/spaces/1/files/delete", map[string]string{
		"path": "conflict.txt",
	})
	deleteRec := httptest.NewRecorder()
	if webErr := handler.handleFileDelete(deleteRec, deleteReq, 1); webErr != nil {
		t.Fatalf("delete failed: %+v", webErr)
	}

	if err := os.WriteFile(filepath.Join(root, "conflict.txt"), []byte("existing"), 0o644); err != nil {
		t.Fatalf("failed to create destination conflict file: %v", err)
	}

	listReq := newJSONRequestWithClaims(t, http.MethodGet, "/api/spaces/1/files/trash", nil)
	listRec := httptest.NewRecorder()
	if webErr := handler.handleTrashList(listRec, listReq, 1); webErr != nil {
		t.Fatalf("trash list failed: %+v", webErr)
	}
	listPayload := decodeJSONBody(t, listRec)
	items := listPayload["items"].([]interface{})
	if len(items) != 1 {
		t.Fatalf("expected one trash item, got %d", len(items))
	}
	trashID := int64(items[0].(map[string]interface{})["id"].(float64))

	restoreWithoutPolicyReq := newJSONRequestWithClaims(t, http.MethodPost, "/api/spaces/1/files/trash-restore", map[string]interface{}{
		"ids": []int64{trashID},
	})
	restoreWithoutPolicyRec := httptest.NewRecorder()
	if webErr := handler.handleTrashRestore(restoreWithoutPolicyRec, restoreWithoutPolicyReq, 1); webErr != nil {
		t.Fatalf("restore request should succeed with failed payload, got %+v", webErr)
	}
	restoreWithoutPolicyPayload := decodeJSONBody(t, restoreWithoutPolicyRec)
	failed := restoreWithoutPolicyPayload["failed"].([]interface{})
	if len(failed) != 1 {
		t.Fatalf("expected one failed restore without policy, got %d", len(failed))
	}
	if failed[0].(map[string]interface{})["code"] != fileConflictCodeDestinationExists {
		t.Fatalf("expected destination_exists code, got %v", failed[0])
	}

	restoreOverwriteReq := newJSONRequestWithClaims(t, http.MethodPost, "/api/spaces/1/files/trash-restore", map[string]interface{}{
		"ids":            []int64{trashID},
		"conflictPolicy": "overwrite",
	})
	restoreOverwriteRec := httptest.NewRecorder()
	if webErr := handler.handleTrashRestore(restoreOverwriteRec, restoreOverwriteReq, 1); webErr != nil {
		t.Fatalf("restore overwrite failed: %+v", webErr)
	}
	content, err := os.ReadFile(filepath.Join(root, "conflict.txt"))
	if err != nil {
		t.Fatalf("restored file should exist after overwrite: %v", err)
	}
	if string(content) != "from-trash" {
		t.Fatalf("expected overwrite restore content from trash, got %q", string(content))
	}
}

func TestTrash_DeletePermanentAndEmpty(t *testing.T) {
	handler, root := setupTrashHandler(t)

	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("A"), 0o644); err != nil {
		t.Fatalf("failed to create file a: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("B"), 0o644); err != nil {
		t.Fatalf("failed to create file b: %v", err)
	}

	for _, path := range []string{"a.txt", "b.txt"} {
		deleteReq := newJSONRequestWithClaims(t, http.MethodPost, "/api/spaces/1/files/delete", map[string]string{
			"path": path,
		})
		deleteRec := httptest.NewRecorder()
		if webErr := handler.handleFileDelete(deleteRec, deleteReq, 1); webErr != nil {
			t.Fatalf("delete %s failed: %+v", path, webErr)
		}
	}

	listReq := newJSONRequestWithClaims(t, http.MethodGet, "/api/spaces/1/files/trash", nil)
	listRec := httptest.NewRecorder()
	if webErr := handler.handleTrashList(listRec, listReq, 1); webErr != nil {
		t.Fatalf("trash list failed: %+v", webErr)
	}
	listPayload := decodeJSONBody(t, listRec)
	items := listPayload["items"].([]interface{})
	if len(items) != 2 {
		t.Fatalf("expected two trash items, got %d", len(items))
	}

	firstID := int64(items[0].(map[string]interface{})["id"].(float64))
	deleteTrashReq := newJSONRequestWithClaims(t, http.MethodPost, "/api/spaces/1/files/trash-delete", map[string]interface{}{
		"ids": []int64{firstID},
	})
	deleteTrashRec := httptest.NewRecorder()
	if webErr := handler.handleTrashDelete(deleteTrashRec, deleteTrashReq, 1); webErr != nil {
		t.Fatalf("trash delete failed: %+v", webErr)
	}

	emptyReq := newJSONRequestWithClaims(t, http.MethodPost, "/api/spaces/1/files/trash-empty", map[string]interface{}{})
	emptyRec := httptest.NewRecorder()
	if webErr := handler.handleTrashEmpty(emptyRec, emptyReq, 1); webErr != nil {
		t.Fatalf("trash empty failed: %+v", webErr)
	}

	listRecAfterEmpty := httptest.NewRecorder()
	if webErr := handler.handleTrashList(listRecAfterEmpty, listReq, 1); webErr != nil {
		t.Fatalf("trash list after empty failed: %+v", webErr)
	}
	listPayloadAfterEmpty := decodeJSONBody(t, listRecAfterEmpty)
	itemsAfterEmpty := listPayloadAfterEmpty["items"].([]interface{})
	if len(itemsAfterEmpty) != 0 {
		t.Fatalf("expected empty trash, got %d items", len(itemsAfterEmpty))
	}
}

func TestTrash_InternalTrashPathIsBlockedFromRegularBrowseAndDownload(t *testing.T) {
	handler, root := setupTrashHandler(t)

	trashDir := filepath.Join(root, spaceTrashDirectoryName)
	if err := os.MkdirAll(trashDir, 0o755); err != nil {
		t.Fatalf("failed to create trash dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(trashDir, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatalf("failed to create trash file: %v", err)
	}

	downloadReq := newJSONRequestWithClaims(
		t,
		http.MethodGet,
		"/api/spaces/1/files/download?path=.cohesion_trash/secret.txt",
		nil,
	)
	downloadRec := httptest.NewRecorder()
	downloadErr := handler.handleFileDownload(downloadRec, downloadReq, 1)
	if downloadErr == nil {
		t.Fatal("expected forbidden error for download from trash path")
	}
	if downloadErr.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden status for trash path download, got %d", downloadErr.Code)
	}

	browseReq := newJSONRequestWithClaims(
		t,
		http.MethodGet,
		"/api/spaces/1/browse?path=.cohesion_trash",
		nil,
	)
	browseRec := httptest.NewRecorder()
	browseErr := handler.handleSpaceBrowse(browseRec, browseReq, 1)
	if browseErr == nil {
		t.Fatal("expected forbidden error for browse into trash path")
	}
	if browseErr.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden status for trash path browse, got %d", browseErr.Code)
	}
}
