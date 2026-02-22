package auth_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"taeu.kr/cohesion/internal/account"
	accountstore "taeu.kr/cohesion/internal/account/store"
	"taeu.kr/cohesion/internal/auth"
	"taeu.kr/cohesion/internal/platform/database"
)

const (
	testAdminUsername = "admin-test"
	testAdminPassword = "admin-test-password"
	testUserUsername  = "member-test"
	testUserPassword  = "member-test-password"
)

func setupAuthTestService(t *testing.T) (*auth.Service, *account.Service, *sql.DB) {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	accountSvc := account.NewService(accountstore.NewStore(db))
	authSvc := auth.NewService(accountSvc, auth.Config{
		Secret:         "test-secret-key",
		Issuer:         "cohesion-test",
		AccessTokenTTL: 15 * time.Minute,
		RefreshTTL:     24 * time.Hour,
	})

	return authSvc, accountSvc, db
}

func seedAuthUsers(t *testing.T, accountSvc *account.Service) (*account.User, *account.User) {
	t.Helper()

	ctx := context.Background()

	admin, err := accountSvc.BootstrapInitialAdmin(ctx, &account.CreateUserRequest{
		Username: testAdminUsername,
		Password: testAdminPassword,
		Nickname: "Admin Tester",
	})
	if err != nil {
		t.Fatalf("bootstrap admin: %v", err)
	}

	user, err := accountSvc.CreateUser(ctx, &account.CreateUserRequest{
		Username: testUserUsername,
		Password: testUserPassword,
		Nickname: "Member Tester",
		Role:     account.RoleUser,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	return admin, user
}
