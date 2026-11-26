package handler

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"taeu.kr/cohesion/internal/platform/database"
	"testing"
	"time"

	store "taeu.kr/cohesion/internal/space/store"
)

func setupBundleFiles(t *testing.T, s *shareFixture) {
	t.Helper()
	for i, dir := range []string{"first", "second"} {
		folder := filepath.Join(s.files, dir)
		if err := os.Mkdir(folder, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(folder, "보고서.txt"), []byte(strconv.Itoa(i+1)), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

const bundleRequest = `{"paths":["first/보고서.txt","second/보고서.txt"],"days":1}`

func downloadItem(s *shareFixture, token, id string, status int) *httptest.ResponseRecorder {
	s.t.Helper()
	return s.serve("POST", "/api/shared-file", url.Values{"token": {token}, "item": {id}}.Encode(), map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, 0, status)
}

func TestDownloadBundleLifecycle(t *testing.T) {
	s := newShareFixture(t)
	setupBundleFiles(t, s)
	create := s.serve("POST", linkRoute, bundleRequest, nil, 1, 201)
	var link store.DownloadLink
	if err := json.Unmarshal(create.Body.Bytes(), &link); err != nil {
		t.Fatal(err)
	}
	if link.Size != 2 || link.ItemCount != 2 || link.Name != store.BundleDownloadName || link.Path != "" {
		t.Fatal("unexpected bundle description")
	}
	for _, dir := range []string{"first", "second"} {
		if err := os.Remove(filepath.Join(s.files, dir, "보고서.txt")); err != nil {
			t.Fatal(err)
		}
	}
	s.restart()
	list := s.serve("GET", linkRoute, "", nil, 1, 200)
	var links []store.DownloadLink
	if err := json.Unmarshal(list.Body.Bytes(), &links); err != nil || len(links) != 1 || links[0].ItemCount != 2 || links[0].ID != link.ID {
		t.Fatal("bundle missing from management list", err)
	}
	headers := map[string]string{"X-Download-Link": link.Token}
	meta := s.serve("GET", "/api/shared-file", "", headers, 0, 200)
	var result struct{ Items []store.DownloadLinkItem }
	if err := json.Unmarshal(meta.Body.Bytes(), &result); err != nil || len(result.Items) != 2 {
		t.Fatal("bad item metadata", err)
	}
	rec := s.download(link.Token, 200)
	archive, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil || len(archive.File) != 2 {
		t.Fatal("invalid archive", err)
	}
	for i, name := range []string{"보고서.txt", "보고서 (2).txt"} {
		item := result.Items[i]
		if item.Name != name || item.Size != 1 || item.ID != i+1 || archive.File[i].Name != name {
			t.Fatalf("entry %d: %+v", i, item)
		}
		f, err := archive.File[i].Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil || string(data) != strconv.Itoa(i+1) {
			t.Fatal("archive data mismatch", err)
		}
		if got := downloadItem(s, link.Token, strconv.Itoa(item.ID), 200).Body.String(); got != string(data) {
			t.Fatal("individual download mismatch")
		}
	}
	for _, bad := range []string{"0", "3", "../x", ""} {
		downloadItem(s, link.Token, bad, 404)
	}
	s.managers[1] = false
	s.serve("GET", "/api/shared-file", "", headers, 0, 404)
	downloadItem(s, link.Token, "1", 404)
	s.download(link.Token, 404)
	s.managers[1] = true
	if _, err := s.db.Exec("UPDATE download_links SET expires=? WHERE id=?", time.Now().Add(-time.Hour).Unix(), link.ID); err != nil {
		t.Fatal(err)
	}
	s.download(link.Token, 404)
	downloadItem(s, link.Token, "1", 404)
	s.serve("GET", "/api/shared-file", "", headers, 0, 404)
	if _, err := s.db.Exec("UPDATE download_links SET expires=? WHERE id=?", time.Now().Add(time.Hour).Unix(), link.ID); err != nil {
		t.Fatal(err)
	}
	s.serve("DELETE", linkRoute+"?id="+strconv.FormatInt(link.ID, 10), "", nil, 1, 204)
	s.download(link.Token, 404)
	downloadItem(s, link.Token, "1", 404)
	s.serve("GET", "/api/shared-file", "", headers, 0, 404)
}

func TestDownloadBundleRejectsPartialPublication(t *testing.T) {
	s := newShareFixture(t)
	setupBundleFiles(t, s)
	for _, bad := range []string{
		`{"paths":[],"days":1}`,
		`{"paths":["first/보고서.txt","first/보고서.txt"],"days":1}`,
		`{"paths":["first/보고서.txt","missing"],"days":1}`,
		`{"path":"first/보고서.txt","paths":["first/보고서.txt","second/보고서.txt"],"days":1}`,
	} {
		s.serve("POST", linkRoute, bad, nil, 1, 400)
	}
	quota := int64(1)
	s.spaces.spacesByID[1].QuotaBytes = &quota
	s.serve("POST", linkRoute, bundleRequest, nil, 1, 507)
	quota = 2
	if _, err := s.db.Exec("CREATE TRIGGER refuse_bundle BEFORE INSERT ON download_links BEGIN SELECT RAISE(ABORT, 'simulated publish failure'); END;"); err != nil {
		t.Fatal(err)
	}
	s.serve("POST", linkRoute, bundleRequest, nil, 1, 500)
	entries, err := os.ReadDir(s.h.transferRuntime.shared.dir)
	if err != nil || len(entries) != 0 || len(s.h.transferRuntime.shared.pending) != 0 {
		t.Fatal("failed publish retained snapshot", err)
	}
	if got := s.serve("GET", linkRoute, "", nil, 1, 200).Body.String(); string(bytes.TrimSpace([]byte(got))) != "[]" {
		t.Fatal("partial link was published")
	}
	if _, err := s.db.Exec("DROP TRIGGER refuse_bundle"); err != nil {
		t.Fatal(err)
	}
	s.serve("POST", linkRoute, bundleRequest, nil, 1, 201)
	s.serve("POST", linkRoute, bundleRequest, nil, 1, 507)
}

func TestExistingSingleLinkSurvivesBundleMigration(t *testing.T) {
	s := newShareFixture(t)
	if err := os.WriteFile(filepath.Join(s.files, "note"), []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	link := s.issue("note", 1)
	if _, err := s.db.Exec("ALTER TABLE download_links DROP COLUMN manifest"); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(t.Context(), s.db); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(t.Context(), s.db); err != nil {
		t.Fatal(err)
	}
	if got := s.download(link.Token, 200).Body.String(); got != "old" {
		t.Fatal("legacy download changed")
	}
	downloadItem(s, link.Token, "1", 404)
	setupBundleFiles(t, s)
	s.serve("POST", linkRoute, bundleRequest, nil, 1, 201)
}
