package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"taeu.kr/cohesion/internal/space"
)

type fakeRenameSpaceStore struct {
	spacesByID map[int64]*space.Space
}

func (f *fakeRenameSpaceStore) GetAll(context.Context) ([]*space.Space, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeRenameSpaceStore) GetByName(_ context.Context, name string) (*space.Space, error) {
	for _, item := range f.spacesByID {
		if item.SpaceName == name {
			return cloneSpace(item), nil
		}
	}
	return nil, errors.New("space not found")
}

func (f *fakeRenameSpaceStore) GetByID(_ context.Context, id int64) (*space.Space, error) {
	item, ok := f.spacesByID[id]
	if !ok {
		return nil, errors.New("space not found")
	}
	return cloneSpace(item), nil
}

func (f *fakeRenameSpaceStore) Create(context.Context, *space.CreateSpaceRequest) (*space.Space, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeRenameSpaceStore) Delete(context.Context, int64) error {
	return errors.New("not implemented")
}

func (f *fakeRenameSpaceStore) Update(_ context.Context, id int64, req *space.UpdateSpaceRequest) (*space.Space, error) {
	item, ok := f.spacesByID[id]
	if !ok {
		return nil, errors.New("space not found")
	}
	if req.SpaceName != nil {
		item.SpaceName = *req.SpaceName
	}
	return cloneSpace(item), nil
}

func cloneSpace(item *space.Space) *space.Space {
	if item == nil {
		return nil
	}
	cloned := *item
	return &cloned
}

func TestHandleSpaceByID_RenameSpace(t *testing.T) {
	store := &fakeRenameSpaceStore{
		spacesByID: map[int64]*space.Space{
			1: {ID: 1, SpaceName: "Alpha", SpacePath: "/tmp/alpha"},
		},
	}
	handler := NewHandler(space.NewService(store), nil, nil)

	body := bytes.NewBufferString(`{"space_name":"Alpha Renamed"}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/spaces/1", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	if webErr := handler.handleSpaceByID(rec, req); webErr != nil {
		t.Fatalf("unexpected web error: %+v", webErr)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp struct {
		ID        int64  `json:"id"`
		SpaceName string `json:"space_name"`
		Message   string `json:"message"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.SpaceName != "Alpha Renamed" {
		t.Fatalf("expected renamed space name, got %q", resp.SpaceName)
	}
	if store.spacesByID[1].SpaceName != "Alpha Renamed" {
		t.Fatalf("expected store to persist renamed space, got %q", store.spacesByID[1].SpaceName)
	}
}

func TestHandleSpaceByID_RenameSpaceRejectsInvalidOrDuplicate(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		expectedStatus int
		expectedName   string
	}{
		{
			name:           "empty name",
			body:           `{"space_name":"   "}`,
			expectedStatus: http.StatusBadRequest,
			expectedName:   "Alpha",
		},
		{
			name:           "duplicate name",
			body:           `{"space_name":"Beta"}`,
			expectedStatus: http.StatusConflict,
			expectedName:   "Alpha",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeRenameSpaceStore{
				spacesByID: map[int64]*space.Space{
					1: {ID: 1, SpaceName: "Alpha", SpacePath: "/tmp/alpha"},
					2: {ID: 2, SpaceName: "Beta", SpacePath: "/tmp/beta"},
				},
			}
			handler := NewHandler(space.NewService(store), nil, nil)

			req := httptest.NewRequest(http.MethodPatch, "/api/spaces/1", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			webErr := handler.handleSpaceByID(rec, req)
			if webErr == nil {
				t.Fatal("expected web error")
			}
			if webErr.Code != tc.expectedStatus {
				t.Fatalf("expected status %d, got %d", tc.expectedStatus, webErr.Code)
			}
			if store.spacesByID[1].SpaceName != tc.expectedName {
				t.Fatalf("expected stored name %q, got %q", tc.expectedName, store.spacesByID[1].SpaceName)
			}
		})
	}
}
