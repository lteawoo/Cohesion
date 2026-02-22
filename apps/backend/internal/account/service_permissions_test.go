package account_test

import (
	"context"
	"strings"
	"testing"

	"taeu.kr/cohesion/internal/account"
)

func TestCanAccessSpaceByID_RespectsPermissionHierarchy(t *testing.T) {
	svc, db := setupRBACService(t)
	defer db.Close()

	ctx := context.Background()
	user, err := svc.CreateUser(ctx, &account.CreateUserRequest{
		Username: "space-editor",
		Password: "space-editor-password",
		Nickname: "Space Editor",
		Role:     account.RoleUser,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	result, err := db.ExecContext(ctx, `INSERT INTO space (space_name, space_path) VALUES (?, ?)`, "workspace", "/tmp/workspace")
	if err != nil {
		t.Fatalf("insert space: %v", err)
	}
	spaceID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("read inserted space id: %v", err)
	}

	if err := svc.ReplaceUserPermissions(ctx, user.ID, []*account.UserSpacePermission{
		{UserID: user.ID, SpaceID: spaceID, Permission: account.PermissionWrite},
	}); err != nil {
		t.Fatalf("replace user permissions: %v", err)
	}

	canRead, err := svc.CanAccessSpaceByID(ctx, user.Username, spaceID, account.PermissionRead)
	if err != nil {
		t.Fatalf("check read permission: %v", err)
	}
	if !canRead {
		t.Fatal("expected write permission to allow read access")
	}

	canWrite, err := svc.CanAccessSpaceByID(ctx, user.Username, spaceID, account.PermissionWrite)
	if err != nil {
		t.Fatalf("check write permission: %v", err)
	}
	if !canWrite {
		t.Fatal("expected write permission to allow write access")
	}

	canManage, err := svc.CanAccessSpaceByID(ctx, user.Username, spaceID, account.PermissionManage)
	if err != nil {
		t.Fatalf("check manage permission: %v", err)
	}
	if canManage {
		t.Fatal("expected write permission to deny manage access")
	}
}

func TestReplaceUserPermissions_DetectsUserIDMismatch(t *testing.T) {
	svc, db := setupRBACService(t)
	defer db.Close()

	ctx := context.Background()
	user, err := svc.CreateUser(ctx, &account.CreateUserRequest{
		Username: "permission-check",
		Password: "permission-check-password",
		Nickname: "Permission Check",
		Role:     account.RoleUser,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	err = svc.ReplaceUserPermissions(ctx, user.ID, []*account.UserSpacePermission{
		{
			UserID:     user.ID + 1,
			SpaceID:    1,
			Permission: account.PermissionRead,
		},
	})
	if err == nil {
		t.Fatal("expected userId mismatch error")
	}
	if !strings.Contains(err.Error(), "userId mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}
