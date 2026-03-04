package smb

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"taeu.kr/cohesion/internal/account"
	accountstore "taeu.kr/cohesion/internal/account/store"
	"taeu.kr/cohesion/internal/platform/database"
	"taeu.kr/cohesion/internal/space"
	spacestore "taeu.kr/cohesion/internal/space/store"
)

func TestGuard_ResolvePath_AndPermission(t *testing.T) {
	accountSvc, spaceSvc, db := setupGuardServices(t)
	defer db.Close()

	ctx := context.Background()
	rootA := t.TempDir()
	createdA, err := spaceSvc.CreateSpace(ctx, &space.CreateSpaceRequest{SpaceName: "alpha", SpacePath: rootA})
	if err != nil {
		t.Fatalf("create space alpha: %v", err)
	}

	user, err := accountSvc.CreateUser(ctx, &account.CreateUserRequest{
		Username: "guard-user",
		Password: "guard-user-password",
		Nickname: "Guard User",
		Role:     account.RoleUser,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := accountSvc.ReplaceUserPermissions(ctx, user.ID, []*account.UserSpacePermission{{
		UserID:     user.ID,
		SpaceID:    createdA.ID,
		Permission: account.PermissionRead,
	}}); err != nil {
		t.Fatalf("replace permissions: %v", err)
	}

	guard := NewGuard(spaceSvc, accountSvc, user.Username)

	_, absPath, relPath, err := guard.ResolvePath("/alpha/docs/report.txt", account.PermissionRead)
	if err != nil {
		t.Fatalf("resolve read path: %v", err)
	}
	if relPath != "docs/report.txt" {
		t.Fatalf("expected relPath docs/report.txt, got %q", relPath)
	}
	if !IsPathWithinSpace(absPath, rootA) {
		t.Fatalf("expected absolute path within space root, got %q", absPath)
	}

	_, _, _, err = guard.ResolvePath("/alpha/docs/report.txt", account.PermissionWrite)
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("expected write denial, got %v", err)
	}
}

func TestGuard_DeniesPathEscape_AndCrossSpaceRename(t *testing.T) {
	accountSvc, spaceSvc, db := setupGuardServices(t)
	defer db.Close()

	ctx := context.Background()
	rootA := t.TempDir()
	rootB := t.TempDir()
	spaceA, err := spaceSvc.CreateSpace(ctx, &space.CreateSpaceRequest{SpaceName: "alpha", SpacePath: rootA})
	if err != nil {
		t.Fatalf("create space alpha: %v", err)
	}
	spaceB, err := spaceSvc.CreateSpace(ctx, &space.CreateSpaceRequest{SpaceName: "beta", SpacePath: rootB})
	if err != nil {
		t.Fatalf("create space beta: %v", err)
	}

	user, err := accountSvc.CreateUser(ctx, &account.CreateUserRequest{
		Username: "rename-user",
		Password: "rename-user-password",
		Nickname: "Rename User",
		Role:     account.RoleUser,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := accountSvc.ReplaceUserPermissions(ctx, user.ID, []*account.UserSpacePermission{
		{UserID: user.ID, SpaceID: spaceA.ID, Permission: account.PermissionWrite},
		{UserID: user.ID, SpaceID: spaceB.ID, Permission: account.PermissionWrite},
	}); err != nil {
		t.Fatalf("replace permissions: %v", err)
	}

	guard := NewGuard(spaceSvc, accountSvc, user.Username)

	_, escaped, _, err := guard.ResolvePath("/alpha/../alpha/../../etc/passwd", account.PermissionRead)
	if err == nil {
		t.Fatalf("expected path resolution denial, got err=%v path=%q", err, escaped)
	}

	if err := guard.ValidateRename("/alpha/docs/a.txt", "/beta/docs/a.txt"); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("expected cross-space rename denial, got %v", err)
	}
}

func setupGuardServices(t *testing.T) (*account.Service, *space.Service, *sql.DB) {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	accountSvc := account.NewService(accountstore.NewStore(db))
	spaceSvc := space.NewService(spacestore.NewStore(db))
	return accountSvc, spaceSvc, db
}

func TestSplitVirtualPath_AndNormalize(t *testing.T) {
	clean := NormalizeVirtualPath(`\\alpha\\docs\\report.txt`)
	if clean != "/alpha/docs/report.txt" {
		t.Fatalf("unexpected normalized path: %q", clean)
	}

	spaceName, relPath, err := SplitVirtualPath(clean)
	if err != nil {
		t.Fatalf("split path: %v", err)
	}
	if spaceName != "alpha" || relPath != filepath.ToSlash("docs/report.txt") {
		t.Fatalf("unexpected split result: %q %q", spaceName, relPath)
	}
}
