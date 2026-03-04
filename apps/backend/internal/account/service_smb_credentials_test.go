package account_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

func TestPrewarmSMBMaterialKey_BootstrapsSMBKeyFileWhenNoCredentialData(t *testing.T) {
	account.SetSMBMaterialKeyRequired(false)
	t.Cleanup(func() {
		account.SetSMBMaterialKeyRequired(false)
	})
	t.Setenv("COHESION_SMB_MATERIAL_KEY", "")

	secretPath := filepath.Join(t.TempDir(), "smb_material_key")
	t.Setenv("COHESION_SMB_MATERIAL_KEY_FILE", secretPath)

	svc, db := setupRBACService(t)
	defer db.Close()

	ctx := context.Background()
	prewarm, err := svc.PrewarmSMBMaterialKey(ctx)
	if err != nil {
		t.Fatalf("prewarm smb key: %v", err)
	}
	if prewarm.Source != "generated" {
		t.Fatalf("expected generated source on first prewarm, got %q", prewarm.Source)
	}

	user, err := svc.CreateUser(ctx, &account.CreateUserRequest{
		Username: "bootstrap-user",
		Password: "bootstrap-password",
		Nickname: "Bootstrap User",
		Role:     account.RoleUser,
	})
	if err != nil {
		t.Fatalf("create user with bootstrap key: %v", err)
	}

	content, err := os.ReadFile(secretPath)
	if err != nil {
		t.Fatalf("read generated smb key file: %v", err)
	}
	if strings.TrimSpace(string(content)) == "" {
		t.Fatalf("expected non-empty smb key file, got %q", string(content))
	}

	credential, err := svc.GetSMBCredential(ctx, user.ID)
	if err != nil {
		t.Fatalf("get smb credential: %v", err)
	}
	if credential.MaterialVersion != 4 {
		t.Fatalf("expected material version 4, got %d", credential.MaterialVersion)
	}
	if !strings.HasPrefix(credential.SMBMaterial, "enc:") {
		t.Fatalf("expected encrypted smb material, got %q", credential.SMBMaterial)
	}
}

func TestPrewarmSMBMaterialKey_ReusesExistingKeyFile(t *testing.T) {
	account.SetSMBMaterialKeyRequired(false)
	t.Cleanup(func() {
		account.SetSMBMaterialKeyRequired(false)
	})
	t.Setenv("COHESION_SMB_MATERIAL_KEY", "")

	secretPath := filepath.Join(t.TempDir(), "smb_material_key")
	t.Setenv("COHESION_SMB_MATERIAL_KEY_FILE", secretPath)

	svc, db := setupRBACService(t)
	defer db.Close()

	ctx := context.Background()
	first, err := svc.PrewarmSMBMaterialKey(ctx)
	if err != nil {
		t.Fatalf("first prewarm smb key: %v", err)
	}
	if first.Source != "generated" {
		t.Fatalf("expected generated source, got %q", first.Source)
	}

	firstContent, err := os.ReadFile(secretPath)
	if err != nil {
		t.Fatalf("read first key file: %v", err)
	}

	second, err := svc.PrewarmSMBMaterialKey(ctx)
	if err != nil {
		t.Fatalf("second prewarm smb key: %v", err)
	}
	if second.Source != "file" {
		t.Fatalf("expected file source on second prewarm, got %q", second.Source)
	}

	secondContent, err := os.ReadFile(secretPath)
	if err != nil {
		t.Fatalf("read second key file: %v", err)
	}
	if strings.TrimSpace(string(firstContent)) != strings.TrimSpace(string(secondContent)) {
		t.Fatal("expected smb key file to be reused without regeneration")
	}
}

func TestPrepareSMBCredential_ReturnsRecoverableErrorWhenKeyMissingWithExistingCredentialData(t *testing.T) {
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
	t.Setenv("COHESION_SMB_MATERIAL_KEY_FILE", filepath.Join(t.TempDir(), "missing_smb_material_key"))

	err := svc.PrepareSMBCredential(ctx, "strict-user", password)
	if !errors.Is(err, account.ErrSMBCredentialRecoveryRequired) {
		t.Fatalf("expected recoverable smb credential error, got %v", err)
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

func TestResolveSMBPassword_ReturnsRecoverableErrorOnLegacyMaterialVersion(t *testing.T) {
	account.SetSMBMaterialKeyRequired(false)
	t.Cleanup(func() {
		account.SetSMBMaterialKeyRequired(false)
	})
	t.Setenv("COHESION_SMB_MATERIAL_KEY", "legacy-unsupported-key")

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

	if _, err := db.ExecContext(
		ctx,
		"UPDATE user_smb_credentials SET smb_material = ?, material_version = ? WHERE user_id = ?",
		"plain:Zm9v",
		3,
		user.ID,
	); err != nil {
		t.Fatalf("seed legacy smb material: %v", err)
	}

	_, err = svc.ResolveSMBPassword(ctx, user.Username)
	if !errors.Is(err, account.ErrSMBCredentialRecoveryRequired) {
		t.Fatalf("expected recoverable smb credential error, got %v", err)
	}
}
