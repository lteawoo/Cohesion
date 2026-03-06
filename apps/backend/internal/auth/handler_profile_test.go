package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"taeu.kr/cohesion/internal/account"
	"taeu.kr/cohesion/internal/auth"
)

func TestHandleUpdateMe_UpdatesOwnProfile(t *testing.T) {
	authSvc, accountSvc, db := setupAuthTestService(t)
	defer db.Close()
	_, seededUser := seedAuthUsers(t, accountSvc)

	tokenPair, _, err := authSvc.Login(context.Background(), testUserUsername, testUserPassword)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	handler := auth.NewHandler(authSvc)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	app := authSvc.Middleware(mux)

	payload := map[string]any{
		"nickname":        "Updated Member",
		"currentPassword": testUserPassword,
		"newPassword":     "member-password-updated",
		"role":            "admin",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPatch, "/api/auth/me", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.AccessCookieName, Value: tokenPair.AccessToken})
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d with body %s", rec.Code, rec.Body.String())
	}

	var response struct {
		ID          int64    `json:"id"`
		Username    string   `json:"username"`
		Nickname    string   `json:"nickname"`
		Role        string   `json:"role"`
		Permissions []string `json:"permissions"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.ID != seededUser.ID {
		t.Fatalf("expected user id %d, got %d", seededUser.ID, response.ID)
	}
	if response.Username != testUserUsername {
		t.Fatalf("expected username %q, got %q", testUserUsername, response.Username)
	}
	if response.Nickname != "Updated Member" {
		t.Fatalf("expected updated nickname, got %q", response.Nickname)
	}
	if response.Role != string(account.RoleUser) {
		t.Fatalf("expected role to remain %q, got %q", account.RoleUser, response.Role)
	}
	if len(response.Permissions) == 0 {
		t.Fatal("expected permissions in response")
	}

	reloadedUser, err := accountSvc.GetUserByID(context.Background(), seededUser.ID)
	if err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if reloadedUser.Role != account.RoleUser {
		t.Fatalf("expected stored role %q, got %q", account.RoleUser, reloadedUser.Role)
	}

	oldAuthed, err := accountSvc.Authenticate(context.Background(), testUserUsername, testUserPassword)
	if err != nil {
		t.Fatalf("authenticate old password: %v", err)
	}
	if oldAuthed {
		t.Fatal("expected old password to be rejected")
	}
	newAuthed, err := accountSvc.Authenticate(context.Background(), testUserUsername, "member-password-updated")
	if err != nil {
		t.Fatalf("authenticate new password: %v", err)
	}
	if !newAuthed {
		t.Fatal("expected new password to be accepted")
	}
}

func TestHandleUpdateMe_RejectsInvalidCurrentPassword(t *testing.T) {
	authSvc, accountSvc, db := setupAuthTestService(t)
	defer db.Close()
	seedAuthUsers(t, accountSvc)

	tokenPair, _, err := authSvc.Login(context.Background(), testUserUsername, testUserPassword)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	handler := auth.NewHandler(authSvc)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	app := authSvc.Middleware(mux)

	body := bytes.NewBufferString(`{"currentPassword":"wrong-password","newPassword":"member-password-updated"}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/auth/me", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.AccessCookieName, Value: tokenPair.AccessToken})
	rec := httptest.NewRecorder()

	app.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d with body %s", rec.Code, rec.Body.String())
	}

	var response map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if response["error"] != "current password is incorrect" {
		t.Fatalf("expected current password error, got %#v", response)
	}
}
