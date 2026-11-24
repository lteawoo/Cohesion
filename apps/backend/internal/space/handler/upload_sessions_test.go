package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"taeu.kr/cohesion/internal/auth"
	"taeu.kr/cohesion/internal/space"
)

type sessionFixture struct {
	t                    *testing.T
	db                   *sql.DB
	h                    *Handler
	service              *space.Service
	state, dbPath, files string
}

func newSessionFixture(t *testing.T) *sessionFixture {
	root := t.TempDir()
	quota := int64(10)
	f := &sessionFixture{t: t, state: filepath.Join(root, "state"), dbPath: filepath.Join(root, "db"), files: t.TempDir()}
	f.service = space.NewService(&fakeTransferSpaceStore{spacesByID: map[int64]*space.Space{1: {ID: 1, SpaceName: "test", SpacePath: f.files, QuotaBytes: &quota}}})
	f.reopen()
	return f
}

func (f *sessionFixture) reopen() {
	f.t.Helper()
	if f.db != nil {
		if err := f.db.Close(); err != nil {
			f.t.Fatal(err)
		}
	}
	f.db = openTransferTestDB(f.t, f.dbPath)
	f.h = NewHandler(f.service, nil, nil)
	f.h.SetTransferRuntime(openTransferTestRuntime(f.t, f.db, f.service, f.state))
}

func (f *sessionFixture) request(method, query, body string, offset, owner, spaceID int64, want int) uploadSessionResponse {
	f.t.Helper()
	r := httptest.NewRequest(method, "/api/spaces/"+strconv.FormatInt(spaceID, 10)+"/files/upload-sessions"+query, strings.NewReader(body))
	if offset >= 0 {
		r.Header.Set("Upload-Offset", strconv.FormatInt(offset, 10))
	}
	ctx := auth.WithClaims(r.Context(), &auth.Claims{UserID: owner, Username: "tester"})
	rec := httptest.NewRecorder()
	err := f.h.handleUploadSessions(rec, r.WithContext(ctx), spaceID)
	code := rec.Code
	if err != nil {
		code = err.Code
	}
	if code != want {
		f.t.Fatalf("%s %s got %d (%v), want %d", method, query, code, err, want)
	}
	var v uploadSessionResponse
	if code == 200 {
		if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
			f.t.Fatal(err)
		}
	}
	return v
}

func (f *sessionFixture) create() string {
	return f.request("POST", "", `{"filename":"demo.bin","size":8}`, -1, 1, 1, 200).ID
}

func (f *sessionFixture) assertClean() {
	f.t.Helper()
	for _, table := range []string{"upload_sessions", "upload_staging_files", "upload_quota_reservations"} {
		var n int
		if err := f.db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); err != nil || n != 0 {
			f.t.Fatalf("unclean %s: %d %v", table, n, err)
		}
	}
	entries, err := os.ReadDir(f.h.transferRuntime.uploads)
	if err != nil || len(entries) != 0 {
		f.t.Fatalf("stages: %v %v", entries, err)
	}
	if n := f.h.quotaService.ReservedBytes(1); n != 0 {
		f.t.Fatalf("reserved: %d", n)
	}
}

