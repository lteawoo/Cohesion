package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"taeu.kr/cohesion/internal/auth"
)

func issueAccessTokenForTestUser(t *testing.T, authSvc *auth.Service, username string) string {
	t.Helper()

	tokenPair, _, err := authSvc.Login(context.Background(), username, map[string]string{
		testAdminUsername: testAdminPassword,
		testUserUsername:  testUserPassword,
	}[username])
	if err != nil {
		t.Fatalf("login failed for %s: %v", username, err)
	}
	if tokenPair == nil || tokenPair.AccessToken == "" {
		t.Fatalf("expected access token for %s", username)
	}
	return tokenPair.AccessToken
}

func executeMiddlewareRequest(
	t *testing.T,
	authSvc *auth.Service,
	req *http.Request,
	next http.HandlerFunc,
) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	authSvc.Middleware(next).ServeHTTP(rec, req)
	return rec
}

func TestMiddleware_AllowsPublicAPIWithoutToken(t *testing.T) {
	authSvc, _, db := setupAuthTestService(t)
	defer db.Close()

	called := false
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := executeMiddlewareRequest(t, authSvc, req, func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	if !called {
		t.Fatal("expected next handler to be called for public path")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
}

func TestMiddleware_DeniesWithoutAccessCookie(t *testing.T) {
	authSvc, _, db := setupAuthTestService(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
	rec := executeMiddlewareRequest(t, authSvc, req, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestMiddleware_DeniesWhenRolePermissionIsMissing(t *testing.T) {
	authSvc, accountSvc, db := setupAuthTestService(t)
	defer db.Close()
	_, _ = seedAuthUsers(t, accountSvc)

	userToken := issueAccessTokenForTestUser(t, authSvc, testUserUsername)
	req := httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
	req.AddCookie(&http.Cookie{Name: auth.AccessCookieName, Value: userToken})

	rec := executeMiddlewareRequest(t, authSvc, req, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestMiddleware_DeniesWhenSpacePermissionIsMissing(t *testing.T) {
	authSvc, accountSvc, db := setupAuthTestService(t)
	defer db.Close()
	_, _ = seedAuthUsers(t, accountSvc)

	userToken := issueAccessTokenForTestUser(t, authSvc, testUserUsername)
	req := httptest.NewRequest(http.MethodPost, "/api/spaces/1/files/move", nil)
	req.AddCookie(&http.Cookie{Name: auth.AccessCookieName, Value: userToken})

	rec := executeMiddlewareRequest(t, authSvc, req, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestMiddleware_AllowsAndInjectsClaims_WhenAuthorized(t *testing.T) {
	authSvc, accountSvc, db := setupAuthTestService(t)
	defer db.Close()
	_, _ = seedAuthUsers(t, accountSvc)

	adminToken := issueAccessTokenForTestUser(t, authSvc, testAdminUsername)
	req := httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
	req.AddCookie(&http.Cookie{Name: auth.AccessCookieName, Value: adminToken})

	called := false
	rec := executeMiddlewareRequest(t, authSvc, req, func(w http.ResponseWriter, r *http.Request) {
		called = true
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok {
			t.Fatal("expected claims in request context")
		}
		if claims.Username != testAdminUsername {
			t.Fatalf("expected username %q, got %q", testAdminUsername, claims.Username)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	if !called {
		t.Fatal("expected next handler to be called")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
}
