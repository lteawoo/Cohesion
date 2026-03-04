package account

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"

	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
	"taeu.kr/cohesion/internal/platform/logging"
)

type Storer interface {
	ListUsers(ctx context.Context) ([]*User, error)
	GetUserByID(ctx context.Context, id int64) (*User, error)
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	CreateUser(ctx context.Context, req *CreateUserRequest, passwordHash, smbMaterial string, materialVersion int) (*User, error)
	UpdateUser(ctx context.Context, id int64, req *UpdateUserRequest, passwordHash *string, smbMaterial *string, materialVersion int) (*User, error)
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
	UpsertSMBCredential(ctx context.Context, userID int64, smbMaterial string, materialVersion int) error
	GetSMBCredential(ctx context.Context, userID int64) (*SMBCredential, error)
}

const (
	smbMaterialVersion       = 4
	legacySMBMaterialVersion = 3
	defaultSMBMaterialKey    = "cohesion-dev-smb-material-key"

	smbMaterialKeySourceEnv           = "env:COHESION_SMB_MATERIAL_KEY"
	smbMaterialKeySourceFallback      = "fallback:development-default"
	smbMaterialKeySourceLegacyJWT     = "legacy:COHESION_JWT_SECRET"
	smbMaterialKeySourceLegacyDefault = "legacy:development-default"
)

type Service struct {
	store Storer
}

type smbMaterialSecret struct {
	value  string
	source string
	legacy bool
}

var (
	ErrInitialSetupCompleted          = errors.New("initial setup already completed")
	ErrSMBCredentialRecoveryRequired  = errors.New("smb credential recovery required")
	requireExplicitSMBMaterialKeyFlag atomic.Bool
)

func SetSMBMaterialKeyRequired(required bool) {
	requireExplicitSMBMaterialKeyFlag.Store(required)
}

func IsSMBMaterialKeyRequired() bool {
	return requireExplicitSMBMaterialKeyFlag.Load()
}

func ValidateSMBMaterialKeyConfiguration() error {
	_, err := resolvePrimarySMBMaterialSecret()
	return err
}

func CurrentSMBMaterialKeySource() string {
	secret, err := resolvePrimarySMBMaterialSecret()
	if err != nil {
		return "unavailable"
	}
	return secret.source
}

func NewService(store Storer) *Service {
	return &Service{store: store}
}

func (s *Service) EnsureDefaultAdmin(ctx context.Context) error {
	needsBootstrap, err := s.NeedsBootstrap(ctx)
	if err != nil {
		return err
	}
	if !needsBootstrap {
		return nil
	}

	username := strings.TrimSpace(os.Getenv("COHESION_ADMIN_USER"))
	password := strings.TrimSpace(os.Getenv("COHESION_ADMIN_PASSWORD"))
	nickname := strings.TrimSpace(os.Getenv("COHESION_ADMIN_NICKNAME"))

	// For consumer UX, do not create weak default credentials automatically.
	// Bootstrap remains possible via one-time setup API or explicit env vars.
	if username == "" && password == "" {
		return nil
	}
	if username == "" || password == "" {
		return errors.New("COHESION_ADMIN_USER and COHESION_ADMIN_PASSWORD must be set together")
	}

	_, err = s.BootstrapInitialAdmin(ctx, &CreateUserRequest{
		Username: username,
		Password: password,
		Nickname: nickname,
	})
	if errors.Is(err, ErrInitialSetupCompleted) {
		return nil
	}
	return err
}

func (s *Service) NeedsBootstrap(ctx context.Context) (bool, error) {
	adminCount, err := s.store.CountAdmins(ctx)
	if err != nil {
		return false, err
	}
	return adminCount == 0, nil
}

