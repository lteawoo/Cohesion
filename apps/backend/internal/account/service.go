package account

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

type Storer interface {
	ListUsers(ctx context.Context) ([]*User, error)
	GetUserByID(ctx context.Context, id int64) (*User, error)
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	CreateUser(ctx context.Context, req *CreateUserRequest, passwordHash string) (*User, error)
	UpdateUser(ctx context.Context, id int64, req *UpdateUserRequest, passwordHash *string) (*User, error)
	DeleteUser(ctx context.Context, id int64) error
	CountAdmins(ctx context.Context) (int, error)
	GetUserPermissions(ctx context.Context, userID int64) ([]*UserSpacePermission, error)
	ReplaceUserPermissions(ctx context.Context, userID int64, permissions []*UserSpacePermission) error
	ListRoles(ctx context.Context) ([]*RoleDefinition, error)
	GetRoleByName(ctx context.Context, name string) (*RoleDefinition, error)
	CreateRole(ctx context.Context, name, description string) (*RoleDefinition, error)
	DeleteRole(ctx context.Context, name string) error
	CountUsersByRole(ctx context.Context, roleName string) (int, error)
	ListPermissionDefinitions(ctx context.Context) ([]*PermissionDefinition, error)
	GetRolePermissionKeys(ctx context.Context, roleName string) ([]string, error)
	ReplaceRolePermissionKeys(ctx context.Context, roleName string, permissionKeys []string) error
}

type Service struct {
	store Storer
}

func NewService(store Storer) *Service {
	return &Service{store: store}
}

func (s *Service) EnsureDefaultAdmin(ctx context.Context) error {
	adminCount, err := s.store.CountAdmins(ctx)
	if err != nil {
		return err
	}
	if adminCount > 0 {
		return nil
	}

	username := strings.TrimSpace(os.Getenv("COHESION_ADMIN_USER"))
	if username == "" {
		username = "admin"
	}
	password := os.Getenv("COHESION_ADMIN_PASSWORD")
	if strings.TrimSpace(password) == "" {
		password = "admin1234"
	}
	nickname := strings.TrimSpace(os.Getenv("COHESION_ADMIN_NICKNAME"))
	if nickname == "" {
		nickname = "Administrator"
	}

	if existing, err := s.store.GetUserByUsername(ctx, username); err == nil {
		if existing.Role == RoleAdmin {
			return nil
		}
		return errors.New("initial admin username already exists with non-admin role")
	}

	_, err = s.CreateUser(ctx, &CreateUserRequest{
		Username: username,
		Password: password,
		Nickname: nickname,
		Role:     RoleAdmin,
	})
	return err
}

func (s *Service) ListUsers(ctx context.Context) ([]*User, error) {
	return s.store.ListUsers(ctx)
}

func (s *Service) GetUserByID(ctx context.Context, id int64) (*User, error) {
	user, err := s.store.GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}
	user.PasswordHash = ""
	return user, nil
}

func (s *Service) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	user, err := s.store.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	user.PasswordHash = ""
	return user, nil
}

func (s *Service) CreateUser(ctx context.Context, req *CreateUserRequest) (*User, error) {
	if err := validateCreateUser(req); err != nil {
		return nil, err
	}
	if _, err := s.store.GetRoleByName(ctx, string(req.Role)); err != nil {
		return nil, errors.New("Role does not exist")
	}

	hash, err := hashPassword(req.Password)
	if err != nil {
		return nil, err
	}
	return s.store.CreateUser(ctx, req, hash)
}

func (s *Service) UpdateUser(ctx context.Context, id int64, req *UpdateUserRequest) (*User, error) {
	if id <= 0 {
		return nil, errors.New("invalid user id")
	}
	if req == nil {
		return nil, errors.New("request is required")
	}

	if req.Role != nil {
		if _, err := s.store.GetRoleByName(ctx, string(*req.Role)); err != nil {
			return nil, errors.New("Role does not exist")
		}
	}
	if req.Nickname != nil {
		trimmed := strings.TrimSpace(*req.Nickname)
		if trimmed == "" {
			return nil, errors.New("nickname is required")
		}
		req.Nickname = &trimmed
	}

	var passwordHash *string
	if req.Password != nil {
		trimmed := strings.TrimSpace(*req.Password)
		if len(trimmed) < 6 {
			return nil, errors.New("password must be at least 6 characters")
		}
		hash, err := hashPassword(trimmed)
		if err != nil {
			return nil, err
		}
		passwordHash = &hash
	}

	current, err := s.store.GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if current.Role == RoleAdmin && req.Role != nil && *req.Role == RoleUser {
		adminCount, err := s.store.CountAdmins(ctx)
		if err != nil {
			return nil, err
		}
		if adminCount <= 1 {
			return nil, errors.New("at least one admin user must remain")
		}
	}

	return s.store.UpdateUser(ctx, id, req, passwordHash)
}

func (s *Service) DeleteUser(ctx context.Context, id int64) error {
	user, err := s.store.GetUserByID(ctx, id)
	if err != nil {
		return err
	}
	if user.Role == RoleAdmin {
		adminCount, err := s.store.CountAdmins(ctx)
		if err != nil {
			return err
		}
		if adminCount <= 1 {
			return errors.New("at least one admin user must remain")
		}
	}
	return s.store.DeleteUser(ctx, id)
}

func (s *Service) Authenticate(ctx context.Context, username, password string) (bool, error) {
	user, err := s.store.GetUserByUsername(ctx, username)
	if err != nil {
		return false, nil
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return false, nil
	}
	return true, nil
}

