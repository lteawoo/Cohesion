package account_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"taeu.kr/cohesion/internal/account"
	"taeu.kr/cohesion/internal/account/store"
	"taeu.kr/cohesion/internal/platform/database"
)

func setupRBACService(t *testing.T) (*account.Service, *sql.DB) {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	st := store.NewStore(db)
	return account.NewService(st), db
}

func TestDeleteRole_DeniesWhenRoleAssigned(t *testing.T) {
	svc, db := setupRBACService(t)
	defer db.Close()

	ctx := context.Background()
	role, err := svc.CreateRole(ctx, "editor", "Editor role")
	if err != nil {
		t.Fatalf("create role: %v", err)
	}
	if role.Name != "editor" {
		t.Fatalf("unexpected role name: %s", role.Name)
	}

	if _, err := svc.CreateUser(ctx, &account.CreateUserRequest{
		Username: "editor-user",
		Password: "secret123",
		Nickname: "Editor User",
		Role:     account.Role("editor"),
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	err = svc.DeleteRole(ctx, "editor")
	if err == nil {
		t.Fatal("expected error when deleting assigned role")
	}
	if !strings.Contains(err.Error(), "assigned") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReplaceRolePermissions_DeniesEmptySystemRole(t *testing.T) {
	svc, db := setupRBACService(t)
	defer db.Close()

	err := svc.ReplaceRolePermissions(context.Background(), "admin", []string{})
	if err == nil {
		t.Fatal("expected error for empty system role permissions")
	}
	if !strings.Contains(err.Error(), "System Role") {
		t.Fatalf("unexpected error: %v", err)
	}
}
