package account

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"taeu.kr/cohesion/internal/audit"
	"taeu.kr/cohesion/internal/platform/web"
)

type Handler struct {
	service       *Service
	auditRecorder audit.Recorder
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) SetAuditRecorder(recorder audit.Recorder) {
	h.auditRecorder = recorder
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("/api/accounts", web.Handler(h.handleAccounts))
	mux.Handle("/api/accounts/", web.Handler(h.handleAccountByID))
	mux.Handle("/api/roles", web.Handler(h.handleRoles))
	mux.Handle("/api/roles/", web.Handler(h.handleRoleByName))
	mux.Handle("/api/permissions", web.Handler(h.handlePermissionDefinitions))
	mux.Handle("GET /api/setup/status", web.Handler(h.handleSetupStatus))
	mux.Handle("POST /api/setup/admin", web.Handler(h.handleSetupAdmin))
}

func (h *Handler) handleAccounts(w http.ResponseWriter, r *http.Request) *web.Error {
	switch r.Method {
	case http.MethodGet:
		users, err := h.service.ListUsers(r.Context())
		if err != nil {
			return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to list users", Err: err}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(users)
		return nil
	case http.MethodPost:
		var req CreateUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return &web.Error{Code: http.StatusBadRequest, Message: "Invalid request body", Err: err}
		}
		user, err := h.service.CreateUser(r.Context(), &req)
		if err != nil {
			h.recordAudit(r, audit.Event{
				Action: "account.create",
				Result: audit.ResultFailure,
				Target: "account",
				Metadata: map[string]any{
					"reason": "create_failed",
				},
			})
			return &web.Error{Code: http.StatusBadRequest, Message: "Failed to create user", Err: err}
		}
		h.recordAudit(r, audit.Event{
			Action: "account.create",
			Result: audit.ResultSuccess,
			Target: "user:" + strconv.FormatInt(user.ID, 10),
			Metadata: map[string]any{
				"userId":        user.ID,
				"username":      user.Username,
				"role":          user.Role,
				"changedFields": []string{"username", "nickname", "role"},
			},
		})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(user)
		return nil
	default:
		return &web.Error{Code: http.StatusMethodNotAllowed, Message: "Method not allowed"}
	}
}

func (h *Handler) handleAccountByID(w http.ResponseWriter, r *http.Request) *web.Error {
	path := strings.TrimPrefix(r.URL.Path, "/api/accounts/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		return &web.Error{Code: http.StatusBadRequest, Message: "Invalid account path"}
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return &web.Error{Code: http.StatusBadRequest, Message: "Invalid account id", Err: err}
	}

	if len(parts) > 1 && parts[1] == "permissions" {
		return h.handlePermissions(w, r, id)
	}

	switch r.Method {
	case http.MethodPatch:
		var req UpdateUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return &web.Error{Code: http.StatusBadRequest, Message: "Invalid request body", Err: err}
		}
		user, err := h.service.UpdateUser(r.Context(), id, &req)
		if err != nil {
			h.recordAudit(r, audit.Event{
				Action: "account.update",
				Result: audit.ResultFailure,
				Target: "user:" + strconv.FormatInt(id, 10),
				Metadata: map[string]any{
					"userId": id,
					"reason": "update_failed",
				},
			})
			return &web.Error{Code: http.StatusBadRequest, Message: "Failed to update user", Err: err}
		}
		h.recordAudit(r, audit.Event{
			Action: "account.update",
			Result: audit.ResultSuccess,
			Target: "user:" + strconv.FormatInt(user.ID, 10),
			Metadata: map[string]any{
				"userId":        user.ID,
				"username":      user.Username,
				"role":          user.Role,
				"changedFields": changedUserFields(&req),
			},
		})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(user)
		return nil
	case http.MethodDelete:
		if err := h.service.DeleteUser(r.Context(), id); err != nil {
			h.recordAudit(r, audit.Event{
				Action: "account.delete",
				Result: audit.ResultFailure,
				Target: "user:" + strconv.FormatInt(id, 10),
				Metadata: map[string]any{
					"userId": id,
					"reason": "delete_failed",
				},
			})
			return &web.Error{Code: http.StatusBadRequest, Message: "Failed to delete user", Err: err}
		}
		h.recordAudit(r, audit.Event{
			Action: "account.delete",
			Result: audit.ResultSuccess,
			Target: "user:" + strconv.FormatInt(id, 10),
			Metadata: map[string]any{
				"userId": id,
			},
		})
		w.WriteHeader(http.StatusNoContent)
		return nil
	default:
		return &web.Error{Code: http.StatusMethodNotAllowed, Message: "Method not allowed"}
	}
}

