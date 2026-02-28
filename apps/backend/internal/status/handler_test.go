package status

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"taeu.kr/cohesion/internal/config"
	"taeu.kr/cohesion/internal/space"
)

type statusTestSpaceStore struct {
	spaces []*space.Space
	err    error
}

func (s *statusTestSpaceStore) GetAll(context.Context) ([]*space.Space, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.spaces == nil {
		return []*space.Space{}, nil
	}
	return s.spaces, nil
}

func (s *statusTestSpaceStore) GetByName(context.Context, string) (*space.Space, error) {
	return nil, errors.New("not implemented")
}

func (s *statusTestSpaceStore) GetByID(context.Context, int64) (*space.Space, error) {
	return nil, errors.New("not implemented")
}

func (s *statusTestSpaceStore) Create(context.Context, *space.CreateSpaceRequest) (*space.Space, error) {
	return nil, errors.New("not implemented")
}

func (s *statusTestSpaceStore) Delete(context.Context, int64) error {
	return errors.New("not implemented")
}

func TestHandleStatus_ExcludesSMBProtocol(t *testing.T) {
	originalConf := config.Conf
	defer func() { config.Conf = originalConf }()

	config.Conf.Server.WebdavEnabled = true
	config.Conf.Server.FtpEnabled = false
	config.Conf.Server.FtpPort = 2121
	config.Conf.Server.SftpEnabled = false
	config.Conf.Server.SftpPort = 2222

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	spaceService := space.NewService(&statusTestSpaceStore{})
	handler := NewHandler(db, spaceService, "3000")

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	recorder := httptest.NewRecorder()

	if webErr := handler.handleStatus(recorder, req); webErr != nil {
		t.Fatalf("handleStatus returned error: %+v", webErr)
	}

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	var resp StatusResponse
	if err := json.NewDecoder(recorder.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	for _, required := range []string{"http", "webdav", "ftp", "sftp"} {
		if _, exists := resp.Protocols[required]; !exists {
			t.Fatalf("expected protocol %q in response", required)
		}
	}

	if _, exists := resp.Protocols["smb"]; exists {
		t.Fatalf("did not expect protocol %q in response", "smb")
	}
}

func TestHandleStatus_MethodNotAllowed(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	spaceService := space.NewService(&statusTestSpaceStore{})
	handler := NewHandler(db, spaceService, "3000")

	req := httptest.NewRequest(http.MethodPost, "/api/status", nil)
	recorder := httptest.NewRecorder()

	webErr := handler.handleStatus(recorder, req)
	if webErr == nil {
		t.Fatal("expected method not allowed error, got nil")
	}
	if webErr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, webErr.Code)
	}
}
