package smb

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lteawoo/smb-core"
	"taeu.kr/cohesion/internal/account"
	"taeu.kr/cohesion/internal/space"
)

func TestCoreAuthenticator_Authenticate(t *testing.T) {
	accountSvc, _, db := setupGuardServices(t)
	defer db.Close()

	ctx := context.Background()
	_, err := accountSvc.CreateUser(ctx, &account.CreateUserRequest{
		Username: "auth-user",
		Password: "auth-user-password",
		Nickname: "Auth User",
		Role:     account.RoleUser,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	authn := &coreAuthenticator{accountService: accountSvc}
	principal, err := authn.Authenticate(ctx, "auth-user", "auth-user-password")
	if err != nil {
		t.Fatalf("authenticate success path: %v", err)
	}
	if principal != "auth-user" {
		t.Fatalf("unexpected principal: %q", principal)
	}

	if _, err := authn.Authenticate(ctx, "auth-user", "wrong-password"); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("expected permission error for wrong password, got %v", err)
	}
}

func TestCoreAuthorizer_CanAccessSpace(t *testing.T) {
	accountSvc, spaceSvc, db := setupGuardServices(t)
	defer db.Close()

	ctx := context.Background()
	created, err := spaceSvc.CreateSpace(ctx, &space.CreateSpaceRequest{
		SpaceName: "alpha",
		SpacePath: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("create space: %v", err)
	}

	user, err := accountSvc.CreateUser(ctx, &account.CreateUserRequest{
		Username: "authz-user",
		Password: "authz-user-password",
		Nickname: "Authz User",
		Role:     account.RoleUser,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := accountSvc.ReplaceUserPermissions(ctx, user.ID, []*account.UserSpacePermission{{
		UserID:     user.ID,
		SpaceID:    created.ID,
		Permission: account.PermissionRead,
	}}); err != nil {
		t.Fatalf("replace permissions: %v", err)
	}

	authz := &coreAuthorizer{
		spaceService:   spaceSvc,
		accountService: accountSvc,
	}

	allowed, err := authz.CanAccessSpace(ctx, user.Username, "alpha", smbcore.PermissionRead)
	if err != nil {
		t.Fatalf("read permission check: %v", err)
	}
	if !allowed {
		t.Fatal("expected read permission allow")
	}

	writeAllowed, err := authz.CanAccessSpace(ctx, user.Username, "alpha", smbcore.PermissionWrite)
	if err != nil {
		t.Fatalf("write permission check: %v", err)
	}
	if writeAllowed {
		t.Fatal("expected write permission deny")
	}
}

func TestCoreFileSystem_ListReadAndBoundary(t *testing.T) {
	accountSvc, spaceSvc, db := setupGuardServices(t)
	defer db.Close()

	ctx := context.Background()
	root := t.TempDir()
	docsDir := filepath.Join(root, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	reportPath := filepath.Join(docsDir, "report.txt")
	if err := os.WriteFile(reportPath, []byte("hello-world"), 0644); err != nil {
		t.Fatalf("write report: %v", err)
	}

	created, err := spaceSvc.CreateSpace(ctx, &space.CreateSpaceRequest{
		SpaceName: "alpha",
		SpacePath: root,
	})
	if err != nil {
		t.Fatalf("create space: %v", err)
	}

	user, err := accountSvc.CreateUser(ctx, &account.CreateUserRequest{
		Username: "fs-user",
		Password: "fs-user-password",
		Nickname: "FS User",
		Role:     account.RoleUser,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := accountSvc.ReplaceUserPermissions(ctx, user.ID, []*account.UserSpacePermission{{
		UserID:     user.ID,
		SpaceID:    created.ID,
		Permission: account.PermissionRead,
	}}); err != nil {
		t.Fatalf("replace permissions: %v", err)
	}

	fs := &coreFileSystem{
		spaceService:   spaceSvc,
		accountService: accountSvc,
	}

	statRoot, err := fs.Stat(ctx, user.Username, "/alpha")
	if err != nil {
		t.Fatalf("stat root: %v", err)
	}
	if !statRoot.IsDir {
		t.Fatal("expected space root to be directory")
	}

	entries, err := fs.List(ctx, user.Username, "/alpha")
	if err != nil {
		t.Fatalf("list root: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "docs" || !entries[0].IsDir {
		t.Fatalf("unexpected root entries: %+v", entries)
	}

	data, err := fs.Read(ctx, user.Username, "/alpha/docs/report.txt", 0, 5)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("unexpected read content: %q", string(data))
	}

	if _, err := fs.Read(ctx, user.Username, "/alpha", 0, 1); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("expected deny on directory read, got %v", err)
	}

	if _, err := fs.Stat(ctx, user.Username, "/alpha/../../etc/passwd"); err == nil {
		t.Fatal("expected boundary or scope denial for escaped path")
	}

	if _, err := fs.CreateOrTruncate(ctx, user.Username, "/alpha/docs/new.txt"); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("expected write permission denial for create/truncate, got %v", err)
	}

	if _, err := fs.Write(ctx, user.Username, "/alpha/docs/report.txt", 0, []byte("x")); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("expected write permission denial for write, got %v", err)
	}
}

func TestCoreFileSystem_WriteWithWritePermission(t *testing.T) {
	accountSvc, spaceSvc, db := setupGuardServices(t)
	defer db.Close()

	ctx := context.Background()
	root := t.TempDir()
	docsDir := filepath.Join(root, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}

	created, err := spaceSvc.CreateSpace(ctx, &space.CreateSpaceRequest{
		SpaceName: "alpha",
		SpacePath: root,
	})
	if err != nil {
		t.Fatalf("create space: %v", err)
	}

	user, err := accountSvc.CreateUser(ctx, &account.CreateUserRequest{
		Username: "fs-write-user",
		Password: "fs-write-password",
		Nickname: "FS Write User",
		Role:     account.RoleUser,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := accountSvc.ReplaceUserPermissions(ctx, user.ID, []*account.UserSpacePermission{{
		UserID:     user.ID,
		SpaceID:    created.ID,
		Permission: account.PermissionWrite,
	}}); err != nil {
		t.Fatalf("replace permissions: %v", err)
	}

	fs := &coreFileSystem{
		spaceService:   spaceSvc,
		accountService: accountSvc,
	}

	entry, err := fs.CreateOrTruncate(ctx, user.Username, "/alpha/docs/new.txt")
	if err != nil {
		t.Fatalf("create or truncate: %v", err)
	}
	if entry.IsDir {
		t.Fatal("expected created file entry")
	}

	written, err := fs.Write(ctx, user.Username, "/alpha/docs/new.txt", 0, []byte("hello"))
	if err != nil {
		t.Fatalf("write file: %v", err)
	}
	if written != 5 {
		t.Fatalf("expected 5 bytes written, got %d", written)
	}

	data, err := fs.Read(ctx, user.Username, "/alpha/docs/new.txt", 0, 0)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("unexpected content: %q", string(data))
	}
}