func (h *Handler) handlePermissions(w http.ResponseWriter, r *http.Request, userID int64) *web.Error {
	switch r.Method {
	case http.MethodGet:
		permissions, err := h.service.GetUserPermissions(r.Context(), userID)
		if err != nil {
			return &web.Error{Code: http.StatusBadRequest, Message: "Failed to get permissions", Err: err}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(permissions)
		return nil
	case http.MethodPut:
		var req struct {
			Permissions []*UserSpacePermission `json:"permissions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return &web.Error{Code: http.StatusBadRequest, Message: "Invalid request body", Err: err}
		}
		for _, permission := range req.Permissions {
			permission.UserID = userID
		}
		if err := h.service.ReplaceUserPermissions(r.Context(), userID, req.Permissions); err != nil {
			h.recordAudit(r, audit.Event{
				Action: "account.permissions.replace",
				Result: audit.ResultFailure,
				Target: "user:" + strconv.FormatInt(userID, 10),
				Metadata: map[string]any{
					"userId": userID,
					"count":  len(req.Permissions),
					"reason": "replace_permissions_failed",
				},
			})
			return &web.Error{Code: http.StatusBadRequest, Message: "Failed to update permissions", Err: err}
		}
		spaceIDs := make([]int64, 0, len(req.Permissions))
		permissions := make([]string, 0, len(req.Permissions))
		for _, permission := range req.Permissions {
			spaceIDs = append(spaceIDs, permission.SpaceID)
			permissions = append(permissions, string(permission.Permission))
		}
		h.recordAudit(r, audit.Event{
			Action: "account.permissions.replace",
			Result: audit.ResultSuccess,
			Target: "user:" + strconv.FormatInt(userID, 10),
			Metadata: map[string]any{
				"userId":      userID,
				"count":       len(req.Permissions),
				"spaceIds":    spaceIDs,
				"permissions": permissions,
			},
		})
		w.WriteHeader(http.StatusNoContent)
		return nil
	default:
		return &web.Error{Code: http.StatusMethodNotAllowed, Message: "Method not allowed"}
	}
}

func (h *Handler) handleRoles(w http.ResponseWriter, r *http.Request) *web.Error {
	switch r.Method {
	case http.MethodGet:
		roles, err := h.service.ListRolesWithPermissions(r.Context())
		if err != nil {
			return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to list roles", Err: err}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(roles)
		return nil
	case http.MethodPost:
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return &web.Error{Code: http.StatusBadRequest, Message: "Invalid request body", Err: err}
		}
		role, err := h.service.CreateRole(r.Context(), req.Name, req.Description)
		if err != nil {
			h.recordAudit(r, audit.Event{
				Action: "role.create",
				Result: audit.ResultFailure,
				Target: req.Name,
				Metadata: map[string]any{
					"name":   req.Name,
					"reason": "create_role_failed",
				},
			})
			return &web.Error{Code: http.StatusBadRequest, Message: "Failed to create Role", Err: err}
		}
		h.recordAudit(r, audit.Event{
			Action: "role.create",
			Result: audit.ResultSuccess,
			Target: role.Name,
			Metadata: map[string]any{
				"name": role.Name,
			},
		})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(role)
		return nil
	default:
		return &web.Error{Code: http.StatusMethodNotAllowed, Message: "Method not allowed"}
	}
}

func (h *Handler) handleRoleByName(w http.ResponseWriter, r *http.Request) *web.Error {
	path := strings.TrimPrefix(r.URL.Path, "/api/roles/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		return &web.Error{Code: http.StatusBadRequest, Message: "Role name is required"}
	}
	roleName := parts[0]

	if len(parts) > 1 && parts[1] == "permissions" {
		if r.Method != http.MethodPut {
			return &web.Error{Code: http.StatusMethodNotAllowed, Message: "Method not allowed"}
		}
		var req struct {
			Permissions []string `json:"permissions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return &web.Error{Code: http.StatusBadRequest, Message: "Invalid request body", Err: err}
		}
		if err := h.service.ReplaceRolePermissions(r.Context(), roleName, req.Permissions); err != nil {
			h.recordAudit(r, audit.Event{
				Action: "role.permissions.replace",
				Result: audit.ResultFailure,
				Target: roleName,
				Metadata: map[string]any{
					"name":   roleName,
					"count":  len(req.Permissions),
					"reason": "replace_role_permissions_failed",
				},
			})
			return &web.Error{Code: http.StatusBadRequest, Message: "Failed to update Role permissions", Err: err}
		}
		h.recordAudit(r, audit.Event{
			Action: "role.permissions.replace",
			Result: audit.ResultSuccess,
			Target: roleName,
			Metadata: map[string]any{
				"name":        roleName,
				"count":       len(req.Permissions),
				"permissions": req.Permissions,
			},
		})
		w.WriteHeader(http.StatusNoContent)
		return nil
	}

	if r.Method != http.MethodDelete {
		return &web.Error{Code: http.StatusMethodNotAllowed, Message: "Method not allowed"}
	}
	if err := h.service.DeleteRole(r.Context(), roleName); err != nil {
		h.recordAudit(r, audit.Event{
			Action: "role.delete",
			Result: audit.ResultFailure,
			Target: roleName,
			Metadata: map[string]any{
				"name":   roleName,
				"reason": "delete_role_failed",
			},
		})
		return &web.Error{Code: http.StatusBadRequest, Message: "Failed to delete Role", Err: err}
	}
	h.recordAudit(r, audit.Event{
		Action: "role.delete",
		Result: audit.ResultSuccess,
		Target: roleName,
		Metadata: map[string]any{
			"name": roleName,
		},
	})
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) handlePermissionDefinitions(w http.ResponseWriter, r *http.Request) *web.Error {
	if r.Method != http.MethodGet {
		return &web.Error{Code: http.StatusMethodNotAllowed, Message: "Method not allowed"}
	}
	definitions, err := h.service.ListPermissionDefinitions(r.Context())
	if err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to list permissions", Err: err}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(definitions)
	return nil
}

func (h *Handler) handleSetupStatus(w http.ResponseWriter, r *http.Request) *web.Error {
	needsSetup, err := h.service.NeedsBootstrap(r.Context())
	if err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to check setup status", Err: err}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{
		"requiresSetup": needsSetup,
	})
	return nil
}

