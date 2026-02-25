package auth

import (
	"net/http"
	"net/url"
	"testing"

	"taeu.kr/cohesion/internal/account"
)

func TestRequiredPermissionForRequest_RoleEndpoints(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		path     string
		expected string
	}{
		{name: "list roles", method: http.MethodGet, path: "/api/roles", expected: PermissionAccountRead},
		{name: "create role", method: http.MethodPost, path: "/api/roles", expected: PermissionAccountWrite},
		{name: "delete role", method: http.MethodDelete, path: "/api/roles/editor", expected: PermissionAccountWrite},
		{name: "list permission definitions", method: http.MethodGet, path: "/api/permissions", expected: PermissionAccountRead},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := &http.Request{
				Method: tc.method,
				URL:    &url.URL{Path: tc.path},
			}
			got, ok := requiredPermissionForRequest(req)
			if !ok {
				t.Fatalf("expected permission mapping for %s %s", tc.method, tc.path)
			}
			if got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestRequiredPermissionForRequest_SearchEndpoint(t *testing.T) {
	req := &http.Request{
		Method: http.MethodGet,
		URL:    &url.URL{Path: "/api/search/files"},
	}

	got, ok := requiredPermissionForRequest(req)
	if !ok {
		t.Fatal("expected permission mapping for search endpoint")
	}
	if got != PermissionFileRead {
		t.Fatalf("expected %q, got %q", PermissionFileRead, got)
	}
}

func TestRequiredPermissionForRequest_SpaceUsageAndQuota(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		path     string
		expected string
	}{
		{
			name:     "space usage list",
			method:   http.MethodGet,
			path:     "/api/spaces/usage",
			expected: PermissionSpaceRead,
		},
		{
			name:     "space quota patch",
			method:   http.MethodPatch,
			path:     "/api/spaces/1/quota",
			expected: PermissionSpaceWrite,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := &http.Request{
				Method: tc.method,
				URL:    &url.URL{Path: tc.path},
			}
			got, ok := requiredPermissionForRequest(req)
			if !ok {
				t.Fatalf("expected permission mapping for %s %s", tc.method, tc.path)
			}
			if got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestRequiredPermissionForRequest_SystemUpdateEndpoints(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		path     string
		expected string
	}{
		{
			name:     "get update status",
			method:   http.MethodGet,
			path:     "/api/system/update/status",
			expected: PermissionServerRead,
		},
		{
			name:     "start update",
			method:   http.MethodPost,
			path:     "/api/system/update/start",
			expected: PermissionServerWrite,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := &http.Request{
				Method: tc.method,
				URL:    &url.URL{Path: tc.path},
			}
			got, ok := requiredPermissionForRequest(req)
			if !ok {
				t.Fatalf("expected permission mapping for %s %s", tc.method, tc.path)
			}
			if got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestRequiredSpacePermissionForRequest_SpaceQuotaPatch(t *testing.T) {
	req := &http.Request{
		Method: http.MethodPatch,
		URL:    &url.URL{Path: "/api/spaces/7/quota"},
	}

	got, ok := requiredSpacePermissionForRequest(req)
	if !ok {
		t.Fatal("expected space permission mapping for quota patch endpoint")
	}
	if got.spaceID != 7 {
		t.Fatalf("expected space id 7, got %d", got.spaceID)
	}
	if got.required != account.PermissionWrite {
		t.Fatalf("expected required permission %s, got %s", account.PermissionWrite, got.required)
	}
}
