package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"taeu.kr/cohesion/internal/account"
	accountStore "taeu.kr/cohesion/internal/account/store"
	"taeu.kr/cohesion/internal/auth"
	"taeu.kr/cohesion/internal/config"
	"taeu.kr/cohesion/internal/platform/database"
	"taeu.kr/cohesion/internal/space"
	spaceStore "taeu.kr/cohesion/internal/space/store"
)

type recoveryTestTerminal struct {
	bytes.Buffer
	username string
	password func(prompt string) (string, error)
}

func (t *recoveryTestTerminal) ReadLine() (string, error) {
	return t.username, nil
}

func (t *recoveryTestTerminal) ReadPassword(prompt string) (string, error) {
	return t.password(prompt)
}

func configureRecoveryTest(t *testing.T, databaseURL string) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv(config.ProductionStateRootEnv, root)
	configDir := filepath.Join(root, "config")
	if err := os.Mkdir(configDir, 0700); err != nil {
		t.Fatal(err)
	}
	content := "database:\n  url: " + databaseURL + "\n"
	if err := os.WriteFile(filepath.Join(configDir, "config.prod.yaml"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestResetAdministratorPassword(t *testing.T) {
	const (
		adminUsername = "other-admin"
		oldPassword   = "initial-password"
		newPassword   = "  replacement password  "
		userPassword  = "member-password"
	)
	root := configureRecoveryTest(t, "../custom/instance database.db")
	dataDir := filepath.Join(root, "custom")
	if err := os.Mkdir(dataDir, 0700); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dataDir, "instance database.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}

	store := accountStore.NewStore(db)
	accounts := account.NewService(store)
	admin, err := accounts.CreateUser(ctx, &account.CreateUserRequest{
		Username: adminUsername, Password: oldPassword, Nickname: "Existing operator", Role: account.RoleAdmin,
	})
	if err != nil {
		t.Fatal(err)
	}
	member, err := accounts.CreateUser(ctx, &account.CreateUserRequest{
		Username: "member", Password: userPassword, Nickname: "Existing member", Role: account.RoleUser,
	})
	if err != nil {
		t.Fatal(err)
	}

	folder := t.TempDir()
	file := filepath.Join(folder, "existing.txt")
	if err := os.WriteFile(file, []byte("existing contents"), 0600); err != nil {
		t.Fatal(err)
	}
	spaces := spaceStore.NewStore(db)
	existingSpace, err := spaces.Create(ctx, &space.CreateSpaceRequest{SpaceName: "Existing files", SpacePath: folder})
	if err != nil {
		t.Fatal(err)
	}
	if err := accounts.ReplaceUserPermissions(ctx, member.ID, []*account.UserSpacePermission{
		{UserID: member.ID, SpaceID: existingSpace.ID, Permission: account.PermissionRead},
	}); err != nil {
		t.Fatal(err)
	}

	currentAdmin, err := store.GetUserByID(ctx, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, username, newPassword, confirmation, wantError string
		cancel                                               bool
	}{
		{name: "cancelled", username: adminUsername, newPassword: newPassword, cancel: true, wantError: "cancelled"},
		{name: "mismatch", username: adminUsername, newPassword: newPassword, confirmation: "different password", wantError: "passwords do not match"},
		{name: "invalid password", username: adminUsername, newPassword: " short ", confirmation: " short ", wantError: "at least 8 characters"},
		{name: "not an administrator", username: member.Username, wantError: "not an administrator"},
		{name: "missing administrator", username: "missing-admin", wantError: "not found"},
	} {
		t.Run(test.name, func(t *testing.T) {
			terminal := &recoveryTestTerminal{username: test.username}
			terminal.password = func(prompt string) (string, error) {
				if test.username != adminUsername {
					t.Fatal("password prompt appeared before confirming an administrator")
				}
				switch prompt {
				case "New password: ":
					return test.newPassword, nil
				case "Confirm new password: ":
					if test.cancel {
						return "", io.EOF
					}
					return test.confirmation, nil
				default:
					t.Fatalf("unexpected password prompt %q", prompt)
					return "", nil
				}
			}
			err := resetAdministratorPassword("production", terminal)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("expected %q, got %v", test.wantError, err)
			}
			unchangedAdmin, err := store.GetUserByID(ctx, admin.ID)
			if err != nil || !reflect.DeepEqual(currentAdmin, unchangedAdmin) {
				t.Fatalf("failed reset changed the administrator row: %v", err)
			}
		})
	}

	t.Run("concurrent password change", func(t *testing.T) {
		concurrentPassword := "another confirmed password"
		terminal := &recoveryTestTerminal{username: adminUsername}
		terminal.password = func(prompt string) (string, error) {
			if prompt == "Confirm new password: " {
				_, err := accounts.UpdateUser(ctx, admin.ID, &account.UpdateUserRequest{Password: &concurrentPassword})
				if err != nil {
					t.Fatal(err)
				}
			}
			return newPassword, nil
		}
		err := resetAdministratorPassword("production", terminal)
		if err == nil || !strings.Contains(err.Error(), "changed during recovery") {
			t.Fatalf("expected conflict, got %v", err)
		}
		if ok, err := accounts.Authenticate(ctx, adminUsername, concurrentPassword); err != nil || !ok {
			t.Fatalf("concurrent password was not kept: %v", err)
		}
		restored := oldPassword
		if _, err := accounts.UpdateUser(ctx, admin.ID, &account.UpdateUserRequest{Password: &restored}); err != nil {
			t.Fatal(err)
		}
	})

	terminal := &recoveryTestTerminal{}
	var prompts []string
	terminal.password = func(prompt string) (string, error) {
		prompts = append(prompts, prompt)
		if !strings.Contains(terminal.String(), adminUsername) {
			t.Fatal("password requested before displaying the administrator")
		}
		return newPassword, nil
	}
	if err := resetAdministratorPassword("production", terminal); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(prompts, []string{"New password: ", "Confirm new password: "}) {
		t.Fatalf("unexpected password prompts: %v", prompts)
	}
	if strings.Contains(terminal.String(), newPassword) || !strings.Contains(terminal.String(), "Password reset") {
		t.Fatal("unexpected password reset output")
	}

	signIn := auth.NewService(accounts, auth.Config{Secret: strings.Repeat("x", 32)})
	if _, _, err := signIn.Login(ctx, adminUsername, oldPassword); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("expected old password to fail, got %v", err)
	}
	_, signedIn, err := signIn.Login(ctx, adminUsername, newPassword)
	if err != nil || signedIn.ID != admin.ID || signedIn.Nickname != admin.Nickname || signedIn.Role != admin.Role {
		t.Fatalf("expected original administrator to sign in with the new password: %v", err)
	}
	if _, _, err := signIn.Login(ctx, member.Username, userPassword); err != nil {
		t.Fatalf("existing member credentials changed: %v", err)
	}
	if currentSpace, err := spaces.GetByID(ctx, existingSpace.ID); err != nil || currentSpace.SpacePath != folder || currentSpace.SpaceName != existingSpace.SpaceName {
		t.Fatalf("existing space changed: %v", err)
	}
	if allowed, err := accounts.CanAccessSpaceByID(ctx, member.Username, existingSpace.ID, account.PermissionRead); err != nil || !allowed {
		t.Fatalf("existing member lost space access: %v", err)
	}
	if content, err := os.ReadFile(file); err != nil || string(content) != "existing contents" {
		t.Fatalf("existing file changed: %v", err)
	}

	secondAdmin, err := accounts.CreateUser(ctx, &account.CreateUserRequest{
		Username: "second-admin", Password: oldPassword, Nickname: "Second operator", Role: account.RoleAdmin,
	})
	if err != nil {
		t.Fatal(err)
	}
	terminal = &recoveryTestTerminal{}
	err = resetAdministratorPassword("production", terminal)
	if err == nil || !strings.Contains(err.Error(), "enter the username") {
		t.Fatalf("expected recovery to require an explicit selection, got %v", err)
	}
	for _, username := range []string{admin.Username, secondAdmin.Username} {
		if !strings.Contains(terminal.String(), username) {
			t.Fatalf("missing administrator %q from the list", username)
		}
	}

	terminal = &recoveryTestTerminal{
		username: secondAdmin.Username,
		password: func(string) (string, error) { return newPassword, nil },
	}
	if err := resetAdministratorPassword("production", terminal); err != nil {
		t.Fatal(err)
	}
	if _, _, err := signIn.Login(ctx, secondAdmin.Username, newPassword); err != nil {
		t.Fatalf("selected administrator could not sign in: %v", err)
	}
	if _, _, err := signIn.Login(ctx, admin.Username, newPassword); err != nil {
		t.Fatalf("unselected administrator credentials changed: %v", err)
	}
}