func (s *Service) GetUserPermissions(ctx context.Context, userID int64) ([]*UserSpacePermission, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}
	return s.store.GetUserPermissions(ctx, userID)
}

func (s *Service) ReplaceUserPermissions(ctx context.Context, userID int64, permissions []*UserSpacePermission) error {
	if userID <= 0 {
		return errors.New("invalid user id")
	}
	if _, err := s.store.GetUserByID(ctx, userID); err != nil {
		return err
	}
	for _, permission := range permissions {
		if permission.UserID != userID {
			return errors.New("userId mismatch")
		}
		switch permission.Permission {
		case PermissionRead, PermissionWrite, PermissionManage:
		default:
			return errors.New("permission must be read, write, or manage")
		}
	}
	return s.store.ReplaceUserPermissions(ctx, userID, permissions)
}

func (s *Service) CanAccessSpaceByID(ctx context.Context, username string, spaceID int64, required Permission) (bool, error) {
	user, err := s.store.GetUserByUsername(ctx, username)
	if err != nil {
		return false, nil
	}
	if user.Role == RoleAdmin {
		return true, nil
	}

	permissions, err := s.store.GetUserPermissions(ctx, user.ID)
	if err != nil {
		return false, err
	}
	for _, permission := range permissions {
		if permission.SpaceID == spaceID && permission.Permission.Allows(required) {
			return true, nil
		}
	}
	return false, nil
}

func (s *Service) IsAdmin(ctx context.Context, username string) (bool, error) {
	user, err := s.store.GetUserByUsername(ctx, username)
	if err != nil {
		return false, nil
	}
	return user.Role == RoleAdmin, nil
}

func (s *Service) ListRolesWithPermissions(ctx context.Context) ([]*RoleWithPermissions, error) {
	roles, err := s.store.ListRoles(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*RoleWithPermissions, 0, len(roles))
	for _, role := range roles {
		keys, err := s.store.GetRolePermissionKeys(ctx, role.Name)
		if err != nil {
			return nil, err
		}
		result = append(result, &RoleWithPermissions{
			Name:        role.Name,
			Description: role.Description,
			IsSystem:    role.IsSystem,
			Permissions: keys,
		})
	}
	return result, nil
}

func (s *Service) ListPermissionDefinitions(ctx context.Context) ([]*PermissionDefinition, error) {
	return s.store.ListPermissionDefinitions(ctx)
}

func (s *Service) CreateRole(ctx context.Context, name, description string) (*RoleDefinition, error) {
	normalizedName := strings.TrimSpace(strings.ToLower(name))
	if normalizedName == "" {
		return nil, errors.New("Role name is required")
	}
	if len(normalizedName) < 2 || len(normalizedName) > 32 {
		return nil, errors.New("Role name must be between 2 and 32 characters")
	}
	for _, ch := range normalizedName {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-' {
			continue
		}
		return nil, errors.New("Role name can only contain lowercase letters, numbers, '-' and '_'")
	}

	return s.store.CreateRole(ctx, normalizedName, strings.TrimSpace(description))
}

func (s *Service) DeleteRole(ctx context.Context, name string) error {
	role, err := s.store.GetRoleByName(ctx, name)
	if err != nil {
		return err
	}
	if role.IsSystem {
		return errors.New("System Role cannot be deleted")
	}
	assignedCount, err := s.store.CountUsersByRole(ctx, name)
	if err != nil {
		return err
	}
	if assignedCount > 0 {
		return errors.New("Role is assigned to users and cannot be deleted")
	}
	return s.store.DeleteRole(ctx, name)
}

func (s *Service) ReplaceRolePermissions(ctx context.Context, roleName string, permissionKeys []string) error {
	if _, err := s.store.GetRoleByName(ctx, roleName); err != nil {
		return errors.New("Role does not exist")
	}
	permissionDefs, err := s.store.ListPermissionDefinitions(ctx)
	if err != nil {
		return err
	}
	allowed := make(map[string]struct{}, len(permissionDefs))
	for _, definition := range permissionDefs {
		allowed[definition.Key] = struct{}{}
	}

	seen := make(map[string]struct{}, len(permissionKeys))
	normalized := make([]string, 0, len(permissionKeys))
	for _, key := range permissionKeys {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		if _, ok := allowed[trimmed]; !ok {
			return errors.New("invalid permission key")
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}

	role, err := s.store.GetRoleByName(ctx, roleName)
	if err != nil {
		return err
	}
	if role.IsSystem && len(normalized) == 0 {
		return errors.New("System Role must keep at least one permission")
	}

	return s.store.ReplaceRolePermissionKeys(ctx, roleName, normalized)
}

func (s *Service) GetRolePermissionKeys(ctx context.Context, roleName string) ([]string, error) {
	return s.store.GetRolePermissionKeys(ctx, roleName)
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

func validateCreateUser(req *CreateUserRequest) error {
	if req == nil {
		return errors.New("request is required")
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Nickname = strings.TrimSpace(req.Nickname)
	if req.Username == "" {
		return errors.New("username is required")
	}
	if len(req.Username) < 3 {
		return errors.New("username must be at least 3 characters")
	}
	if req.Nickname == "" {
		return errors.New("nickname is required")
	}
	if len(strings.TrimSpace(req.Password)) < 6 {
		return errors.New("password must be at least 6 characters")
	}
	if req.Role == "" {
		req.Role = RoleUser
	}
	return nil
}