func (h *Handler) handleSetupAdmin(w http.ResponseWriter, r *http.Request) *web.Error {
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return &web.Error{Code: http.StatusBadRequest, Message: "Invalid request body", Err: err}
	}

	user, err := h.service.BootstrapInitialAdmin(r.Context(), &req)
	if err != nil {
		if errors.Is(err, ErrInitialSetupCompleted) {
			return &web.Error{Code: http.StatusConflict, Message: "Initial setup already completed", Err: err}
		}
		return &web.Error{Code: http.StatusBadRequest, Message: "Failed to setup admin account", Err: err}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(user)
	return nil
}

func (h *Handler) recordAudit(r *http.Request, event audit.Event) {
	if h.auditRecorder == nil {
		return
	}
	event.Actor = strings.TrimSpace(r.Header.Get("X-Cohesion-Actor"))
	if event.RequestID == "" {
		event.RequestID = strings.TrimSpace(r.Header.Get("X-Request-Id"))
	}
	h.auditRecorder.RecordBestEffort(event)
}

func changedUserFields(req *UpdateUserRequest) []string {
	fields := make([]string, 0, 3)
	if req == nil {
		return fields
	}
	if req.Nickname != nil {
		fields = append(fields, "nickname")
	}
	if req.Password != nil {
		fields = append(fields, "password")
	}
	if req.Role != nil {
		fields = append(fields, "role")
	}
	return fields
}
