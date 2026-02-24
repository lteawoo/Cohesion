package system

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNormalizeVersionTag(t *testing.T) {
	testCases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: "dev"},
		{name: "already v prefixed", input: "v0.3.0", want: "v0.3.0"},
		{name: "upper v prefixed", input: "V0.3.0", want: "v0.3.0"},
		{name: "number only", input: "0.3.0", want: "v0.3.0"},
		{name: "non semver token", input: "dev", want: "dev"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeVersionTag(tc.input)
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestIsNewerVersion(t *testing.T) {
	testCases := []struct {
		name    string
		current string
		latest  string
		want    bool
	}{
		{name: "latest patch", current: "v0.3.0", latest: "v0.3.1", want: true},
		{name: "same version", current: "v0.3.1", latest: "v0.3.1", want: false},
		{name: "older latest", current: "v0.4.0", latest: "v0.3.9", want: false},
		{name: "pre-release latest", current: "v0.3.0", latest: "v0.4.0-rc.1", want: true},
		{name: "invalid current", current: "dev", latest: "v0.4.0", want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := isNewerVersion(tc.current, tc.latest)
			if got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestUpdateCheckerCheck(t *testing.T) {
	t.Run("returns update available when newer release exists", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/releases/latest" {
				t.Fatalf("unexpected request path: %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name":"v0.4.0","html_url":"https://github.com/lteawoo/Cohesion/releases/tag/v0.4.0"}`))
		}))
		defer server.Close()

		checker := NewUpdateChecker(UpdateCheckerConfig{
			APIBaseURL: server.URL,
			CacheTTL:   time.Minute,
		})

		result, err := checker.Check(context.Background(), "v0.3.0")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !result.UpdateAvailable {
			t.Fatal("expected updateAvailable=true")
		}
		if result.LatestVersion != "v0.4.0" {
			t.Fatalf("expected latest version v0.4.0, got %q", result.LatestVersion)
		}
		if result.ReleaseURL == "" {
			t.Fatal("expected release url to be set")
		}
	})

	t.Run("returns error when release api fails", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		checker := NewUpdateChecker(UpdateCheckerConfig{
			APIBaseURL: server.URL,
			CacheTTL:   time.Minute,
		})

		_, err := checker.Check(context.Background(), "v0.3.0")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
