package webdav

import (
	"net/http"
	"net/http/httptest"
	"testing"

	corewebdav "taeu.kr/cohesion/internal/webdav"
)

func TestServeHTTP_OPTIONS_Root_NoAuthAllowed(t *testing.T) {
	h := &Handler{webDavService: corewebdav.NewService(nil, nil)}
	testPaths := []string{"/dav", "/dav/"}
	for _, path := range testPaths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodOptions, path, nil)
			rec := httptest.NewRecorder()

			if err := h.ServeHTTP(rec, req); err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}

			if rec.Code != http.StatusOK {
				t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
			}
			if got := rec.Header().Get("DAV"); got != "1, 2" {
				t.Fatalf("expected DAV header %q, got %q", "1, 2", got)
			}
			if got := rec.Header().Get("MS-Author-Via"); got != "DAV" {
				t.Fatalf("expected MS-Author-Via header %q, got %q", "DAV", got)
			}
		})
	}
}

func TestServeHTTP_OPTIONS_NonRoot_NoAuthUnauthorized(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodOptions, "/dav/test-space", nil)
	rec := httptest.NewRecorder()

	if err := h.ServeHTTP(rec, req); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	if got := rec.Header().Get("WWW-Authenticate"); got == "" {
		t.Fatalf("expected WWW-Authenticate header")
	}
}

func TestServeHTTP_PROPFIND_NoAuthUnauthorized(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest("PROPFIND", "/dav", nil)
	rec := httptest.NewRecorder()

	if err := h.ServeHTTP(rec, req); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	if got := rec.Header().Get("WWW-Authenticate"); got == "" {
		t.Fatalf("expected WWW-Authenticate header")
	}
}
