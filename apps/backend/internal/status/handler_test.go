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
	"taeu.kr/cohesion/internal/smb"
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

type statusTestSMBReadiness struct {
	readiness smb.Readiness
}

func (s statusTestSMBReadiness) Readiness() smb.Readiness {
	return s.readiness
}

func TestHandleStatus_IncludesSMBProtocol(t *testing.T) {
	originalConf := config.Conf
	defer func() { config.Conf = originalConf }()

	config.Conf.Server.WebdavEnabled = true
	config.Conf.Server.FtpEnabled = false
	config.Conf.Server.FtpPort = 2121
	config.Conf.Server.SftpEnabled = false
	config.Conf.Server.SftpPort = 2222
	config.Conf.Server.SmbEnabled = false
	config.Conf.Server.SmbPort = 1445
	config.Conf.Server.SmbRolloutPhase = config.SMBRolloutPhaseReadOnly

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	spaceService := space.NewService(&statusTestSpaceStore{})
	handler := NewHandler(db, spaceService, nil, "3000")

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

	for _, required := range []string{"http", "webdav", "ftp", "sftp", "smb"} {
		if _, exists := resp.Protocols[required]; !exists {
			t.Fatalf("expected protocol %q in response", required)
		}
	}

	if resp.Protocols["smb"].EndpointMode != config.SMBEndpointModeDirect {
		t.Fatalf("expected endpoint mode %q, got %q", config.SMBEndpointModeDirect, resp.Protocols["smb"].EndpointMode)
	}
	if resp.Protocols["smb"].Port != "1445" {
		t.Fatalf("expected smb port %q, got %q", "1445", resp.Protocols["smb"].Port)
	}
	if resp.Protocols["smb"].RolloutPhase != config.SMBRolloutPhaseReadOnly {
		t.Fatalf("expected rollout phase %q, got %q", config.SMBRolloutPhaseReadOnly, resp.Protocols["smb"].RolloutPhase)
	}
	if resp.Protocols["smb"].PolicySource != "config" {
		t.Fatalf("expected policy source %q, got %q", "config", resp.Protocols["smb"].PolicySource)
	}
	if resp.Protocols["smb"].MinVersion != config.DefaultSMBMinVersion {
		t.Fatalf("expected min version %q, got %q", config.DefaultSMBMinVersion, resp.Protocols["smb"].MinVersion)
	}
	if resp.Protocols["smb"].MaxVersion != config.DefaultSMBMaxVersion {
		t.Fatalf("expected max version %q, got %q", config.DefaultSMBMaxVersion, resp.Protocols["smb"].MaxVersion)
	}
}

func TestHandleStatus_MethodNotAllowed(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	spaceService := space.NewService(&statusTestSpaceStore{})
	handler := NewHandler(db, spaceService, nil, "3000")

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

func TestHandleStatus_UsesSMBReadinessProviderState(t *testing.T) {
	originalConf := config.Conf
	defer func() { config.Conf = originalConf }()

	config.Conf.Server.WebdavEnabled = true
	config.Conf.Server.FtpEnabled = false
	config.Conf.Server.SftpEnabled = false
	config.Conf.Server.SmbEnabled = true
	config.Conf.Server.SmbPort = 1445
	config.Conf.Server.SmbRolloutPhase = config.SMBRolloutPhaseReadOnly

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	spaceService := space.NewService(&statusTestSpaceStore{})
	handler := NewHandler(db, spaceService, statusTestSMBReadiness{
		readiness: smb.Readiness{
			State:        smb.StateUnhealthy,
			Reason:       smb.ReasonRuntimeNotReady,
			Message:      "SMB readonly 프로토콜 준비 안됨",
			RolloutPhase: config.SMBRolloutPhaseReadOnly,
			PolicySource: "config",
			BindReady:    true,
			RuntimeReady: false,
		},
	}, "3000")

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	recorder := httptest.NewRecorder()

	if webErr := handler.handleStatus(recorder, req); webErr != nil {
		t.Fatalf("handleStatus returned error: %+v", webErr)
	}

	var resp StatusResponse
	if err := json.NewDecoder(recorder.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Protocols["smb"].Status != "unhealthy" {
		t.Fatalf("expected smb unhealthy, got %q", resp.Protocols["smb"].Status)
	}
	if resp.Protocols["smb"].Message != "SMB readonly 프로토콜 준비 안됨" {
		t.Fatalf("unexpected smb message: %q", resp.Protocols["smb"].Message)
	}
	if resp.Protocols["smb"].Reason != smb.ReasonRuntimeNotReady {
		t.Fatalf("unexpected smb reason: %q", resp.Protocols["smb"].Reason)
	}
	if resp.Protocols["smb"].Port != "1445" {
		t.Fatalf("expected smb port %q, got %q", "1445", resp.Protocols["smb"].Port)
	}
	if resp.Protocols["smb"].RolloutPhase != config.SMBRolloutPhaseReadOnly {
		t.Fatalf("expected rollout phase %q, got %q", config.SMBRolloutPhaseReadOnly, resp.Protocols["smb"].RolloutPhase)
	}
	if resp.Protocols["smb"].PolicySource != "config" {
		t.Fatalf("expected policy source %q, got %q", "config", resp.Protocols["smb"].PolicySource)
	}
	if !resp.Protocols["smb"].BindReady {
		t.Fatal("expected bindReady=true")
	}
	if resp.Protocols["smb"].RuntimeReady {
		t.Fatal("expected runtimeReady=false")
	}
}