func TestResumeAfterRestartAndLostResponse(t *testing.T) {
	f := newSessionFixture(t)
	id := f.create()
	q := "?id=" + id
	f.request("POST", "", `{"filename":"other","size":3}`, -1, 1, 1, 507)
	f.request("GET", q, "", -1, 2, 1, 404)
	f.request("PATCH", q, "bad", 0, 1, 2, 404)
	f.request("PATCH", q, "abc", 0, 1, 1, 200)
	f.request("PATCH", q, "no", 0, 1, 1, 409)
	f.request("POST", q, "", -1, 1, 1, 409)
	u, err := f.h.transferRuntime.store.Session(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(u.Stage, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.WriteString("xx"); err != nil {
		t.Fatal(err)
	}
	file.Close()
	f.reopen()
	if got := f.h.quotaService.ReservedBytes(1); got != 8 {
		t.Fatalf("restored reservation %d", got)
	}
	if v := f.request("GET", q, "", -1, 1, 1, 200); v.Offset != 3 {
		t.Fatal(v)
	}
	f.request("PATCH", q, "123456", 3, 1, 1, 400)
	f.request("PATCH", q, "defgh", 3, 1, 1, 200)
	v := f.request("POST", q, "", -1, 1, 1, 200)
	if v.Status != "uploaded" {
		t.Fatal(v)
	}
	f.assertClean()
	f.reopen()
	v = f.request("GET", q, "", -1, 1, 1, 200)
	if v.Status != "uploaded" || v.Filename != "demo.bin" {
		t.Fatal(v)
	}
	f.request("POST", q, "", -1, 2, 1, 404)
	data, err := os.ReadFile(filepath.Join(f.files, "demo.bin"))
	if err != nil || string(data) != "abcdefgh" {
		t.Fatalf("%q %v", data, err)
	}
}

func TestUploadCancellationExpiryAndValidation(t *testing.T) {
	f := newSessionFixture(t)
	f.request("POST", "", `{"filename":"../bad","size":1}`, -1, 1, 1, 400)
	f.request("POST", "", `{"filename":"ok","path":"../bad","size":1}`, -1, 1, 1, 400)
	f.request("POST", "", `{"filename":"ok","size":1}`, -1, 0, 1, 401)
	for _, expire := range []bool{false, true} {
		id := f.create()
		q := "?id=" + id
		f.request("PATCH", q, "a", 0, 1, 1, 200)
		if expire {
			u, err := f.h.transferRuntime.store.Session(t.Context(), id)
			if err != nil {
				t.Fatal(err)
			}
			u.Expires = time.Now().Add(-time.Minute).Unix()
			if err := f.h.transferRuntime.store.SaveSession(t.Context(), u); err != nil {
				t.Fatal(err)
			}
			f.request("GET", q, "", -1, 1, 1, 404)
			if err := f.h.transferRuntime.expireUploadSessions(t.Context()); err != nil {
				t.Fatal(err)
			}
		} else {
			f.request("DELETE", q, "", -1, 2, 1, 404)
			f.request("DELETE", q, "", -1, 1, 1, 204)
		}
		f.assertClean()
		f.request("GET", q, "", -1, 1, 1, 404)
		if _, err := os.Stat(filepath.Join(f.files, "demo.bin")); !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
	}
}

func TestUploadConflictAtCompletionKeepsBytes(t *testing.T) {
	f := newSessionFixture(t)
	q := "?id=" + f.create()
	f.request("PATCH", q, "abcdefgh", 0, 1, 1, 200)
	if err := os.WriteFile(filepath.Join(f.files, "demo.bin"), []byte("a"), 0600); err != nil {
		t.Fatal(err)
	}
	f.request("POST", q, "", -1, 1, 1, 409)
	v := f.request("POST", q+"&conflictPolicy=rename", "", -1, 1, 1, 200)
	if v.Filename != "demo (1).bin" {
		t.Fatal(v)
	}
	f.assertClean()
}

func TestResumeCompletesPublicationAfterInterruptedRename(t *testing.T) {
	f := newSessionFixture(t)
	id := f.create()
	q := "?id=" + id
	f.request("PATCH", q, "abcdefgh", 0, 1, 1, 200)
	u, err := f.h.transferRuntime.store.Session(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := openUploadDir(u)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	temp := uploadStagePrefix + id
	if err := copyForPublication(dir, temp, u); err != nil {
		t.Fatal(err)
	}
	u.Destination = u.Name
	u.PublishReady = true
	if err := f.h.transferRuntime.store.SaveSession(t.Context(), u); err != nil {
		t.Fatal(err)
	}
	if err := dir.Rename(temp, u.Name); err != nil {
		t.Fatal(err)
	}
	f.reopen()
	v := f.request("POST", q, "", -1, 1, 1, 200)
	if v.Status != "uploaded" || v.Filename != u.Name {
		t.Fatal(v)
	}
	f.assertClean()
}

func TestConcurrentChunksAdvanceOffsetOnce(t *testing.T) {
	f := newSessionFixture(t)
	id := f.create()
	results := make(chan int, 2)
	for _, chunk := range []string{"abcd", "WXYZ"} {
		go func() {
			r := httptest.NewRequest("PATCH", "/api/spaces/1/files/upload-sessions?id="+id, strings.NewReader(chunk))
			r.Header.Set("Upload-Offset", "0")
			rec := httptest.NewRecorder()
			ctx := auth.WithClaims(r.Context(), &auth.Claims{UserID: 1, Username: "tester"})
			if err := f.h.handleUploadSessions(rec, r.WithContext(ctx), 1); err != nil {
				results <- err.Code
			} else {
				results <- rec.Code
			}
		}()
	}
	first, second := <-results, <-results
	if !((first == 200 && second == 409) || (first == 409 && second == 200)) {
		t.Fatalf("chunk responses %d, %d", first, second)
	}
	v := f.request("GET", "?id="+id, "", -1, 1, 1, 200)
	if v.Offset != 4 {
		t.Fatal(v)
	}
}
