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

type deniedAuditRule struct {
	Action            string
	AllowUnauthorized bool
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
		return PermissionSpaceWrite, true
	}
	if path == "/api/search/files" && method == http.MethodGet {
		return PermissionFileRead, true
	}
	if path == "/api/auth/me" && method == http.MethodPatch {
		return PermissionProfileWrite, true
	}
	if path == "/api/spaces/usage" && method == http.MethodGet {
		return PermissionSpaceRead, true
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
	if isDirectSpaceRoute(path) && method == http.MethodPatch {
		return PermissionSpaceWrite, true
	}
	if strings.HasPrefix(path, "/api/spaces/") && strings.HasSuffix(path, "/quota") && method == http.MethodPatch {
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
	if path == "/api/audit/logs" || strings.HasPrefix(path, "/api/audit/logs/") {
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
	case "/api/system/update/status":
		if method == http.MethodGet {
			return PermissionServerRead, true
		}
	case "/api/system/update/start":
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
	if isDirectSpaceRoute(path) && r.Method == http.MethodPatch {
		return &spacePermissionRequirement{
			spaceID:  spaceID,
			required: account.PermissionWrite,
		}, true
	}
	if strings.HasSuffix(path, "/quota") && r.Method == http.MethodPatch {
		return &spacePermissionRequirement{
			spaceID:  spaceID,
			required: account.PermissionWrite,
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

func isDirectSpaceRoute(path string) bool {
	trimmed := strings.TrimPrefix(path, "/api/spaces/")
	parts := strings.Split(trimmed, "/")
	return len(parts) == 1 && parts[0] != ""
}

func deniedAuditRuleForRequest(r *http.Request) (deniedAuditRule, bool) {
	method := r.Method
	path := r.URL.Path

	if path == "/api/accounts" {
		switch method {
		case http.MethodGet:
			return deniedAuditRule{Action: "account.list", AllowUnauthorized: true}, true
		case http.MethodPost:
			return deniedAuditRule{Action: "account.create", AllowUnauthorized: true}, true
		}
	}
	if strings.HasPrefix(path, "/api/accounts/") {
		accountPath := strings.TrimPrefix(path, "/api/accounts/")
		if strings.Trim(accountPath, "/") == "" && method == http.MethodGet {
			return deniedAuditRule{Action: "account.list", AllowUnauthorized: true}, true
		}
		parts := strings.Split(accountPath, "/")
		if len(parts) > 0 && parts[0] != "" {
			if len(parts) == 1 && method == http.MethodPatch {
				return deniedAuditRule{Action: "account.update", AllowUnauthorized: true}, true
			}
			if len(parts) == 1 && method == http.MethodDelete {
				return deniedAuditRule{Action: "account.delete", AllowUnauthorized: true}, true
			}
			if len(parts) > 1 && parts[1] == "permissions" && method == http.MethodPut {
				return deniedAuditRule{Action: "account.permissions.replace", AllowUnauthorized: true}, true
			}
		}
	}

	if path == "/api/roles" {
		switch method {
		case http.MethodGet:
			return deniedAuditRule{Action: "role.list", AllowUnauthorized: true}, true
		case http.MethodPost:
			return deniedAuditRule{Action: "role.create", AllowUnauthorized: true}, true
		}
	}
	if strings.HasPrefix(path, "/api/roles/") {
		rolePath := strings.TrimPrefix(path, "/api/roles/")
		if strings.Trim(rolePath, "/") == "" && method == http.MethodGet {
			return deniedAuditRule{Action: "role.list", AllowUnauthorized: true}, true
		}
		parts := strings.Split(rolePath, "/")
		if len(parts) > 0 && parts[0] != "" {
			if len(parts) == 1 && method == http.MethodDelete {
				return deniedAuditRule{Action: "role.delete", AllowUnauthorized: true}, true
			}
			if len(parts) > 1 && parts[1] == "permissions" && method == http.MethodPut {
				return deniedAuditRule{Action: "role.permissions.replace", AllowUnauthorized: true}, true
			}
		}
	}
	if path == "/api/permissions" && method == http.MethodGet {
		return deniedAuditRule{Action: "permission.list", AllowUnauthorized: true}, true
	}
	if path == "/api/auth/me" && method == http.MethodPatch {
		return deniedAuditRule{Action: "profile.update", AllowUnauthorized: true}, true
	}

	if (path == "/api/audit/logs" || strings.HasPrefix(path, "/api/audit/logs/")) && method == http.MethodGet {
		return deniedAuditRule{Action: "audit.logs.read", AllowUnauthorized: true}, true
	}
	if path == "/api/audit/logs/cleanup" && method == http.MethodPost {
		return deniedAuditRule{Action: "audit.logs.cleanup", AllowUnauthorized: true}, true
	}

	if path == "/api/config" {
		if method == http.MethodGet {
			return deniedAuditRule{Action: "config.read", AllowUnauthorized: true}, true
		}
		if method == http.MethodPut {
			return deniedAuditRule{Action: "config.update", AllowUnauthorized: true}, true
		}
	}
	if path == "/api/system/restart" && method == http.MethodPost {
		return deniedAuditRule{Action: "system.restart", AllowUnauthorized: true}, true
	}
	if path == "/api/system/update/start" && method == http.MethodPost {
		return deniedAuditRule{Action: "system.update.start", AllowUnauthorized: true}, true
	}

	if path == "/api/spaces" && method == http.MethodPost {
		return deniedAuditRule{Action: "space.create", AllowUnauthorized: true}, true
	}
	if strings.HasPrefix(path, "/api/spaces/") && method == http.MethodDelete {
		if _, ok := extractSpaceID(path); ok {
			return deniedAuditRule{Action: "space.delete", AllowUnauthorized: true}, true
		}
	}
	if isDirectSpaceRoute(path) && method == http.MethodPatch {
		if _, ok := extractSpaceID(path); ok {
			return deniedAuditRule{Action: "space.update", AllowUnauthorized: true}, true
		}
	}
	if strings.HasPrefix(path, "/api/spaces/") && strings.HasSuffix(path, "/quota") && method == http.MethodPatch {
		if _, ok := extractSpaceID(path); ok {
			return deniedAuditRule{Action: "space.quota.update", AllowUnauthorized: true}, true
		}
	}
	if strings.HasPrefix(path, "/api/spaces/") {
		spaceAction, ok := extractSpaceFileAction(path)
		if ok {
			action, mapped := DeniedAuditActionForSpaceFileAction(spaceAction)
			if mapped {
				return deniedAuditRule{Action: action, AllowUnauthorized: true}, true
			}
		}
	}
	if strings.HasPrefix(path, "/api/downloads/") && method == http.MethodGet {
		return deniedAuditRule{Action: "file.download-ticket", AllowUnauthorized: true}, true
	}

	return deniedAuditRule{}, false
}

// DeniedAuditActionForSpaceFileAction maps space file actions to denied audit actions.
func DeniedAuditActionForSpaceFileAction(action string) (string, bool) {
	switch action {
	case "download":
		return "file.download", true
	case "download-ticket":
		return "file.download-ticket", true
	case "rename":
		return "file.rename", true
	case "delete":
		return "file.delete", true
	case "delete-multiple":
		return "file.delete-multiple", true
	case "create-folder":
		return "file.mkdir", true
	case "upload":
		return "file.upload", true
	case "move":
		return "file.move", true
	case "copy":
		return "file.copy", true
	case "download-multiple":
		return "file.download-multiple", true
	case "download-multiple-ticket":
		return "file.download-multiple-ticket", true
	default:
		return "", false
	}
}