func (s *Service) BootstrapInitialAdmin(ctx context.Context, req *CreateUserRequest) (*User, error) {
	if req == nil {
		return nil, errors.New("request is required")
	}

	needsBootstrap, err := s.NeedsBootstrap(ctx)
	if err != nil {
		return nil, err
	}
	if !needsBootstrap {
		return nil, ErrInitialSetupCompleted
	}

	nickname := strings.TrimSpace(req.Nickname)
	if nickname == "" {
		nickname = "Administrator"
	}

	return s.CreateUser(ctx, &CreateUserRequest{
		Username: strings.TrimSpace(req.Username),
		Password: strings.TrimSpace(req.Password),
		Nickname: nickname,
		Role:     RoleAdmin,
	})
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
	material, err := deriveSMBMaterial(req.Username, req.Password)
	if err != nil {
		return nil, err
	}
	return s.store.CreateUser(ctx, req, hash, material, smbMaterialVersion)
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
	var rawPassword *string
	var smbMaterial *string
	if req.Password != nil {
		raw := *req.Password
		if len(strings.TrimSpace(raw)) < 6 {
			return nil, errors.New("password must be at least 6 characters")
		}
		hash, err := hashPassword(raw)
		if err != nil {
			return nil, err
		}
		passwordHash = &hash
		rawPassword = &raw
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

	if rawPassword != nil {
		material, err := deriveSMBMaterial(current.Username, *rawPassword)
		if err != nil {
			return nil, err
		}
		smbMaterial = &material
	}

	return s.store.UpdateUser(ctx, id, req, passwordHash, smbMaterial, smbMaterialVersion)
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

func (s *Service) PrepareSMBCredential(ctx context.Context, username, password string) error {
	trimmedUsername := strings.TrimSpace(username)
	if trimmedUsername == "" || password == "" {
		return errors.New("username and password are required")
	}

	user, err := s.store.GetUserByUsername(ctx, trimmedUsername)
	if err != nil {
		return err
	}
	if err := s.upsertSMBCredential(ctx, user, password); err != nil {
		return wrapSMBCredentialRecoveryError(err)
	}
	return nil
}

func (s *Service) GetSMBCredential(ctx context.Context, userID int64) (*SMBCredential, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}
	return s.store.GetSMBCredential(ctx, userID)
}

func (s *Service) ResolveSMBPassword(ctx context.Context, username string) (string, error) {
	trimmed := strings.TrimSpace(username)
	if trimmed == "" {
		return "", errors.New("username is required")
	}

	user, err := s.store.GetUserByUsername(ctx, trimmed)
	if err != nil {
		return "", err
	}

	credential, err := s.store.GetSMBCredential(ctx, user.ID)
	if err != nil {
		return "", err
	}

	password, requiresMigration, err := decodeSMBMaterial(credential.SMBMaterial, credential.MaterialVersion)
	if err != nil {
		logging.Event(log.Warn(), logging.ComponentAuth, "warn.smb.material.decode_failed").
			Err(err).
			Str("username", user.Username).
			Int64("user_id", user.ID).
			Int("material_version", credential.MaterialVersion).
			Msg("[SMB] failed to decode credential material")
		return "", wrapSMBCredentialRecoveryError(err)
	}

	if requiresMigration {
		if err := s.upsertSMBCredential(ctx, user, password); err != nil {
			logging.Event(log.Warn(), logging.ComponentAuth, "warn.smb.material.migration_failed").
				Err(err).
				Str("username", user.Username).
				Int64("user_id", user.ID).
				Int("material_version", credential.MaterialVersion).
				Msg("[SMB] failed to migrate credential material")
			return "", wrapSMBCredentialRecoveryError(err)
		}
		logging.Event(log.Info(), logging.ComponentAuth, "info.smb.material.migrated").
			Str("username", user.Username).
			Int64("user_id", user.ID).
			Int("previous_material_version", credential.MaterialVersion).
			Msg("[SMB] credential material migrated to current policy")
	}

	return password, nil
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

func (s *Service) upsertSMBCredential(ctx context.Context, user *User, password string) error {
	if user == nil {
		return errors.New("user is required")
	}
	if password == "" {
		return errors.New("password is required")
	}

	material, err := deriveSMBMaterial(user.Username, password)
	if err != nil {
		return err
	}
	return s.store.UpsertSMBCredential(ctx, user.ID, material, smbMaterialVersion)
}

func deriveSMBMaterial(username, password string) (string, error) {
	_ = username
	if password == "" {
		return "", errors.New("password is required")
	}
	secret, err := resolvePrimarySMBMaterialSecret()
	if err != nil {
		return "", err
	}
	aead, err := newSMBMaterialAEAD(secret.value)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate smb material nonce: %w", err)
	}
	ciphertext := aead.Seal(nil, nonce, []byte(password), nil)
	return "enc:" + base64.StdEncoding.EncodeToString(nonce) + ":" + base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decodeSMBMaterial(material string, version int) (string, bool, error) {
	switch version {
	case smbMaterialVersion:
		return decodeEncryptedSMBMaterial(material)
	case legacySMBMaterialVersion:
		password, err := decodeLegacySMBMaterial(material)
		if err != nil {
			return "", false, err
		}
		return password, true, nil
	default:
		return "", false, fmt.Errorf("unsupported smb material version: %d", version)
	}
}

func decodeLegacySMBMaterial(material string) (string, error) {
	payload := strings.TrimSpace(material)
	if !strings.HasPrefix(payload, "plain:") {
		return "", errors.New("invalid smb material format")
	}

	encoded := strings.TrimPrefix(payload, "plain:")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("invalid smb material encoding: %w", err)
	}
	password := string(decoded)
	if password == "" {
		return "", errors.New("empty smb password material")
	}
	return password, nil
}

func decodeEncryptedSMBMaterial(material string) (string, bool, error) {
	payload := strings.TrimSpace(material)
	parts := strings.Split(payload, ":")
	if len(parts) != 3 || parts[0] != "enc" {
		return "", false, errors.New("invalid smb material format")
	}

	nonce, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", false, fmt.Errorf("invalid smb material nonce: %w", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return "", false, fmt.Errorf("invalid smb material ciphertext: %w", err)
	}

	secrets, err := resolveSMBMaterialDecryptionSecrets()
	if err != nil {
		return "", false, err
	}

	var lastErr error
	for _, secret := range secrets {
		aead, err := newSMBMaterialAEAD(secret.value)
		if err != nil {
			return "", false, err
		}
		if len(nonce) != aead.NonceSize() {
			return "", false, errors.New("invalid smb material nonce size")
		}
		plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
		if err != nil {
			lastErr = err
			continue
		}

		password := string(plaintext)
		if password == "" {
			return "", false, errors.New("empty smb password material")
		}
		return password, secret.legacy, nil
	}

	if lastErr == nil {
		lastErr = errors.New("failed to decrypt smb material")
	}
	return "", false, fmt.Errorf("failed to decrypt smb material: %w", lastErr)
}

