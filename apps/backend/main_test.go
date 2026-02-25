package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
