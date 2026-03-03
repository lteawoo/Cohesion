package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"taeu.kr/cohesion/internal/browse"
)

func TestHandleBrowseTreatsRequestsAsSystemBrowse(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	visibleDir := filepath.Join(tempDir, "visible")
	if err := os.Mkdir(visibleDir, 0o755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	tests := []struct {
		name     string
		rawQuery string
	}{
		{
			name:     "without system query",
			rawQuery: "path=" + url.QueryEscape(tempDir),
		},
		{
			name:     "with legacy system query",
			rawQuery: "path=" + url.QueryEscape(tempDir) + "&system=true",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := NewHandler(browse.NewService(), nil)
			req := httptest.NewRequest(http.MethodGet, "/api/browse?"+tc.rawQuery, nil)
			rec := httptest.NewRecorder()

			if webErr := h.handleBrowse(rec, req); webErr != nil {
				t.Fatalf("expected no error, got %+v", webErr)
			}
			if rec.Code != http.StatusOK {
				t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
			}

			var payload []browse.FileInfo
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if len(payload) == 0 {
				t.Fatalf("expected at least one entry in response, got %d", len(payload))
			}
		})
	}
}
