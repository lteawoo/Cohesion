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

func TestRequiredPermissionForRequest_ProfileUpdateEndpoint(t *testing.T) {
	req := &http.Request{
		Method: http.MethodPatch,
		URL:    &url.URL{Path: "/api/auth/me"},
	}

	got, ok := requiredPermissionForRequest(req)
	if !ok {
		t.Fatal("expected permission mapping for profile update endpoint")
	}
	if got != PermissionProfileWrite {
		t.Fatalf("expected %q, got %q", PermissionProfileWrite, got)
	}
}

func TestRequiredPermissionForRequest_BrowseEndpointUsesSpaceWrite(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		rawQuery string
	}{
		{
			name:     "without system query",
			rawQuery: "",
		},
		{
			name:     "with legacy system query",
			rawQuery: "system=true",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := &http.Request{
				Method: http.MethodGet,
				URL: &url.URL{
					Path:     "/api/browse",
					RawQuery: tc.rawQuery,
				},
			}

			got, ok := requiredPermissionForRequest(req)
			if !ok {
				t.Fatal("expected permission mapping for browse endpoint")
			}
			if got != PermissionSpaceWrite {
				t.Fatalf("expected %q, got %q", PermissionSpaceWrite, got)
			}
		})
	}
}

func TestRequiredPermissionForRequest_BaseDirectoriesEndpointUsesSpaceWrite(t *testing.T) {
	t.Parallel()

	req := &http.Request{
		Method: http.MethodGet,
		URL: &url.URL{
			Path: "/api/browse/base-directories",
		},
	}

	got, ok := requiredPermissionForRequest(req)
	if !ok {
		t.Fatal("expected permission mapping for base directories endpoint")
	}
	if got != PermissionSpaceWrite {
		t.Fatalf("expected %q, got %q", PermissionSpaceWrite, got)
	}
}

func TestRequiredPermissionForRequest_AuditEndpoints(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		path     string
		expected string
	}{
		{name: "list audit logs", method: http.MethodGet, path: "/api/audit/logs", expected: PermissionAccountRead},
		{name: "get audit log by id", method: http.MethodGet, path: "/api/audit/logs/12", expected: PermissionAccountRead},
		{name: "export audit logs", method: http.MethodGet, path: "/api/audit/logs/export", expected: PermissionAccountRead},
		{name: "cleanup audit logs", method: http.MethodPost, path: "/api/audit/logs/cleanup", expected: PermissionAccountWrite},
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
			name:     "space rename patch",
			method:   http.MethodPatch,
			path:     "/api/spaces/1",
			expected: PermissionSpaceWrite,
		},
		{
			name:     "space quota patch",
			method:   http.MethodPatch,
			path:     "/api/spaces/1/quota",
			expected: PermissionSpaceWrite,
		},
		{
			name:     "space root validation",
			method:   http.MethodPost,
			path:     "/api/spaces/validate-root",
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

func TestRequiredPermissionForRequest_SpaceMembers(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		path     string
		expected string
	}{
		{
			name:     "space members list",
			method:   http.MethodGet,
			path:     "/api/spaces/7/members",
			expected: PermissionAccountRead,
		},
		{
			name:     "space members replace",
			method:   http.MethodPut,
			path:     "/api/spaces/7/members",
			expected: PermissionAccountWrite,
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
	tests := []struct {
		name           string
		method         string
		path           string
		expectedSpace  int64
		expectedAccess account.Permission
	}{
		{
			name:           "space rename patch",
			method:         http.MethodPatch,
			path:           "/api/spaces/7",
			expectedSpace:  7,
			expectedAccess: account.PermissionWrite,
		},
		{
			name:           "space quota patch",
			method:         http.MethodPatch,
			path:           "/api/spaces/7/quota",
			expectedSpace:  7,
			expectedAccess: account.PermissionWrite,
		},
		{
			name:           "space members list",
			method:         http.MethodGet,
			path:           "/api/spaces/7/members",
			expectedSpace:  7,
			expectedAccess: account.PermissionRead,
		},
		{
			name:           "space members replace",
			method:         http.MethodPut,
			path:           "/api/spaces/7/members",
			expectedSpace:  7,
			expectedAccess: account.PermissionWrite,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := &http.Request{
				Method: tc.method,
				URL:    &url.URL{Path: tc.path},
			}

			got, ok := requiredSpacePermissionForRequest(req)
			if !ok {
				t.Fatalf("expected space permission mapping for %s", tc.path)
			}
			if got.spaceID != tc.expectedSpace {
				t.Fatalf("expected space id %d, got %d", tc.expectedSpace, got.spaceID)
			}
			if got.required != tc.expectedAccess {
				t.Fatalf("expected required permission %s, got %s", tc.expectedAccess, got.required)
			}
		})
	}
}

func TestDeniedAuditRuleForRequest_IncludedEndpoints(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		expectedAction string
	}{
		{
			name:           "account list",
			method:         http.MethodGet,
			path:           "/api/accounts",
			expectedAction: "account.list",
		},
		{
			name:           "account list with trailing slash",
			method:         http.MethodGet,
			path:           "/api/accounts/",
			expectedAction: "account.list",
		},
		{
			name:           "account create",
			method:         http.MethodPost,
			path:           "/api/accounts",
			expectedAction: "account.create",
		},
		{
			name:           "role list",
			method:         http.MethodGet,
			path:           "/api/roles",
			expectedAction: "role.list",
		},
		{
			name:           "role list with trailing slash",
			method:         http.MethodGet,
			path:           "/api/roles/",
			expectedAction: "role.list",
		},
		{
			name:           "permission list",
			method:         http.MethodGet,
			path:           "/api/permissions",
			expectedAction: "permission.list",
		},
		{
			name:           "audit logs read",
			method:         http.MethodGet,
			path:           "/api/audit/logs",
			expectedAction: "audit.logs.read",
		},
		{
			name:           "config read",
			method:         http.MethodGet,
			path:           "/api/config",
			expectedAction: "config.read",
		},
		{
			name:           "space file download multiple",
			method:         http.MethodPost,
			path:           "/api/spaces/7/files/download-multiple",
			expectedAction: "file.download-multiple",
		},
		{
			name:           "space update",
			method:         http.MethodPatch,
			path:           "/api/spaces/7",
			expectedAction: "space.update",
		},
		{
			name:           "space members replace",
			method:         http.MethodPut,
			path:           "/api/spaces/7/members",
			expectedAction: "space.members.replace",
		},
		{
			name:           "download by ticket",
			method:         http.MethodGet,
			path:           "/api/downloads/abc",
			expectedAction: "file.download-ticket",
		},
		{
			name:           "profile update",
			method:         http.MethodPatch,
			path:           "/api/auth/me",
			expectedAction: "profile.update",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := &http.Request{
				Method: tc.method,
				URL:    &url.URL{Path: tc.path},
			}

			rule, ok := deniedAuditRuleForRequest(req)
			if !ok {
				t.Fatalf("expected denied audit rule for %s %s", tc.method, tc.path)
			}
			if rule.Action != tc.expectedAction {
				t.Fatalf("expected action %q, got %q", tc.expectedAction, rule.Action)
			}
			if !rule.AllowUnauthorized {
				t.Fatalf("expected %s %s to allow unauthorized auditing", tc.method, tc.path)
			}
		})
	}
}

