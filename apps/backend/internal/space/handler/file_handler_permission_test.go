package handler

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"taeu.kr/cohesion/internal/account"
	"taeu.kr/cohesion/internal/auth"
)

type fakeSpaceAccessService struct {
	allowed          bool
	err              error
	called           bool
	gotUsername      string
	gotSpaceID       int64
	gotPermissionReq account.Permission
}

func (f *fakeSpaceAccessService) CanAccessSpaceByID(ctx context.Context, username string, spaceID int64, required account.Permission) (bool, error) {
	f.called = true
	f.gotUsername = username
	f.gotSpaceID = spaceID
	f.gotPermissionReq = required
	return f.allowed, f.err
}

func TestEnsureSpacePermission(t *testing.T) {
	newRequest := func(withClaims bool) *http.Request {
		req, err := http.NewRequest(http.MethodPost, "/api/spaces/1/files/move", nil)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		if !withClaims {
			return req
		}
		claims := &auth.Claims{Username: "tester"}
		return req.WithContext(auth.WithClaims(req.Context(), claims))
	}

	t.Run("unauthorized when claims missing", func(t *testing.T) {
		fakeSvc := &fakeSpaceAccessService{}
		h := &Handler{accountService: fakeSvc}

		webErr := h.ensureSpacePermission(newRequest(false), 12, account.PermissionWrite)
		if webErr == nil {
			t.Fatal("expected unauthorized error")
		}
		if webErr.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, webErr.Code)
		}
		if fakeSvc.called {
			t.Fatal("account service must not be called when claims are missing")
		}
	})

	t.Run("forbidden when account service denies access", func(t *testing.T) {
		fakeSvc := &fakeSpaceAccessService{allowed: false}
		h := &Handler{accountService: fakeSvc}

		webErr := h.ensureSpacePermission(newRequest(true), 34, account.PermissionWrite)
		if webErr == nil {
			t.Fatal("expected forbidden error")
		}
		if webErr.Code != http.StatusForbidden {
			t.Fatalf("expected status %d, got %d", http.StatusForbidden, webErr.Code)
		}
		if !fakeSvc.called {
			t.Fatal("expected account service to be called")
		}
		if fakeSvc.gotUsername != "tester" {
			t.Fatalf("expected username tester, got %s", fakeSvc.gotUsername)
		}
		if fakeSvc.gotSpaceID != 34 {
			t.Fatalf("expected spaceID 34, got %d", fakeSvc.gotSpaceID)
		}
		if fakeSvc.gotPermissionReq != account.PermissionWrite {
			t.Fatalf("expected required permission %s, got %s", account.PermissionWrite, fakeSvc.gotPermissionReq)
		}
	})

	t.Run("internal server error when account service fails", func(t *testing.T) {
		fakeSvc := &fakeSpaceAccessService{err: errors.New("db error")}
		h := &Handler{accountService: fakeSvc}

		webErr := h.ensureSpacePermission(newRequest(true), 56, account.PermissionWrite)
		if webErr == nil {
			t.Fatal("expected internal server error")
		}
		if webErr.Code != http.StatusInternalServerError {
			t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, webErr.Code)
		}
	})

	t.Run("allow when account service grants access", func(t *testing.T) {
		fakeSvc := &fakeSpaceAccessService{allowed: true}
		h := &Handler{accountService: fakeSvc}

		webErr := h.ensureSpacePermission(newRequest(true), 78, account.PermissionWrite)
		if webErr != nil {
			t.Fatalf("expected nil error, got %+v", webErr)
		}
	})
}
