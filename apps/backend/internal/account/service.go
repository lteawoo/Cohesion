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
	"io/fs"
	"os"
	"path/filepath"
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
	HasAnySMBCredential(ctx context.Context) (bool, error)
}

const (
	smbMaterialVersion = 4

	smbMaterialKeySourceEnv           = "env:COHESION_SMB_MATERIAL_KEY"
	smbMaterialKeySourceFileExisting  = "file:existing"
	smbMaterialKeySourceFileGenerated = "file:generated"

	smbMaterialKeyPathEnv        = "COHESION_SMB_MATERIAL_KEY_FILE"
	defaultSMBMaterialKeyFile    = "smb_material_key"
	smbMaterialKeyDirPermission  = 0o700
	smbMaterialKeyFilePermission = 0o600
)

type Service struct {
	store Storer
}

type smbMaterialSecret struct {
	value  string
	source string
}

type SMBMaterialKeyPrewarmResult struct {
	Source string
	Path   string
}

var (
	ErrInitialSetupCompleted          = errors.New("initial setup already completed")
	ErrSMBCredentialRecoveryRequired  = errors.New("smb credential recovery required")
	errSMBMaterialKeyMissing          = errors.New("smb material key missing")
	requireExplicitSMBMaterialKeyFlag atomic.Bool
)

func SetSMBMaterialKeyRequired(required bool) {
	requireExplicitSMBMaterialKeyFlag.Store(required)
}

func IsSMBMaterialKeyRequired() bool {
	return requireExplicitSMBMaterialKeyFlag.Load()
}

func ValidateSMBMaterialKeyConfiguration() error {
	_, err := resolvePrimarySMBMaterialSecretWithoutBootstrap()
	return err
}

func CurrentSMBMaterialKeySource() string {
	secret, err := resolvePrimarySMBMaterialSecretWithoutBootstrap()
	if err != nil {
		return "unavailable"
	}
	return secret.source
}

func (s *Service) ValidateSMBMaterialKeyConfiguration(ctx context.Context) (string, error) {
	secret, err := s.resolvePrimarySMBMaterialSecret(ctx)
	if err != nil {
		return "", err
	}
	return secret.source, nil
}