func resolvePrimarySMBMaterialSecret() (smbMaterialSecret, error) {
	secret := strings.TrimSpace(os.Getenv("COHESION_SMB_MATERIAL_KEY"))
	if secret != "" {
		return smbMaterialSecret{
			value:  secret,
			source: smbMaterialKeySourceEnv,
		}, nil
	}
	if IsSMBMaterialKeyRequired() {
		return smbMaterialSecret{}, errors.New("COHESION_SMB_MATERIAL_KEY is required when SMB is enabled in production")
	}
	return smbMaterialSecret{
		value:  defaultSMBMaterialKey,
		source: smbMaterialKeySourceFallback,
	}, nil
}

func resolveSMBMaterialDecryptionSecrets() ([]smbMaterialSecret, error) {
	primary, err := resolvePrimarySMBMaterialSecret()
	if err != nil {
		return nil, err
	}
	secrets := []smbMaterialSecret{primary}

	legacyJWT := strings.TrimSpace(os.Getenv("COHESION_JWT_SECRET"))
	if legacyJWT != "" && legacyJWT != primary.value {
		secrets = append(secrets, smbMaterialSecret{
			value:  legacyJWT,
			source: smbMaterialKeySourceLegacyJWT,
			legacy: true,
		})
	}
	if defaultSMBMaterialKey != primary.value {
		secrets = append(secrets, smbMaterialSecret{
			value:  defaultSMBMaterialKey,
			source: smbMaterialKeySourceLegacyDefault,
			legacy: true,
		})
	}
	return secrets, nil
}

func newSMBMaterialAEAD(secret string) (cipher.AEAD, error) {
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("failed to initialize smb material cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize smb material gcm: %w", err)
	}
	return aead, nil
}

func wrapSMBCredentialRecoveryError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrSMBCredentialRecoveryRequired) {
		return err
	}
	return fmt.Errorf("%w: %v", ErrSMBCredentialRecoveryRequired, err)
}
