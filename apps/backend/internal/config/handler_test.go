package config

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testConfigState() Config {
	return Config{
		Server: Server{
			Port:          "3000",
			WebdavEnabled: true,
			FtpEnabled:    false,
			FtpPort:       2121,
			SftpEnabled:   false,
			SftpPort:      2222,
		},
		AuditLogRetentionDays: 0,
		Datasource:            Datasource{URL: "data/test.db"},
	}
}

func setupConfigHandlerState(t *testing.T) string {
	t.Helper()

	prevConf := Conf
	prevConfigFilePath := configFilePath
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	Conf = testConfigState()
	configFilePath = configPath
	if err := SaveConfig(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	t.Cleanup(func() {
		Conf = prevConf
		configFilePath = prevConfigFilePath
	})

	return configPath
}

func TestGetConfig_IncludesAuditLogRetentionDays(t *testing.T) {
	setupConfigHandlerState(t)
	Conf.AuditLogRetentionDays = 30

	handler := NewHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()

	if webErr := handler.GetConfig(rec, req); webErr != nil {
		t.Fatalf("get config returned error: %+v", webErr)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response PublicConfigResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.AuditLogRetentionDays != 30 {
		t.Fatalf("expected retention days 30, got %d", response.AuditLogRetentionDays)
	}
}

func TestUpdateConfig_PersistsAuditLogRetentionDays(t *testing.T) {
	configPath := setupConfigHandlerState(t)
	handler := NewHandler()

	payload := map[string]any{
		"server": map[string]any{
			"port":          "3300",
			"webdavEnabled": true,
			"ftpEnabled":    false,
			"ftpPort":       2121,
			"sftpEnabled":   false,
			"sftpPort":      2222,
		},
		"auditLogRetentionDays": 45,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	if webErr := handler.UpdateConfig(rec, req); webErr != nil {
		t.Fatalf("update config returned error: %+v", webErr)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if Conf.AuditLogRetentionDays != 45 {
		t.Fatalf("expected retention days 45, got %d", Conf.AuditLogRetentionDays)
	}

	saved, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if !strings.Contains(string(saved), "audit_log_retention_days: 45") {
		t.Fatalf("expected saved config to include retention days, got %s", string(saved))
	}
}

func TestUpdateConfig_RejectsNegativeAuditLogRetentionDays(t *testing.T) {
	setupConfigHandlerState(t)
	handler := NewHandler()

	body := bytes.NewBufferString(`{"server":{"port":"3000","webdavEnabled":true,"ftpEnabled":false,"ftpPort":2121,"sftpEnabled":false,"sftpPort":2222},"auditLogRetentionDays":-1}`)
	req := httptest.NewRequest(http.MethodPut, "/api/config", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	webErr := handler.UpdateConfig(rec, req)
	if webErr == nil {
		t.Fatal("expected validation error")
	}
	if webErr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, webErr.Code)
	}
	if Conf.AuditLogRetentionDays != 0 {
		t.Fatalf("expected retention days to remain unchanged, got %d", Conf.AuditLogRetentionDays)
	}
}

func TestUpdateConfig_PreservesAuditLogRetentionDaysWhenFieldIsOmitted(t *testing.T) {
	setupConfigHandlerState(t)
	Conf.AuditLogRetentionDays = 30
	handler := NewHandler()

	body := bytes.NewBufferString(`{"server":{"port":"3200","webdavEnabled":true,"ftpEnabled":false,"ftpPort":2121,"sftpEnabled":false,"sftpPort":2222}}`)
	req := httptest.NewRequest(http.MethodPut, "/api/config", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	if webErr := handler.UpdateConfig(rec, req); webErr != nil {
		t.Fatalf("update config returned error: %+v", webErr)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if Conf.Server.Port != "3200" {
		t.Fatalf("expected port to be updated, got %s", Conf.Server.Port)
	}
	if Conf.AuditLogRetentionDays != 30 {
		t.Fatalf("expected retention days to remain 30, got %d", Conf.AuditLogRetentionDays)
	}
}
