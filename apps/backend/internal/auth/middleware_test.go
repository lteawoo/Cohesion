package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"taeu.kr/cohesion/internal/audit"
	"taeu.kr/cohesion/internal/auth"
)

type recordingAuditRecorder struct {
	mu     sync.Mutex
	events []audit.Event
}

func (r *recordingAuditRecorder) RecordBestEffort(event audit.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *recordingAuditRecorder) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.events)
}

func (r *recordingAuditRecorder) Last() (audit.Event, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.events) == 0 {
		return audit.Event{}, false
	}
	return r.events[len(r.events)-1], true
}

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
	recorder := &recordingAuditRecorder{}
	authSvc.SetAuditRecorder(recorder)

	req := httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
	rec := executeMiddlewareRequest(t, authSvc, req, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	if recorder.Count() != 0 {
		t.Fatalf("expected no denied audit events for missing cookie, got %d", recorder.Count())
	}
}

func TestMiddleware_DeniesWhenTokenUserNoLongerExists(t *testing.T) {
	authSvc, accountSvc, db := setupAuthTestService(t)
	defer db.Close()
	_, user := seedAuthUsers(t, accountSvc)

	userToken := issueAccessTokenForTestUser(t, authSvc, testUserUsername)
	if err := accountSvc.DeleteUser(context.Background(), user.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	called := false
	req := httptest.NewRequest(http.MethodGet, "/api/spaces", nil)
	req.AddCookie(&http.Cookie{Name: auth.AccessCookieName, Value: userToken})

	rec := executeMiddlewareRequest(t, authSvc, req, func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	if called {
		t.Fatal("did not expect next handler to be called")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestMiddleware_DeniesWhenRolePermissionIsMissing(t *testing.T) {
	authSvc, accountSvc, db := setupAuthTestService(t)
	defer db.Close()
	_, _ = seedAuthUsers(t, accountSvc)
	recorder := &recordingAuditRecorder{}
	authSvc.SetAuditRecorder(recorder)

	userToken := issueAccessTokenForTestUser(t, authSvc, testUserUsername)
	req := httptest.NewRequest(http.MethodPost, "/api/accounts", nil)
	req.AddCookie(&http.Cookie{Name: auth.AccessCookieName, Value: userToken})

	rec := executeMiddlewareRequest(t, authSvc, req, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	last, ok := recorder.Last()
	if !ok {
		t.Fatal("expected denied audit event to be recorded")
	}
	if last.Action != "account.create" {
		t.Fatalf("expected action account.create, got %s", last.Action)
	}
	if last.Result != audit.ResultDenied {
		t.Fatalf("expected denied result, got %s", last.Result)
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

func TestMiddleware_DeniesSystemBrowseEndpointsWithoutSpaceWrite(t *testing.T) {
	authSvc, accountSvc, db := setupAuthTestService(t)
	defer db.Close()
	_, _ = seedAuthUsers(t, accountSvc)

	userToken := issueAccessTokenForTestUser(t, authSvc, testUserUsername)

	tests := []struct {
		name string
		url  string
	}{
		{
			name: "browse endpoint",
			url:  "/api/browse?path=%2F",
		},
		{
			name: "base directories endpoint",
			url:  "/api/browse/base-directories",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			req.AddCookie(&http.Cookie{Name: auth.AccessCookieName, Value: userToken})

			rec := executeMiddlewareRequest(t, authSvc, req, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})

			if rec.Code != http.StatusForbidden {
				t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
			}
		})
	}
}

func TestMiddleware_DeniesAuditLogsWithoutAccountRead(t *testing.T) {
	authSvc, accountSvc, db := setupAuthTestService(t)
	defer db.Close()
	_, _ = seedAuthUsers(t, accountSvc)
	recorder := &recordingAuditRecorder{}
	authSvc.SetAuditRecorder(recorder)

	userToken := issueAccessTokenForTestUser(t, authSvc, testUserUsername)
	req := httptest.NewRequest(http.MethodGet, "/api/audit/logs?page=1&pageSize=20", nil)
	req.AddCookie(&http.Cookie{Name: auth.AccessCookieName, Value: userToken})

	rec := executeMiddlewareRequest(t, authSvc, req, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	last, ok := recorder.Last()
	if !ok {
		t.Fatal("expected denied audit event to be recorded")
	}
	if last.Action != "audit.logs.read" {
		t.Fatalf("expected action audit.logs.read, got %s", last.Action)
	}
	if last.Result != audit.ResultDenied {
		t.Fatalf("expected denied result, got %s", last.Result)
	}
}

func TestMiddleware_DeniesAuditCleanupWithoutAccountWrite(t *testing.T) {
	authSvc, accountSvc, db := setupAuthTestService(t)
	defer db.Close()
	_, _ = seedAuthUsers(t, accountSvc)
	recorder := &recordingAuditRecorder{}
	authSvc.SetAuditRecorder(recorder)

	userToken := issueAccessTokenForTestUser(t, authSvc, testUserUsername)
	req := httptest.NewRequest(http.MethodPost, "/api/audit/logs/cleanup", nil)
	req.AddCookie(&http.Cookie{Name: auth.AccessCookieName, Value: userToken})

	rec := executeMiddlewareRequest(t, authSvc, req, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	last, ok := recorder.Last()
	if !ok {
		t.Fatal("expected denied audit event to be recorded")
	}
	if last.Action != "audit.logs.cleanup" {
		t.Fatalf("expected action audit.logs.cleanup, got %s", last.Action)
	}
	if last.Result != audit.ResultDenied {
		t.Fatalf("expected denied result, got %s", last.Result)
	}
}

func TestMiddleware_RecordsDeniedForIncludedHighRiskReadEndpoints(t *testing.T) {
	authSvc, accountSvc, db := setupAuthTestService(t)
	defer db.Close()
	_, _ = seedAuthUsers(t, accountSvc)

	userToken := issueAccessTokenForTestUser(t, authSvc, testUserUsername)
	tests := []struct {
		name           string
		path           string
		expectedAction string
	}{
		{name: "accounts list", path: "/api/accounts", expectedAction: "account.list"},
		{name: "accounts list trailing slash", path: "/api/accounts/", expectedAction: "account.list"},
		{name: "roles list", path: "/api/roles", expectedAction: "role.list"},
		{name: "roles list trailing slash", path: "/api/roles/", expectedAction: "role.list"},
		{name: "permissions list", path: "/api/permissions", expectedAction: "permission.list"},
		{name: "config read", path: "/api/config", expectedAction: "config.read"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			recorder := &recordingAuditRecorder{}
			authSvc.SetAuditRecorder(recorder)

			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.AddCookie(&http.Cookie{Name: auth.AccessCookieName, Value: userToken})

			rec := executeMiddlewareRequest(t, authSvc, req, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})

			if rec.Code != http.StatusForbidden {
				t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
			}
			last, ok := recorder.Last()
			if !ok {
				t.Fatal("expected denied audit event to be recorded")
			}
			if last.Action != tc.expectedAction {
				t.Fatalf("expected action %s, got %s", tc.expectedAction, last.Action)
			}
			if last.Result != audit.ResultDenied {
				t.Fatalf("expected denied result, got %s", last.Result)
			}
			if got, _ := last.Metadata["code"].(string); got != "auth.permission_denied" {
				t.Fatalf("expected metadata code auth.permission_denied, got %v", last.Metadata["code"])
			}
		})
	}
}

func TestMiddleware_RecordsDeniedOnInvalidTokenForProtectedEndpoint(t *testing.T) {
	authSvc, _, db := setupAuthTestService(t)
	defer db.Close()
	recorder := &recordingAuditRecorder{}
	authSvc.SetAuditRecorder(recorder)

	req := httptest.NewRequest(http.MethodPost, "/api/accounts", nil)
	req.AddCookie(&http.Cookie{Name: auth.AccessCookieName, Value: "not-a-valid-token"})

	rec := executeMiddlewareRequest(t, authSvc, req, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	last, ok := recorder.Last()
	if !ok {
		t.Fatal("expected denied audit event to be recorded")
	}
	if last.Action != "account.create" {
		t.Fatalf("expected action account.create, got %s", last.Action)
	}
	if last.Result != audit.ResultDenied {
		t.Fatalf("expected denied result, got %s", last.Result)
	}
	if got, _ := last.Metadata["code"].(string); got != "auth.invalid_token" {
		t.Fatalf("expected metadata code auth.invalid_token, got %v", last.Metadata["code"])
	}
}

func TestMiddleware_DoesNotRecordDeniedForExcludedBrowsePath(t *testing.T) {
	authSvc, accountSvc, db := setupAuthTestService(t)
	defer db.Close()
	_, _ = seedAuthUsers(t, accountSvc)
	recorder := &recordingAuditRecorder{}
	authSvc.SetAuditRecorder(recorder)

	userToken := issueAccessTokenForTestUser(t, authSvc, testUserUsername)
	req := httptest.NewRequest(http.MethodGet, "/api/browse/base-directories", nil)
	req.AddCookie(&http.Cookie{Name: auth.AccessCookieName, Value: userToken})

	rec := executeMiddlewareRequest(t, authSvc, req, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if recorder.Count() != 0 {
		t.Fatalf("expected no denied audit event for excluded browse path, got %d", recorder.Count())
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