func TestResetAdministratorPasswordRequiresExistingInstallation(t *testing.T) {
	t.Run("missing configuration", func(t *testing.T) {
		stateRoot := filepath.Join(t.TempDir(), "absent")
		t.Setenv(config.ProductionStateRootEnv, stateRoot)
		err := resetAdministratorPassword("production", &recoveryTestTerminal{})
		if err == nil || !strings.Contains(err.Error(), "existing Cohesion installation") {
			t.Fatalf("expected missing configuration error, got %v", err)
		}
		if _, err := os.Stat(stateRoot); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("unexpected state directory: %v", err)
		}
	})

	t.Run("missing database", func(t *testing.T) {
		root := configureRecoveryTest(t, "../data/cohesion.db")
		err := resetAdministratorPassword("production", &recoveryTestTerminal{})
		if err == nil || !strings.Contains(err.Error(), "existing Cohesion database") {
			t.Fatalf("expected missing database error, got %v", err)
		}
		if _, err := os.Stat(filepath.Join(root, "data")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("unexpected data directory: %v", err)
		}
	})
}

func TestAdminCommandRequiresInteractiveInput(t *testing.T) {
	input, output, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	defer output.Close()

	err = runAdminCommand([]string{"reset-password"}, input, output)
	if err == nil || !strings.Contains(err.Error(), "interactive terminal") {
		t.Fatalf("expected interactive terminal error, got %v", err)
	}
	err = runAdminCommand([]string{"reset-password", "password-in-argument"}, input, output)
	if err == nil || err.Error() != adminCommandUsage {
		t.Fatalf("expected usage without supplied arguments, got %v", err)
	}
}
