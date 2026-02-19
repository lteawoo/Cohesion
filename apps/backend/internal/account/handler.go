package account

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"taeu.kr/cohesion/internal/platform/web"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("/api/accounts", web.Handler(h.handleAccounts))
	mux.Handle("/api/accounts/", web.Handler(h.handleAccountByID))
	mux.Handle("/api/roles", web.Handler(h.handleRoles))
	mux.Handle("/api/roles/", web.Handler(h.handleRoleByName))
	mux.Handle("/api/permissions", web.Handler(h.handlePermissionDefinitions))
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
			return &web.Error{Code: http.StatusBadRequest, Message: "Failed to create user", Err: err}
		}
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
			return &web.Error{Code: http.StatusBadRequest, Message: "Failed to update user", Err: err}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(user)
		return nil
	case http.MethodDelete:
		if err := h.service.DeleteUser(r.Context(), id); err != nil {
			return &web.Error{Code: http.StatusBadRequest, Message: "Failed to delete user", Err: err}
		}
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
			return &web.Error{Code: http.StatusBadRequest, Message: "Failed to update permissions", Err: err}
		}
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
			return &web.Error{Code: http.StatusBadRequest, Message: "Failed to create Role", Err: err}
		}
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
			return &web.Error{Code: http.StatusBadRequest, Message: "Failed to update Role permissions", Err: err}
		}
		w.WriteHeader(http.StatusNoContent)
		return nil
	}

	if r.Method != http.MethodDelete {
		return &web.Error{Code: http.StatusMethodNotAllowed, Message: "Method not allowed"}
	}
	if err := h.service.DeleteRole(r.Context(), roleName); err != nil {
		return &web.Error{Code: http.StatusBadRequest, Message: "Failed to delete Role", Err: err}
	}
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