func TestDeniedAuditActionForSpaceFileAction_Mapping(t *testing.T) {
	tests := []struct {
		action         string
		expected       string
		expectedMapped bool
	}{
		{action: "download", expected: "file.download", expectedMapped: true},
		{action: "download-ticket", expected: "file.download-ticket", expectedMapped: true},
		{action: "rename", expected: "file.rename", expectedMapped: true},
		{action: "delete", expected: "file.delete", expectedMapped: true},
		{action: "delete-multiple", expected: "file.delete-multiple", expectedMapped: true},
		{action: "create-folder", expected: "file.mkdir", expectedMapped: true},
		{action: "upload", expected: "file.upload", expectedMapped: true},
		{action: "move", expected: "file.move", expectedMapped: true},
		{action: "copy", expected: "file.copy", expectedMapped: true},
		{action: "download-multiple", expected: "file.download-multiple", expectedMapped: true},
		{action: "download-multiple-ticket", expected: "file.download-multiple-ticket", expectedMapped: true},
		{action: "trash", expectedMapped: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.action, func(t *testing.T) {
			got, mapped := DeniedAuditActionForSpaceFileAction(tc.action)
			if mapped != tc.expectedMapped {
				t.Fatalf("expected mapped=%v, got %v", tc.expectedMapped, mapped)
			}
			if got != tc.expected {
				t.Fatalf("expected action %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestDeniedAuditRuleForRequest_ExcludedEndpoints(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{name: "system browse", method: http.MethodGet, path: "/api/browse"},
		{name: "search", method: http.MethodGet, path: "/api/search/files"},
		{name: "space browse", method: http.MethodGet, path: "/api/spaces/3/browse"},
		{name: "space list", method: http.MethodGet, path: "/api/spaces"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := &http.Request{
				Method: tc.method,
				URL:    &url.URL{Path: tc.path},
			}

			if _, ok := deniedAuditRuleForRequest(req); ok {
				t.Fatalf("did not expect denied audit rule for %s %s", tc.method, tc.path)
			}
		})
	}
}
