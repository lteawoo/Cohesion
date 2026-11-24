package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"taeu.kr/cohesion/internal/auth"
	store "taeu.kr/cohesion/internal/space/store"
)

func (f *sessionFixture) manager(method, query, body string, owner int64, want int) []byte {
	f.t.Helper()
	req := httptest.NewRequest(method, "/api/spaces/1/files/file-requests?folder=inbox"+query, strings.NewReader(body))
	req = req.WithContext(auth.WithClaims(req.Context(), &auth.Claims{UserID: owner}))
	rec := httptest.NewRecorder()
	e := f.h.handleFileRequests(rec, req, 1)
	code := rec.Code
	if e != nil {
		code = e.Code
	}
	if code != want {
		f.t.Fatalf("manager %s returned %d, want %d: %v", method, code, want, e)
	}
	return rec.Body.Bytes()
}

func (f *sessionFixture) drop(method, token, key, body string, offset, want int) []byte {
	f.t.Helper()
	r := httptest.NewRequest(method, "/api/file-drop", strings.NewReader(body))
	r.Header.Set("X-File-Request", token)
	r.Header.Set("X-Submission-Key", key)
	if offset >= 0 {
		r.Header.Set("Upload-Offset", strconv.Itoa(offset))
	}
	rec := httptest.NewRecorder()
	e := f.h.handleFileDrop(rec, r)
	code := rec.Code
	if e != nil {
		code = e.Code
	}
	if code != want {
		f.t.Fatalf("drop %s returned %d, want %d: %v", method, code, want, e)
	}
	return rec.Body.Bytes()
}

func (f *sessionFixture) allowDrops() {
	f.h.SetFileRequestAccess(func(_ context.Context, owner, spaceID int64) (bool, error) { return owner == 1 && spaceID == 1, nil })
}

func newRequestFixture(t *testing.T) (*sessionFixture, store.FileRequest) {
	t.Helper()
	f := newSessionFixture(t)
	if _, err := f.db.Exec("INSERT INTO users(id,username,password_hash,nickname) VALUES(1,'tester','unused','tester')"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec("INSERT INTO space(id,space_name,space_path) VALUES(1,'test',?)", f.files); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(f.files, "inbox"), 0700); err != nil {
		t.Fatal(err)
	}
	f.allowDrops()
	var link store.FileRequest
	if err := json.Unmarshal(f.manager("POST", "", `{"days":1,"byteLimit":6}`, 1, 201), &link); err != nil {
		t.Fatal(err)
	}
	return f, link
}

func TestFileRequestIsolationRestartAndLimits(t *testing.T) {
	f, link := newRequestFixture(t)
	first, second := strings.Repeat("A", 32), strings.Repeat("B", 32)
	f.manager("GET", "", "", 0, 401)
	if string(f.manager("GET", "", "", 2, 200)) != "[]\n" {
		t.Fatal("another owner saw request")
	}
	info := map[string]any{}
	if err := json.Unmarshal(f.drop("GET", link.Token, "", "", -1, 200), &info); err != nil || len(info) != 2 || info["title"] != "inbox" {
		t.Fatal("public metadata", info, err)
	}
	f.drop("GET", "", "", "", -1, 404)
	body := `{"filename":"photo.jpg","size":3}`
	for _, key := range []string{first, second} {
		f.drop("POST", link.Token, key, body, -1, 200)
	}
	f.drop("POST", link.Token, strings.Repeat("C", 32), body, -1, 409)
	f.drop("PATCH", link.Token, first, "a", 0, 200)
	f.reopen()
	f.allowDrops()
	if !strings.Contains(string(f.drop("GET", link.Token, first, "", -1, 200)), `"offset":1`) {
		t.Fatal("offset lost")
	}
	if !strings.Contains(string(f.drop("GET", link.Token, second, "", -1, 200)), `"offset":0`) {
		t.Fatal("submissions mixed")
	}
	f.drop("PATCH", link.Token, first, "bc", 1, 200)
	f.drop("PATCH", link.Token, second, "xyz", 0, 200)
	for _, key := range []string{first, second} {
		for range 2 {
			res := string(f.drop("PUT", link.Token, key, "", -1, 200))
			if strings.Contains(res, "photo") || !strings.Contains(res, "uploaded") {
				t.Fatal(res)
			}
		}
	}
	for name, want := range map[string]string{"photo.jpg": "abc", "photo (1).jpg": "xyz"} {
		data, err := os.ReadFile(filepath.Join(f.files, "inbox", name))
		if err != nil || string(data) != want {
			t.Fatal(name, err, string(data))
		}
	}
	f.drop("POST", link.Token, strings.Repeat("C", 32), body, -1, 409)
	var list []store.FileRequest
	if err := json.Unmarshal(f.manager("GET", "", "", 1, 200), &list); err != nil || len(list) != 1 || list[0].Received != 6 {
		t.Fatal(list, err)
	}
	f.assertClean()
}

func TestFileRequestClosureAndExpiry(t *testing.T) {
	for _, end := range []string{"close", "expire", "revoke"} {
		t.Run(end, func(t *testing.T) {
			f, link := newRequestFixture(t)
			key := strings.Repeat("Z", 32)
			f.drop("POST", link.Token, key, `{"filename":"report","size":1}`, -1, 200)
			f.drop("PATCH", link.Token, key, "x", 0, 200)
			switch end {
			case "expire":
				if _, err := f.db.Exec("UPDATE file_requests SET expires=0"); err != nil {
					t.Fatal(err)
				}
			case "close":
				query := "&id=" + strconv.FormatInt(link.ID, 10)
				f.manager("DELETE", query, "", 2, 404)
				f.manager("DELETE", query, "", 1, 204)
			case "revoke":
				f.h.SetFileRequestAccess(func(context.Context, int64, int64) (bool, error) { return false, nil })
			}
			f.drop("PUT", link.Token, key, "", -1, 404)
			if _, err := os.Stat(filepath.Join(f.files, "inbox", "report")); !os.IsNotExist(err) {
				t.Fatal(err)
			}
			if end != "revoke" {
				if err := f.h.transferRuntime.expireUploadSessions(t.Context()); err != nil {
					t.Fatal(err)
				}
				f.assertClean()
			}
		})
	}
}
