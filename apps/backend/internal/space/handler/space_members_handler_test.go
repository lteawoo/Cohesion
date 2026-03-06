package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"taeu.kr/cohesion/internal/account"
	accountstore "taeu.kr/cohesion/internal/account/store"
	"taeu.kr/cohesion/internal/audit"
	"taeu.kr/cohesion/internal/auth"
	"taeu.kr/cohesion/internal/platform/database"
	"taeu.kr/cohesion/internal/space"
	spacestore "taeu.kr/cohesion/internal/space/store"
)

func setupSpaceMembersHandler(t *testing.T) (*Handler, *account.Service, *sql.DB, *recordingAuditSink) {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	accountSvc := account.NewService(accountstore.NewStore(db))
	spaceSvc := space.NewService(spacestore.NewStore(db))
	handler := NewHandler(spaceSvc, nil, accountSvc)
	recorder := &recordingAuditSink{}
	handler.SetAuditRecorder(recorder)

	return handler, accountSvc, db, recorder
}

func insertTestSpace(t *testing.T, db *sql.DB, name string) int64 {
	t.Helper()

	result, err := db.ExecContext(context.Background(), `INSERT INTO space (space_name, space_path) VALUES (?, ?)`, name, fmt.Sprintf("/tmp/%s", name))
	if err != nil {
		t.Fatalf("insert space: %v", err)
	}
	spaceID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}
	return spaceID
}

func TestHandleSpaceMembers_GetListsAssignedMembers(t *testing.T) {
	handler, accountSvc, db, _ := setupSpaceMembersHandler(t)
	defer db.Close()

	ctx := context.Background()
	user, err := accountSvc.CreateUser(ctx, &account.CreateUserRequest{
		Username: "space-member",
		Password: "space-member-password",
		Nickname: "Space Member",
		Role:     account.RoleUser,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	spaceID := insertTestSpace(t, db, "alpha")

	if err := accountSvc.ReplaceSpaceMembers(ctx, spaceID, []*account.UserSpacePermission{
		{UserID: user.ID, SpaceID: spaceID, Permission: account.PermissionRead},
	}); err != nil {
		t.Fatalf("replace space members: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/spaces/%d/members", spaceID), nil)
	rec := httptest.NewRecorder()

	if webErr := handler.handleSpaceByID(rec, req); webErr != nil {
		t.Fatalf("unexpected web error: %+v", webErr)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var members []account.SpaceMember
	if err := json.NewDecoder(rec.Body).Decode(&members); err != nil {
		t.Fatalf("decode members response: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}
	if members[0].Username != user.Username {
		t.Fatalf("expected username %q, got %q", user.Username, members[0].Username)
	}
	if members[0].Permission != account.PermissionRead {
		t.Fatalf("expected permission %q, got %q", account.PermissionRead, members[0].Permission)
	}
}

func TestHandleSpaceMembers_PutReplacesAssignmentsAndRecordsAudit(t *testing.T) {
	handler, accountSvc, db, recorder := setupSpaceMembersHandler(t)
	defer db.Close()

	ctx := context.Background()
	user, err := accountSvc.CreateUser(ctx, &account.CreateUserRequest{
		Username: "space-editor",
		Password: "space-editor-password",
		Nickname: "Space Editor",
		Role:     account.RoleUser,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	spaceID := insertTestSpace(t, db, "beta")

	body := bytes.NewBufferString(fmt.Sprintf(`{"members":[{"userId":%d,"permission":"write"}]}`, user.ID))
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/spaces/%d/members", spaceID), body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(auth.WithClaims(req.Context(), &auth.Claims{Username: "admin"}))
	rec := httptest.NewRecorder()

	if webErr := handler.handleSpaceByID(rec, req); webErr != nil {
		t.Fatalf("unexpected web error: %+v", webErr)
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}

	members, err := accountSvc.ListSpaceMembers(ctx, spaceID)
	if err != nil {
		t.Fatalf("list space members: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}
	if members[0].Permission != account.PermissionWrite {
		t.Fatalf("expected permission %q, got %q", account.PermissionWrite, members[0].Permission)
	}

	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(recorder.events))
	}
	if recorder.events[0].Action != "space.members.replace" {
		t.Fatalf("expected action %q, got %q", "space.members.replace", recorder.events[0].Action)
	}
	if recorder.events[0].Result != audit.ResultSuccess {
		t.Fatalf("expected result %q, got %q", audit.ResultSuccess, recorder.events[0].Result)
	}
}

func TestHandleSpaceMembers_PutRejectsInvalidMembers(t *testing.T) {
	handler, _, db, recorder := setupSpaceMembersHandler(t)
	defer db.Close()

	spaceID := insertTestSpace(t, db, "gamma")
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/spaces/%d/members", spaceID), bytes.NewBufferString(`{"members":[{"userId":0,"permission":"read"}]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	webErr := handler.handleSpaceByID(rec, req)
	if webErr == nil {
		t.Fatal("expected web error")
	}
	if webErr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, webErr.Code)
	}
	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(recorder.events))
	}
	if recorder.events[0].Result != audit.ResultFailure {
		t.Fatalf("expected failure audit result, got %q", recorder.events[0].Result)
	}
}
