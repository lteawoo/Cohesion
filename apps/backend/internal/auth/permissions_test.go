package auth

import (
	"net/http"
	"net/url"
	"testing"
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
