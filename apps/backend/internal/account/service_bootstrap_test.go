package account_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"taeu.kr/cohesion/internal/account"
	"taeu.kr/cohesion/internal/account/store"
	"taeu.kr/cohesion/internal/platform/database"
)

func setupBootstrapService(t *testing.T) (*account.Service, *sql.DB) {
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

func TestEnsureDefaultAdmin_DoesNotCreateWeakDefaults(t *testing.T) {
	t.Setenv("COHESION_ADMIN_USER", "")
	t.Setenv("COHESION_ADMIN_PASSWORD", "")
	t.Setenv("COHESION_ADMIN_NICKNAME", "")

	svc, db := setupBootstrapService(t)
	defer db.Close()

	if err := svc.EnsureDefaultAdmin(context.Background()); err != nil {
		t.Fatalf("ensure default admin: %v", err)
	}

	needsBootstrap, err := svc.NeedsBootstrap(context.Background())
	if err != nil {
		t.Fatalf("needs bootstrap: %v", err)
	}
	if !needsBootstrap {
		t.Fatal("expected bootstrap to remain required without explicit admin credentials")
	}
}

func TestEnsureDefaultAdmin_CreatesAdminWhenEnvProvided(t *testing.T) {
	t.Setenv("COHESION_ADMIN_USER", "admin-env")
	t.Setenv("COHESION_ADMIN_PASSWORD", "env-secret-123")
	t.Setenv("COHESION_ADMIN_NICKNAME", "Env Admin")

	svc, db := setupBootstrapService(t)
	defer db.Close()

	if err := svc.EnsureDefaultAdmin(context.Background()); err != nil {
		t.Fatalf("ensure default admin: %v", err)
	}

	needsBootstrap, err := svc.NeedsBootstrap(context.Background())
	if err != nil {
		t.Fatalf("needs bootstrap: %v", err)
	}
	if needsBootstrap {
		t.Fatal("expected bootstrap to complete when env credentials are provided")
	}
}

func TestBootstrapInitialAdmin_OnlyAllowsFirstRun(t *testing.T) {
	svc, db := setupBootstrapService(t)
	defer db.Close()

	_, err := svc.BootstrapInitialAdmin(context.Background(), &account.CreateUserRequest{
		Username: "first-admin",
		Password: "first-admin-password",
		Nickname: "First Admin",
	})
	if err != nil {
		t.Fatalf("bootstrap initial admin: %v", err)
	}

	_, err = svc.BootstrapInitialAdmin(context.Background(), &account.CreateUserRequest{
		Username: "second-admin",
		Password: "second-admin-password",
		Nickname: "Second Admin",
	})
	if !errors.Is(err, account.ErrInitialSetupCompleted) {
		t.Fatalf("expected ErrInitialSetupCompleted, got %v", err)
	}
}
