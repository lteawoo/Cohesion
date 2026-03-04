package account_test

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"testing"

	"taeu.kr/cohesion/internal/account"
)

func TestCreateAndUpdateUser_RefreshesSMBCredentialMaterial(t *testing.T) {
	account.SetSMBMaterialKeyRequired(false)
	t.Cleanup(func() {
		account.SetSMBMaterialKeyRequired(false)
	})
	t.Setenv("COHESION_SMB_MATERIAL_KEY", "test-smb-key")

	svc, db := setupRBACService(t)
	defer db.Close()

	ctx := context.Background()
	user, err := svc.CreateUser(ctx, &account.CreateUserRequest{
		Username: "smb-credential-user",
		Password: "first-password",
		Nickname: "SMB Credential User",
		Role:     account.RoleUser,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	before, err := svc.GetSMBCredential(ctx, user.ID)
	if err != nil {
		t.Fatalf("get smb credential after create: %v", err)
	}
	if before.SMBMaterial == "" {
		t.Fatal("expected smb material after create")
	}

	newPassword := "second-password"
	updated, err := svc.UpdateUser(ctx, user.ID, &account.UpdateUserRequest{
		Password: &newPassword,
	})
	if err != nil {
		t.Fatalf("update user password: %v", err)
	}

	after, err := svc.GetSMBCredential(ctx, updated.ID)
	if err != nil {
		t.Fatalf("get smb credential after update: %v", err)
	}
	if after.SMBMaterial == "" {
		t.Fatal("expected smb material after update")
	}
	if before.SMBMaterial == after.SMBMaterial {
		t.Fatal("expected smb material to change when password changes")
	}
	if after.MaterialVersion != 4 {
		t.Fatalf("expected smb material version 4, got %d", after.MaterialVersion)
	}
}

func TestResolveSMBPassword_MigratesLegacyMaterialVersion(t *testing.T) {
	account.SetSMBMaterialKeyRequired(false)
	t.Cleanup(func() {
		account.SetSMBMaterialKeyRequired(false)
	})
	t.Setenv("COHESION_SMB_MATERIAL_KEY", "migration-key")

	svc, db := setupRBACService(t)
	defer db.Close()

	ctx := context.Background()
	password := "legacy-password"
	user, err := svc.CreateUser(ctx, &account.CreateUserRequest{
		Username: "legacy-user",
		Password: password,
		Nickname: "Legacy User",
		Role:     account.RoleUser,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	legacyMaterial := "plain:" + base64.StdEncoding.EncodeToString([]byte(password))
	if _, err := db.ExecContext(
		ctx,
		"UPDATE user_smb_credentials SET smb_material = ?, material_version = ? WHERE user_id = ?",
		legacyMaterial,
		3,
		user.ID,
	); err != nil {
		t.Fatalf("seed legacy smb material: %v", err)
	}

	resolved, err := svc.ResolveSMBPassword(ctx, user.Username)
	if err != nil {
		t.Fatalf("resolve smb password: %v", err)
	}
	if resolved != password {
		t.Fatalf("expected resolved password %q, got %q", password, resolved)
	}

	credential, err := svc.GetSMBCredential(ctx, user.ID)
	if err != nil {
		t.Fatalf("get smb credential: %v", err)
	}
	if credential.MaterialVersion != 4 {
		t.Fatalf("expected migrated material version 4, got %d", credential.MaterialVersion)
	}
	if !strings.HasPrefix(credential.SMBMaterial, "enc:") {
		t.Fatalf("expected encrypted smb material after migration, got %q", credential.SMBMaterial)
	}
	if credential.SMBMaterial == legacyMaterial {
		t.Fatal("expected migrated smb material to differ from legacy payload")
	}
}

func TestResolveSMBPassword_MigratesLegacyJWTFallbackCiphertext(t *testing.T) {
	account.SetSMBMaterialKeyRequired(true)
	t.Cleanup(func() {
		account.SetSMBMaterialKeyRequired(false)
	})
	t.Setenv("COHESION_SMB_MATERIAL_KEY", "current-material-key")
	t.Setenv("COHESION_JWT_SECRET", "legacy-jwt-key")

	svc, db := setupRBACService(t)
	defer db.Close()

	ctx := context.Background()
	password := "jwt-legacy-password"
	user, err := svc.CreateUser(ctx, &account.CreateUserRequest{
		Username: "legacy-jwt-user",
		Password: password,
		Nickname: "Legacy JWT User",
		Role:     account.RoleUser,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	legacyEncrypted, err := encryptForSMBMaterial("legacy-jwt-key", password)
	if err != nil {
		t.Fatalf("encrypt legacy jwt payload: %v", err)
	}
	if _, err := db.ExecContext(
		ctx,
		"UPDATE user_smb_credentials SET smb_material = ?, material_version = ? WHERE user_id = ?",
		legacyEncrypted,
		4,
		user.ID,
	); err != nil {
		t.Fatalf("seed legacy jwt encrypted smb material: %v", err)
	}

	resolved, err := svc.ResolveSMBPassword(ctx, user.Username)
	if err != nil {
		t.Fatalf("resolve smb password: %v", err)
	}
	if resolved != password {
		t.Fatalf("expected resolved password %q, got %q", password, resolved)
	}

	credential, err := svc.GetSMBCredential(ctx, user.ID)
	if err != nil {
		t.Fatalf("get smb credential: %v", err)
	}
	if credential.MaterialVersion != 4 {
		t.Fatalf("expected material version 4, got %d", credential.MaterialVersion)
	}
	if credential.SMBMaterial == legacyEncrypted {
		t.Fatal("expected legacy jwt ciphertext to be re-encrypted with current smb key")
	}
}

func TestResolveSMBPassword_ReturnsRecoverableErrorOnDecodeFailure(t *testing.T) {
	account.SetSMBMaterialKeyRequired(true)
	t.Cleanup(func() {
		account.SetSMBMaterialKeyRequired(false)
	})
	t.Setenv("COHESION_SMB_MATERIAL_KEY", "current-material-key")

	svc, db := setupRBACService(t)
	defer db.Close()

	ctx := context.Background()
	user, err := svc.CreateUser(ctx, &account.CreateUserRequest{
		Username: "decode-fail-user",
		Password: "decode-fail-password",
		Nickname: "Decode Fail User",
		Role:     account.RoleUser,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	if _, err := db.ExecContext(
		ctx,
		"UPDATE user_smb_credentials SET smb_material = ?, material_version = ? WHERE user_id = ?",
		"enc:invalid",
		4,
		user.ID,
	); err != nil {
		t.Fatalf("seed corrupted smb material: %v", err)
	}

	_, err = svc.ResolveSMBPassword(ctx, user.Username)
	if !errors.Is(err, account.ErrSMBCredentialRecoveryRequired) {
		t.Fatalf("expected recoverable smb credential error, got %v", err)
	}
}

func TestPrepareSMBCredential_ReturnsRecoverableErrorWhenKeyMissingUnderStrictPolicy(t *testing.T) {
	account.SetSMBMaterialKeyRequired(false)
	t.Cleanup(func() {
		account.SetSMBMaterialKeyRequired(false)
	})
	t.Setenv("COHESION_SMB_MATERIAL_KEY", "bootstrap-key")

	svc, db := setupRBACService(t)
	defer db.Close()

	ctx := context.Background()
	password := "strict-policy-password"
	if _, err := svc.CreateUser(ctx, &account.CreateUserRequest{
		Username: "strict-user",
		Password: password,
		Nickname: "Strict User",
		Role:     account.RoleUser,
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	t.Setenv("COHESION_SMB_MATERIAL_KEY", "")
	account.SetSMBMaterialKeyRequired(true)

	err := svc.PrepareSMBCredential(ctx, "strict-user", password)
	if !errors.Is(err, account.ErrSMBCredentialRecoveryRequired) {
		t.Fatalf("expected recoverable smb credential error, got %v", err)
	}
}

func encryptForSMBMaterial(secret, password string) (string, error) {
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := aead.Seal(nil, nonce, []byte(password), nil)
	return "enc:" + base64.StdEncoding.EncodeToString(nonce) + ":" + base64.StdEncoding.EncodeToString(ciphertext), nil
}