func (s *Service) PrewarmSMBMaterialKey(ctx context.Context) (SMBMaterialKeyPrewarmResult, error) {
	if s == nil {
		return SMBMaterialKeyPrewarmResult{}, errors.New("account service is required")
	}

	if secret := strings.TrimSpace(os.Getenv("COHESION_SMB_MATERIAL_KEY")); secret != "" {
		return SMBMaterialKeyPrewarmResult{Source: "env"}, nil
	}

	path, err := resolveSMBMaterialKeyPath()
	if err != nil {
		return SMBMaterialKeyPrewarmResult{}, err
	}

	secret, exists, err := readSMBMaterialSecretFromFile(path)
	if err != nil {
		return SMBMaterialKeyPrewarmResult{}, err
	}
	if exists && strings.TrimSpace(secret) != "" {
		return SMBMaterialKeyPrewarmResult{Source: "file", Path: path}, nil
	}

	hasCredentials, err := s.hasAnySMBCredential(ctx)
	if err != nil {
		return SMBMaterialKeyPrewarmResult{}, fmt.Errorf("check existing smb credentials: %w", err)
	}
	if hasCredentials {
		return SMBMaterialKeyPrewarmResult{}, missingSMBMaterialKeyWithCredentialDataError()
	}

	generated, err := generateRandomSMBMaterialSecret(48)
	if err != nil {
		return SMBMaterialKeyPrewarmResult{}, err
	}
	if err := writeSMBMaterialSecretToFile(path, generated); err != nil {
		return SMBMaterialKeyPrewarmResult{}, err
	}

	logging.Event(log.Info(), logging.ComponentAuth, "info.smb.material_key_prewarmed").
		Str("source", smbMaterialKeySourceFileGenerated).
		Str("path", path).
		Msg("[SMB] material key prewarmed from generated secret file")

	return SMBMaterialKeyPrewarmResult{Source: "generated", Path: path}, nil
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
	material, err := s.deriveSMBMaterial(ctx, req.Username, req.Password)
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
		material, err := s.deriveSMBMaterial(ctx, current.Username, *rawPassword)
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

	password, err := s.decodeSMBMaterial(ctx, credential.SMBMaterial, credential.MaterialVersion)
	if err != nil {
		logging.Event(log.Warn(), logging.ComponentAuth, "warn.smb.material.decode_failed").
			Err(err).
			Str("username", user.Username).
			Int64("user_id", user.ID).
			Int("material_version", credential.MaterialVersion).
			Msg("[SMB] failed to decode credential material")
		return "", wrapSMBCredentialRecoveryError(err)
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

	material, err := s.deriveSMBMaterial(ctx, user.Username, password)
	if err != nil {
		return err
	}
	return s.store.UpsertSMBCredential(ctx, user.ID, material, smbMaterialVersion)
}

func (s *Service) deriveSMBMaterial(ctx context.Context, username, password string) (string, error) {
	_ = username
	if password == "" {
		return "", errors.New("password is required")
	}
	return s.encodeSMBMaterial(ctx, password)
}

func (s *Service) encodeSMBMaterial(ctx context.Context, password string) (string, error) {
	if password == "" {
		return "", errors.New("password is required")
	}
	secret, err := s.resolvePrimarySMBMaterialSecret(ctx)
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

func (s *Service) decodeSMBMaterial(ctx context.Context, material string, version int) (string, error) {
	if version != smbMaterialVersion {
		return "", fmt.Errorf("unsupported smb material version: %d", version)
	}
	return s.decodeEncryptedSMBMaterial(ctx, material)
}

func (s *Service) decodeEncryptedSMBMaterial(ctx context.Context, material string) (string, error) {
	payload := strings.TrimSpace(material)
	parts := strings.Split(payload, ":")
	if len(parts) != 3 || parts[0] != "enc" {
		return "", errors.New("invalid smb material format")
	}

	nonce, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("invalid smb material nonce: %w", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return "", fmt.Errorf("invalid smb material ciphertext: %w", err)
	}

	secret, err := s.resolvePrimarySMBMaterialSecret(ctx)
	if err != nil {
		return "", err
	}
	aead, err := newSMBMaterialAEAD(secret.value)
	if err != nil {
		return "", err
	}
	if len(nonce) != aead.NonceSize() {
		return "", errors.New("invalid smb material nonce size")
	}
	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt smb material: %w", err)
	}

	password := string(plaintext)
	if password == "" {
		return "", errors.New("empty smb password material")
	}
	return password, nil
}

func resolvePrimarySMBMaterialSecretWithoutBootstrap() (smbMaterialSecret, error) {
	secret := strings.TrimSpace(os.Getenv("COHESION_SMB_MATERIAL_KEY"))
	if secret != "" {
		return smbMaterialSecret{
			value:  secret,
			source: smbMaterialKeySourceEnv,
		}, nil
	}

	path, err := resolveSMBMaterialKeyPath()
	if err != nil {
		return smbMaterialSecret{}, err
	}

	secret, exists, err := readSMBMaterialSecretFromFile(path)
	if err != nil {
		return smbMaterialSecret{}, err
	}
	if exists {
		return smbMaterialSecret{
			value:  secret,
			source: smbMaterialKeySourceFileExisting,
		}, nil
	}

	return smbMaterialSecret{}, missingSMBMaterialKeyError()
}

func (s *Service) resolvePrimarySMBMaterialSecret(ctx context.Context) (smbMaterialSecret, error) {
	secret, err := resolvePrimarySMBMaterialSecretWithoutBootstrap()
	if err == nil {
		return secret, nil
	}
	if !errors.Is(err, errSMBMaterialKeyMissing) {
		return smbMaterialSecret{}, err
	}

	hasCredentials, err := s.hasAnySMBCredential(ctx)
	if err != nil {
		return smbMaterialSecret{}, fmt.Errorf("check existing smb credentials: %w", err)
	}
	if hasCredentials {
		return smbMaterialSecret{}, missingSMBMaterialKeyWithCredentialDataError()
	}

	if IsSMBMaterialKeyRequired() {
		return smbMaterialSecret{}, missingSMBMaterialKeyError()
	}

	return smbMaterialSecret{}, missingSMBMaterialKeyPrewarmRequiredError()
}

func (s *Service) hasAnySMBCredential(ctx context.Context) (bool, error) {
	if s == nil || s.store == nil {
		return false, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return s.store.HasAnySMBCredential(ctx)
}

func resolveSMBMaterialKeyPath() (string, error) {
	if customPath := strings.TrimSpace(os.Getenv(smbMaterialKeyPathEnv)); customPath != "" {
		return customPath, nil
	}

	userConfigDir, err := os.UserConfigDir()
	if err == nil && strings.TrimSpace(userConfigDir) != "" {
		return filepath.Join(userConfigDir, "Cohesion", "secrets", defaultSMBMaterialKeyFile), nil
	}

	executablePath, err := os.Executable()
	if err != nil {
		return "", errors.New("failed to resolve smb material key path")
	}
	return filepath.Join(filepath.Dir(executablePath), "data", defaultSMBMaterialKeyFile), nil
}

func readSMBMaterialSecretFromFile(path string) (string, bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}
	secret := strings.TrimSpace(string(content))
	if secret == "" {
		return "", false, nil
	}
	return secret, true, nil
}

func writeSMBMaterialSecretToFile(path, secret string) error {
	if err := os.MkdirAll(filepath.Dir(path), smbMaterialKeyDirPermission); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(secret+"\n"), smbMaterialKeyFilePermission)
}

func generateRandomSMBMaterialSecret(size int) (string, error) {
	if size < 32 {
		return "", errors.New("secret size must be at least 32 bytes")
	}
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func missingSMBMaterialKeyError() error {
	return fmt.Errorf("%w: set COHESION_SMB_MATERIAL_KEY or COHESION_SMB_MATERIAL_KEY_FILE", errSMBMaterialKeyMissing)
}

func missingSMBMaterialKeyPrewarmRequiredError() error {
	return fmt.Errorf("%w: run startup prewarm before handling SMB credentials", errSMBMaterialKeyMissing)
}

func missingSMBMaterialKeyWithCredentialDataError() error {
	return fmt.Errorf("%w: SMB material key is missing while existing SMB credential data is present; restore COHESION_SMB_MATERIAL_KEY or COHESION_SMB_MATERIAL_KEY_FILE from backup", ErrSMBCredentialRecoveryRequired)
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
