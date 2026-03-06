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
	if accessClaims.UserID != seededUser.ID {
		t.Fatalf("expected access token user id %d, got %d", seededUser.ID, accessClaims.UserID)
	}

	refreshClaims, err := authSvc.ParseToken(tokenPair.RefreshToken, "refresh")
	if err != nil {
		t.Fatalf("parse refresh token: %v", err)
	}
	if refreshClaims.Username != seededUser.Username {
		t.Fatalf("expected refresh token username %q, got %q", seededUser.Username, refreshClaims.Username)
	}
	if refreshClaims.UserID != seededUser.ID {
		t.Fatalf("expected refresh token user id %d, got %d", seededUser.ID, refreshClaims.UserID)
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

func TestRefresh_Rejects_WhenUserCreatedAfterTokenIssued(t *testing.T) {
	authSvc, accountSvc, db := setupAuthTestService(t)
	defer db.Close()
	_, seededUser := seedAuthUsers(t, accountSvc)

	tokenPair, _, err := authSvc.Login(context.Background(), testUserUsername, testUserPassword)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	if _, err := db.ExecContext(
		context.Background(),
		"UPDATE users SET created_at = datetime('now', '+1 day') WHERE id = ?",
		seededUser.ID,
	); err != nil {
		t.Fatalf("shift user created_at: %v", err)
	}

	_, _, err = authSvc.Refresh(context.Background(), tokenPair.RefreshToken)
	if !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestUpdateCurrentUser_UpdatesNickname(t *testing.T) {
	authSvc, accountSvc, db := setupAuthTestService(t)
	defer db.Close()
	_, seededUser := seedAuthUsers(t, accountSvc)

	tokenPair, _, err := authSvc.Login(context.Background(), testUserUsername, testUserPassword)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	claims, err := authSvc.ParseToken(tokenPair.AccessToken, "access")
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}

	nickname := "Updated Member"
	updatedUser, err := authSvc.UpdateCurrentUser(context.Background(), claims, &auth.UpdateProfileRequest{
		Nickname: &nickname,
	})
	if err != nil {
		t.Fatalf("update current user: %v", err)
	}
	if updatedUser.Nickname != nickname {
		t.Fatalf("expected nickname %q, got %q", nickname, updatedUser.Nickname)
	}

	reloadedUser, err := accountSvc.GetUserByID(context.Background(), seededUser.ID)
	if err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if reloadedUser.Nickname != nickname {
		t.Fatalf("expected reloaded nickname %q, got %q", nickname, reloadedUser.Nickname)
	}
}

func TestUpdateCurrentUser_RequiresValidCurrentPassword(t *testing.T) {
	authSvc, accountSvc, db := setupAuthTestService(t)
	defer db.Close()
	seedAuthUsers(t, accountSvc)

	tokenPair, _, err := authSvc.Login(context.Background(), testUserUsername, testUserPassword)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	claims, err := authSvc.ParseToken(tokenPair.AccessToken, "access")
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}

	newPassword := "member-password-updated"
	_, err = authSvc.UpdateCurrentUser(context.Background(), claims, &auth.UpdateProfileRequest{
		CurrentPassword: nil,
		NewPassword:     &newPassword,
	})
	if err == nil || err.Error() != "current password is required" {
		t.Fatalf("expected current password required error, got %v", err)
	}

	currentPassword := "wrong-password"
	_, err = authSvc.UpdateCurrentUser(context.Background(), claims, &auth.UpdateProfileRequest{
		CurrentPassword: &currentPassword,
		NewPassword:     &newPassword,
	})
	if err == nil || err.Error() != "current password is incorrect" {
		t.Fatalf("expected current password incorrect error, got %v", err)
	}
}

func TestUpdateCurrentUser_UpdatesPasswordWithEightCharacterPolicy(t *testing.T) {
	authSvc, accountSvc, db := setupAuthTestService(t)
	defer db.Close()
	seedAuthUsers(t, accountSvc)

	tokenPair, _, err := authSvc.Login(context.Background(), testUserUsername, testUserPassword)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	claims, err := authSvc.ParseToken(tokenPair.AccessToken, "access")
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}

	currentPassword := testUserPassword
	shortPassword := "short7"
	_, err = authSvc.UpdateCurrentUser(context.Background(), claims, &auth.UpdateProfileRequest{
		CurrentPassword: &currentPassword,
		NewPassword:     &shortPassword,
	})
	if err == nil || err.Error() != "password must be at least 8 characters" {
		t.Fatalf("expected min length error, got %v", err)
	}

	newPassword := "member-password-updated"
	_, err = authSvc.UpdateCurrentUser(context.Background(), claims, &auth.UpdateProfileRequest{
		CurrentPassword: &currentPassword,
		NewPassword:     &newPassword,
	})
	if err != nil {
		t.Fatalf("update current user password: %v", err)
	}

	oldAuthed, err := accountSvc.Authenticate(context.Background(), testUserUsername, testUserPassword)
	if err != nil {
		t.Fatalf("authenticate old password: %v", err)
	}
	if oldAuthed {
		t.Fatal("expected old password to be rejected")
	}

	newAuthed, err := accountSvc.Authenticate(context.Background(), testUserUsername, newPassword)
	if err != nil {
		t.Fatalf("authenticate new password: %v", err)
	}
	if !newAuthed {
		t.Fatal("expected new password to be accepted")
	}
}
