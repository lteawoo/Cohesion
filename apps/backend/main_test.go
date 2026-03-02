package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"taeu.kr/cohesion/internal/platform/logging"
)

func TestRegisterWebDAVRoutes_NoRedirectForBasePath(t *testing.T) {
	mux := http.NewServeMux()
	registerWebDAVRoutes(mux, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	testPaths := []string{"/dav", "/dav/", "/dav/test-space"}
	for _, path := range testPaths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest("PROPFIND", path, nil)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusNoContent {
				t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
			}
			if location := rec.Header().Get("Location"); location != "" {
				t.Fatalf("expected no redirect location header, got %q", location)
			}
		})
	}
}

func TestEmitAccessLog_IncludesQueryWhenPresent(t *testing.T) {
	previousLogger := accessLogger
	defer func() {
		accessLogger = previousLogger
	}()

	var buffer bytes.Buffer
	accessLogger = newAccessLogger(&buffer)

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=hello&sort=asc", nil)
	emitAccessLog(req, http.StatusOK, 128, 250*time.Millisecond)

	line := buffer.String()
	if !strings.Contains(line, "event="+logging.EventHTTPAccess) {
		t.Fatalf("expected access event in log line, got %q", line)
	}
	if !strings.Contains(line, "path=/api/search") {
		t.Fatalf("expected path field in log line, got %q", line)
	}
	if !strings.Contains(line, `query="q=hello&sort=asc"`) {
		t.Fatalf("expected query field in log line, got %q", line)
	}
}

func TestEmitAccessLog_OmitsQueryWhenEmpty(t *testing.T) {
	previousLogger := accessLogger
	defer func() {
		accessLogger = previousLogger
	}()

	var buffer bytes.Buffer
	accessLogger = newAccessLogger(&buffer)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	emitAccessLog(req, http.StatusOK, 64, 50*time.Millisecond)

	line := buffer.String()
	if strings.Contains(line, "query=") {
		t.Fatalf("expected no query field for empty query string, got %q", line)
	}
}
