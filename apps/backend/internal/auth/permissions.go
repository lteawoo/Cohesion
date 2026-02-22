package auth

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"taeu.kr/cohesion/internal/account"
)

const (
	PermissionAccountRead  = "account.read"
	PermissionAccountWrite = "account.write"
	PermissionProfileRead  = "profile.read"
	PermissionProfileWrite = "profile.write"
	PermissionServerRead   = "server.config.read"
	PermissionServerWrite  = "server.config.write"
	PermissionSpaceRead    = "space.read"
	PermissionSpaceWrite   = "space.write"
	PermissionFileRead     = "file.read"
	PermissionFileWrite    = "file.write"
)

type spacePermissionRequirement struct {
	spaceID  int64
	required account.Permission
}

func (s *Service) PermissionsForRole(ctx context.Context, role account.Role) []string {
	permissions, err := s.accountService.GetRolePermissionKeys(ctx, string(role))
	if err != nil || len(permissions) == 0 {
		return []string{}
	}
	return permissions
}

func (s *Service) HasPermission(ctx context.Context, role account.Role, permission string) (bool, error) {
	permissionKeys, err := s.accountService.GetRolePermissionKeys(ctx, string(role))
	if err != nil {
		return false, err
	}
	for _, item := range permissionKeys {
		if item == permission {
			return true, nil
		}
	}
	return false, nil
}

func requiredPermissionForRequest(r *http.Request) (string, bool) {
	method := r.Method
	path := r.URL.Path

	if path == "/api/browse/base-directories" {
		return PermissionSpaceWrite, true
	}
	if path == "/api/browse" {
		if r.URL.Query().Get("system") == "true" {
			return PermissionSpaceWrite, true
		}
		return PermissionSpaceRead, true
	}
	if path == "/api/search/files" && method == http.MethodGet {
		return PermissionFileRead, true
	}

	if path == "/api/spaces" {
		if method == http.MethodGet {
			return PermissionSpaceRead, true
		}
		if method == http.MethodPost {
			return PermissionSpaceWrite, true
		}
	}

	if strings.HasPrefix(path, "/api/spaces/") && method == http.MethodDelete {
		return PermissionSpaceWrite, true
	}

	if strings.HasPrefix(path, "/api/spaces/") {
		action, ok := extractSpaceFileAction(path)
		if ok {
			if action == "download" || action == "download-ticket" || action == "download-multiple" || action == "download-multiple-ticket" {
				return PermissionFileRead, true
			}
			return PermissionFileWrite, true
		}
	}
	if strings.HasPrefix(path, "/api/downloads/") {
		return PermissionFileRead, true
	}

	if strings.HasPrefix(path, "/api/accounts") {
		if method == http.MethodGet {
			return PermissionAccountRead, true
		}
		return PermissionAccountWrite, true
	}
	if strings.HasPrefix(path, "/api/roles") || path == "/api/permissions" {
		if method == http.MethodGet {
			return PermissionAccountRead, true
		}
		return PermissionAccountWrite, true
	}

	switch path {
	case "/api/config":
		if method == http.MethodGet {
			return PermissionServerRead, true
		}
		if method == http.MethodPut {
			return PermissionServerWrite, true
		}
	case "/api/system/restart":
		if method == http.MethodPost {
			return PermissionServerWrite, true
		}
	}

	return "", false
}

func requiredSpacePermissionForRequest(r *http.Request) (*spacePermissionRequirement, bool) {
	path := r.URL.Path
	if !strings.HasPrefix(path, "/api/spaces/") {
		return nil, false
	}

	spaceID, ok := extractSpaceID(path)
	if !ok {
		return nil, false
	}

	if strings.Contains(path, "/browse") {
		return &spacePermissionRequirement{
			spaceID:  spaceID,
			required: account.PermissionRead,
		}, true
	}

	action, hasAction := extractSpaceFileAction(path)
	if hasAction {
		required := account.PermissionWrite
		if action == "download" || action == "download-ticket" || action == "download-multiple" || action == "download-multiple-ticket" {
			required = account.PermissionRead
		}
		return &spacePermissionRequirement{
			spaceID:  spaceID,
			required: required,
		}, true
	}

	return nil, false
}

func extractSpaceID(path string) (int64, bool) {
	trimmed := strings.TrimPrefix(path, "/api/spaces/")
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 || parts[0] == "" {
		return 0, false
	}

	spaceID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, false
	}
	return spaceID, true
}

func extractSpaceFileAction(path string) (string, bool) {
	trimmed := strings.TrimPrefix(path, "/api/spaces/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 3 {
		return "", false
	}
	if parts[1] != "files" || parts[2] == "" {
		return "", false
	}
	return parts[2], true
}
