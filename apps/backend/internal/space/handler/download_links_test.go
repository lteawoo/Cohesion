package handler

import (
	"context"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"taeu.kr/cohesion/internal/auth"
	"taeu.kr/cohesion/internal/space"
	store "taeu.kr/cohesion/internal/space/store"
)

type linkTestSpaces struct{ *fakeTransferSpaceStore }

func (s linkTestSpaces) GetAll(context.Context) ([]*space.Space, error) {
	var list []*space.Space
	for _, v := range s.spacesByID {
		list = append(list, v)
	}
	return list, nil
}

type shareFixture struct {
	*sessionFixture
	managers map[int64]bool
	spaces   linkTestSpaces
}

func newShareFixture(t *testing.T) *shareFixture {
	t.Helper()
	f := newSessionFixture(t)
	quota := int64(10)
	spaces := linkTestSpaces{&fakeTransferSpaceStore{spacesByID: map[int64]*space.Space{1: {ID: 1, SpaceName: "test", SpacePath: f.files, QuotaBytes: &quota}}}}
	f.service = space.NewService(spaces)
	if _, err := f.db.Exec("INSERT INTO users(id,username,password_hash,nickname) VALUES(1,'one','unused','one'),(2,'two','unused','two')"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec("INSERT INTO space(id,space_name,space_path) VALUES(1,'test',?)", f.files); err != nil {
		t.Fatal(err)
	}
	s := &shareFixture{f, map[int64]bool{1: true, 2: true}, spaces}
	s.restart()
	return s
}
func (s *shareFixture) restart() {
	s.reopen()
	s.h.SetDownloadLinkAccess(func(_ context.Context, owner, id int64) (bool, error) { return id == 1 && s.managers[owner], nil })
}
func (s *shareFixture) serve(method, path, body string, headers map[string]string, owner int64, code int) *httptest.ResponseRecorder {
	s.t.Helper()
	mux := http.NewServeMux()
	s.h.RegisterRoutes(mux)
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if owner != 0 {
		req = req.WithContext(auth.WithClaims(req.Context(), &auth.Claims{UserID: owner, Username: "tester"}))
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != code {
		s.t.Fatalf("%s got %d want %d: %s", method, rec.Code, code, rec.Body.String())
	}
	return rec
}

const linkRoute = "/api/spaces/1/files/download-links"

func (s *shareFixture) issue(path string, owner int64) store.DownloadLink {
	s.t.Helper()
	body, err := json.Marshal(map[string]any{"path": path, "days": 1})
	if err != nil {
		s.t.Fatal(err)
	}
	rec := s.serve("POST", linkRoute, string(body), nil, owner, 201)
	var link store.DownloadLink
	if err := json.Unmarshal(rec.Body.Bytes(), &link); err != nil || !validTransferID(link.Token) {
		s.t.Fatal("invalid created link", err)
	}
	return link
}
func (s *shareFixture) download(token string, code int) *httptest.ResponseRecorder {
	s.t.Helper()
	return s.serve("POST", "/api/shared-file", url.Values{"token": {token}}.Encode(), map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, 0, code)
}

func TestPublicDownloadLinkSnapshotRestartAndRevocation(t *testing.T) {
	s := newShareFixture(t)
	name := "회의 자료.txt"
	if runtime.GOOS != "windows" {
		name = "회의 12:30.txt"
	}
	path := filepath.Join(s.files, name)
	if err := os.WriteFile(path, []byte("v1"), 0600); err != nil {
		t.Fatal(err)
	}
	first := s.issue(name, 1)
	if err := os.WriteFile(path, []byte("v2"), 0600); err != nil {
		t.Fatal(err)
	}
	second := s.issue(name, 2)
	if first.Token == second.Token {
		t.Fatal("duplicate share credential")
	}
	oldPath := path + ".old"
	if err := os.Rename(path, oldPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("v3"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	s.restart()
	meta := s.serve("GET", "/api/shared-file", "", map[string]string{"X-Download-Link": first.Token}, 0, 200)
	var info map[string]any
	if err := json.Unmarshal(meta.Body.Bytes(), &info); err != nil || len(info) != 3 || info["name"] != name || info["size"] != float64(2) || info["expires"] == nil {
		t.Fatal("public metadata mismatch", err)
	}
	for key, want := range map[string]string{first.Token: "v1", second.Token: "v2"} {
		r := s.download(key, 200)
		kind, args, err := mime.ParseMediaType(r.Header().Get("Content-Disposition"))
		if r.Body.String() != want || kind != "attachment" || args["filename"] != name || err != nil {
			t.Fatalf("attachment mismatch, type %s, error %v", kind, err)
		}
		if r.Header().Get("Content-Type") != "application/octet-stream" || r.Header().Get("Cache-Control") != "no-store" || r.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatal("unexpected response headers")
		}
	}
	list := s.serve("GET", linkRoute, "", nil, 2, 200)
	var links []store.DownloadLink
	if err := json.Unmarshal(list.Body.Bytes(), &links); err != nil || len(links) != 2 {
		t.Fatal("manager could not list links", err)
	}
	if got := s.serve("GET", linkRoute+"?path=other-file", "", nil, 2, 200); strings.TrimSpace(got.Body.String()) != "[]" {
		t.Fatal("unexpected links for unrelated path")
	}
	s.serve("DELETE", linkRoute+"?id="+strconv.FormatInt(first.ID, 10), "", nil, 2, 204)
	s.download(first.Token, 404)
	s.download(second.Token, 200)
	stale := s.serve("POST", "/api/shared-file", url.Values{"token": {first.Token}}.Encode(), map[string]string{"Content-Type": "application/x-www-form-urlencoded", "Sec-Fetch-Mode": "navigate", "Accept": "text/html"}, 0, 303)
	if stale.Header().Get("Location") != "/share" {
		t.Fatal("unexpected redirect")
	}
	s.managers[2] = false
	s.download(second.Token, 404)
	s.serve("GET", "/api/shared-file", "", map[string]string{"X-Download-Link": second.Token}, 0, 404)
	s.serve("GET", linkRoute, "", nil, 2, 403)
	s.serve("GET", linkRoute, "", nil, 0, 401)
	s.serve("DELETE", "/api/spaces/2/files/download-links?id="+strconv.FormatInt(second.ID, 10), "", nil, 1, 403)
	s.serve("DELETE", linkRoute+"?id="+strconv.FormatInt(second.ID, 10), "", nil, 1, 204)
	files, err := os.ReadDir(s.h.transferRuntime.shared.dir)
	if err != nil || len(files) != 0 {
		t.Fatalf("unexpected snapshot files %d, error %v", len(files), err)
	}
}

func TestDownloadSnapshotCapacityAndExpiry(t *testing.T) {
	s := newShareFixture(t)
	if err := os.WriteFile(filepath.Join(s.files, "note"), []byte("123456"), 0600); err != nil {
		t.Fatal(err)
	}
	link := s.issue("note", 1)
	body := `{"path":"note","days":1}`
	s.serve("POST", linkRoute, body, nil, 2, 507)
	if _, err := s.db.Exec("UPDATE download_links SET expires=? WHERE id=?", time.Now().Add(-time.Second).Unix(), link.ID); err != nil {
		t.Fatal(err)
	}
	s.download(link.Token, 404)
	rt := s.h.transferRuntime
	pending := store.DownloadLink{SpaceID: 1, Snapshot: strings.Repeat("P", 32), Size: 6}
	quota := int64(10)
	if err := rt.shared.reserve(t.Context(), rt.store, pending, &quota); err != nil {
		t.Fatal(err)
	}
	s.serve("POST", linkRoute, body, nil, 1, 507)
	delete(rt.shared.pending, pending.Snapshot)
	orphan := rt.shared.filename(strings.Repeat("A", 32))
	if err := os.WriteFile(orphan, []byte("abandoned"), 0600); err != nil {
		t.Fatal(err)
	}
	s.restart()
	files, err := os.ReadDir(rt.shared.dir)
	if err != nil || len(files) != 0 {
		t.Fatalf("restart cleanup left %d files (%v)", len(files), err)
	}
	s.spaces.spacesByID[1].QuotaBytes = nil
	s.issue("note", 1)
	s.issue("note", 1)
}

func TestShareSourceAndStorageBoundaries(t *testing.T) {
	s := newShareFixture(t)
	s.serve("POST", linkRoute, `{"path":".","days":1}`, nil, 1, 400)
	s.serve("POST", linkRoute, `{"path":"note","days":366}`, nil, 1, 400)
	s.serve("GET", "/api/shared-file?token=placeholder", "", nil, 0, 404)
	if err := os.Mkdir(filepath.Join(s.files, spaceTrashDirectoryName), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.files, spaceTrashDirectoryName, "note"), []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	s.serve("POST", linkRoute, `{"path":".cohesion_trash/note","days":1}`, nil, 1, 400)
	if err := os.Symlink(filepath.Join(s.files, spaceTrashDirectoryName), filepath.Join(s.files, "alias")); err == nil {
		s.serve("POST", linkRoute, `{"path":"alias/note","days":1}`, nil, 1, 400)
	}
	storage := s.h.transferRuntime.shared.dir
	for _, dir := range []string{storage, filepath.Dir(storage)} {
		valid, err := s.service.ValidateSpaceRoot(t.Context(), dir)
		if err != nil || valid.Valid {
			t.Fatalf("reserved space root accepted (%v)", err)
		}
	}
	s.spaces.spacesByID[2] = &space.Space{ID: 2, SpacePath: filepath.Dir(storage)}
	s.serve("POST", linkRoute, `{"path":"note","days":1}`, nil, 1, 409)
}
func TestDownloadSnapshotCopyError(t *testing.T) {
	if snapshotCopyError(&os.PathError{Op: "write", Path: "test", Err: syscall.ENOSPC}).Code != 507 {
		t.Fatal("storage exhaustion lost")
	}
	if snapshotCopyError(errSnapshotChanged).Code != 409 {
		t.Fatal("source change lost")
	}
	if snapshotCopyError(errors.New("read error")).Code != 500 {
		t.Fatal("unexpected read error mapping")
	}
}
