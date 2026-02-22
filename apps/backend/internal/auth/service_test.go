package auth_test

import (
	"context"
	"errors"
	"testing"

	"taeu.kr/cohesion/internal/auth"
)

func TestLogin_ReturnsSetupRequired_WhenNoAdminExists(t *testing.T) {
	authSvc, _, db := setupAuthTestService(t)
	defer db.Close()

	_, _, err := authSvc.Login(context.Background(), testAdminUsername, testAdminPassword)
	if !errors.Is(err, auth.ErrSetupRequired) {
		t.Fatalf("expected ErrSetupRequired, got %v", err)
	}
}

func TestLogin_IssuesTokenPair_OnValidCredentials(t *testing.T) {
	authSvc, accountSvc, db := setupAuthTestService(t)
	defer db.Close()
	_, seededUser := seedAuthUsers(t, accountSvc)

	tokenPair, user, err := authSvc.Login(context.Background(), testUserUsername, testUserPassword)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if tokenPair == nil || tokenPair.AccessToken == "" || tokenPair.RefreshToken == "" {
		t.Fatalf("expected non-empty token pair, got %#v", tokenPair)
	}
	if user == nil || user.Username != seededUser.Username {
		t.Fatalf("expected user %q, got %#v", seededUser.Username, user)
	}

	accessClaims, err := authSvc.ParseToken(tokenPair.AccessToken, "access")
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}
	if accessClaims.Username != seededUser.Username {
		t.Fatalf("expected access token username %q, got %q", seededUser.Username, accessClaims.Username)
	}

	refreshClaims, err := authSvc.ParseToken(tokenPair.RefreshToken, "refresh")
	if err != nil {
		t.Fatalf("parse refresh token: %v", err)
	}
	if refreshClaims.Username != seededUser.Username {
		t.Fatalf("expected refresh token username %q, got %q", seededUser.Username, refreshClaims.Username)
	}
}

func TestLogin_ReturnsInvalidCredentials_OnWrongPassword(t *testing.T) {
	authSvc, accountSvc, db := setupAuthTestService(t)
	defer db.Close()
	seedAuthUsers(t, accountSvc)

	_, _, err := authSvc.Login(context.Background(), testUserUsername, "wrong-password")
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestRefresh_RejectsAccessToken(t *testing.T) {
	authSvc, accountSvc, db := setupAuthTestService(t)
	defer db.Close()
	seedAuthUsers(t, accountSvc)

	tokenPair, _, err := authSvc.Login(context.Background(), testUserUsername, testUserPassword)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	_, _, err = authSvc.Refresh(context.Background(), tokenPair.AccessToken)
	if !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestRefresh_IssuesNewPair_WithRefreshToken(t *testing.T) {
	authSvc, accountSvc, db := setupAuthTestService(t)
	defer db.Close()
	seedAuthUsers(t, accountSvc)

	tokenPair, loggedInUser, err := authSvc.Login(context.Background(), testUserUsername, testUserPassword)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	newPair, refreshedUser, err := authSvc.Refresh(context.Background(), tokenPair.RefreshToken)
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if newPair == nil || newPair.AccessToken == "" || newPair.RefreshToken == "" {
		t.Fatalf("expected non-empty token pair, got %#v", newPair)
	}
	if refreshedUser == nil || refreshedUser.ID != loggedInUser.ID {
		t.Fatalf("expected refreshed user id %d, got %#v", loggedInUser.ID, refreshedUser)
	}
}
