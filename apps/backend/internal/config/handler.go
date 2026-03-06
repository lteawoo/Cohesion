package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
	"taeu.kr/cohesion/internal/audit"
	"taeu.kr/cohesion/internal/auth"
	"taeu.kr/cohesion/internal/platform/web"
)

var configFileName string
var configFilePath string

func SetConfig(goEnv string) {
	log.Info().Msgf("Loading configuration for environment: %s", goEnv)

	viper.Reset()
	viper.SetConfigType("yaml")

	configSearchPaths := resolveConfigSearchPaths()
	for _, path := range configSearchPaths {
		viper.AddConfigPath(path)
	}

	if goEnv == "production" {
		configFileName = "config.prod"
	} else {
		configFileName = "config.dev"
	}
	viper.SetConfigName(configFileName)

	err := viper.ReadInConfig()
	if err != nil {
		var notFound viper.ConfigFileNotFoundError
		if errors.As(err, &notFound) {
			createdPath, createErr := createDefaultConfigFile(configSearchPaths, configFileName, goEnv)
			if createErr != nil {
				log.Fatal().Err(createErr).Strs("search_paths", configSearchPaths).Msg("Failed to create default config file")
			}
			log.Info().Str("path", createdPath).Msg("Config file not found. Created default config file")
			if err = viper.ReadInConfig(); err != nil {
				log.Fatal().Err(err).Strs("search_paths", configSearchPaths).Msg("Failed to read config file")
			}
		} else {
			log.Fatal().Err(err).Strs("search_paths", configSearchPaths).Msg("Failed to read config file")
		}
	}

	configFilePath = viper.ConfigFileUsed()
	log.Info().Msgf("Config file loaded: %s", configFilePath)

	err = viper.Unmarshal(&Conf)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to unmarshal config")
	}
	applyServerDefaults(&Conf.Server)
}

func createDefaultConfigFile(searchPaths []string, fileName, goEnv string) (string, error) {
	if len(searchPaths) == 0 {
		return "", errors.New("no config search paths available")
	}

	targetDir := searchPaths[0]
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("create config directory: %w", err)
	}

	targetPath := filepath.Join(targetDir, fileName+".yaml")
	if _, err := os.Stat(targetPath); err == nil {
		return targetPath, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("check config file: %w", err)
	}

	defaultConf := defaultConfigForEnv(goEnv)
	data, err := yaml.Marshal(defaultConf)
	if err != nil {
		return "", fmt.Errorf("marshal default config: %w", err)
	}

	if err := os.WriteFile(targetPath, data, 0644); err != nil {
		return "", fmt.Errorf("write default config: %w", err)
	}

	return targetPath, nil
}

func defaultConfigForEnv(goEnv string) Config {
	config := Config{
		Server: Server{
			Port:          "3000",
			WebdavEnabled: true,
			FtpEnabled:    false,
			FtpPort:       2121,
			SftpEnabled:   false,
			SftpPort:      2222,
		},
		AuditLogRetentionDays: 0,
		Datasource: Datasource{
			URL: "data/cohesion.db",
		},
	}

	if goEnv != "production" {
		config.Datasource.URL = "dist/data/cohesion_dev.db"
	}

	return config
}

func resolveConfigSearchPaths() []string {
	paths := make([]string, 0, 3)
	seen := make(map[string]struct{})

	addPath := func(path string) {
		clean := filepath.Clean(path)
		if _, ok := seen[clean]; ok {
			return
		}
		seen[clean] = struct{}{}
		paths = append(paths, clean)
	}

	if exePath, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exePath); err == nil {
			exePath = resolved
		}
		exeDir := filepath.Dir(exePath)
		addPath(filepath.Join(exeDir, "config"))
		addPath(filepath.Join(exeDir, "..", "config"))
	}

	if cwd, err := os.Getwd(); err == nil {
		addPath(filepath.Join(cwd, "config"))
	}

	return paths
}

func ConfigDir() string {
	if configFilePath == "" {
		return ""
	}
	return filepath.Dir(configFilePath)
}

// SaveConfig는 설정을 YAML 파일에 저장합니다
func SaveConfig() error {
	data, err := yaml.Marshal(&Conf)
	if err != nil {
		return err
	}

	err = os.WriteFile(configFilePath, data, 0644)
	if err != nil {
		return err
	}

	log.Info().Msgf("Configuration saved to %s", configFilePath)
	return nil
}

// Handler는 config API 핸들러입니다
type Handler struct {
	auditRecorder audit.Recorder
}

type PublicConfigResponse struct {
	Server                Server `json:"server"`
	AuditLogRetentionDays int    `json:"auditLogRetentionDays"`
}

type UpdateConfigRequest struct {
	Server                Server `json:"server"`
	AuditLogRetentionDays *int   `json:"auditLogRetentionDays"`
}

func applyServerDefaults(server *Server) {
	if server == nil {
		return
	}
}

func validateServerConfig(server Server) *web.Error {
	applyServerDefaults(&server)

	serverPort := strings.TrimSpace(server.Port)
	if serverPort == "" {
		return &web.Error{Code: http.StatusBadRequest, Message: "server.port is required"}
	}

	webPort, err := strconv.Atoi(serverPort)
	if err != nil || webPort < 1 || webPort > 65535 {
		return &web.Error{Code: http.StatusBadRequest, Message: "server.port must be an integer between 1 and 65535"}
	}

	if server.FtpEnabled {
		if server.FtpPort < 1 || server.FtpPort > 65535 {
			return &web.Error{Code: http.StatusBadRequest, Message: "server.ftpPort must be an integer between 1 and 65535 when ftp is enabled"}
		}
		if server.FtpPort == webPort {
			return &web.Error{Code: http.StatusBadRequest, Message: "server.ftpPort must be different from server.port"}
		}
		if server.SftpEnabled && server.FtpPort == server.SftpPort {
			return &web.Error{Code: http.StatusBadRequest, Message: "server.ftpPort must be different from server.sftpPort"}
		}
	}

	if server.SftpEnabled {
		if server.SftpPort < 1 || server.SftpPort > 65535 {
			return &web.Error{Code: http.StatusBadRequest, Message: "server.sftpPort must be an integer between 1 and 65535 when sftp is enabled"}
		}
		if server.SftpPort == webPort {
			return &web.Error{Code: http.StatusBadRequest, Message: "server.sftpPort must be different from server.port"}
		}
	}

	return nil
}

func validateAuditLogRetentionDays(value int) *web.Error {
	if value < 0 {
		return &web.Error{Code: http.StatusBadRequest, Message: "auditLogRetentionDays must be greater than or equal to 0"}
	}
	return nil
}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) SetAuditRecorder(recorder audit.Recorder) {
	h.auditRecorder = recorder
}

// RegisterRoutes는 라우트를 등록합니다
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/config", web.Handler(h.GetConfig))
	mux.Handle("PUT /api/config", web.Handler(h.UpdateConfig))
}

// GetConfig는 현재 설정을 반환합니다
func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) *web.Error {
	w.Header().Set("Content-Type", "application/json")
	response := PublicConfigResponse{
		Server:                Conf.Server,
		AuditLogRetentionDays: Conf.AuditLogRetentionDays,
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		return &web.Error{Err: err, Code: http.StatusInternalServerError, Message: "Failed to encode config"}
	}
	return nil
}

// UpdateConfig는 설정을 업데이트합니다
func (h *Handler) UpdateConfig(w http.ResponseWriter, r *http.Request) *web.Error {
	var req UpdateConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return &web.Error{Err: err, Code: http.StatusBadRequest, Message: "Invalid config format"}
	}

	req.Server.Port = strings.TrimSpace(req.Server.Port)
	applyServerDefaults(&req.Server)
	if validationErr := validateServerConfig(req.Server); validationErr != nil {
		h.recordAudit(r, audit.Event{
			Action: "config.update",
			Result: audit.ResultFailure,
			Target: "server",
			Metadata: map[string]any{
				"reason": "validation_failed",
			},
		})
		return validationErr
	}

	nextAuditLogRetentionDays := Conf.AuditLogRetentionDays
	if req.AuditLogRetentionDays != nil {
		if validationErr := validateAuditLogRetentionDays(*req.AuditLogRetentionDays); validationErr != nil {
			h.recordAudit(r, audit.Event{
				Action: "config.update",
				Result: audit.ResultFailure,
				Target: "server",
				Metadata: map[string]any{
					"reason": "validation_failed",
				},
			})
			return validationErr
		}
		nextAuditLogRetentionDays = *req.AuditLogRetentionDays
	}

	// 설정 업데이트 (민감정보를 포함하는 datasource는 API로 변경하지 않음)
	before := map[string]any{
		"port":                  Conf.Server.Port,
		"webdavEnabled":         Conf.Server.WebdavEnabled,
		"ftpEnabled":            Conf.Server.FtpEnabled,
		"ftpPort":               Conf.Server.FtpPort,
		"sftpEnabled":           Conf.Server.SftpEnabled,
		"sftpPort":              Conf.Server.SftpPort,
		"auditLogRetentionDays": Conf.AuditLogRetentionDays,
	}
	Conf.Server = req.Server
	Conf.AuditLogRetentionDays = nextAuditLogRetentionDays

	// 파일에 저장
	if err := SaveConfig(); err != nil {
		h.recordAudit(r, audit.Event{
			Action: "config.update",
			Result: audit.ResultFailure,
			Target: "server",
			Metadata: map[string]any{
				"reason": "save_failed",
			},
		})
		return &web.Error{Err: err, Code: http.StatusInternalServerError, Message: "Failed to save config"}
	}
	after := map[string]any{
		"port":                  Conf.Server.Port,
		"webdavEnabled":         Conf.Server.WebdavEnabled,
		"ftpEnabled":            Conf.Server.FtpEnabled,
		"ftpPort":               Conf.Server.FtpPort,
		"sftpEnabled":           Conf.Server.SftpEnabled,
		"sftpPort":              Conf.Server.SftpPort,
		"auditLogRetentionDays": Conf.AuditLogRetentionDays,
	}
	h.recordAudit(r, audit.Event{
		Action: "config.update",
		Result: audit.ResultSuccess,
		Target: "server",
		Metadata: map[string]any{
			"before": before,
			"after":  after,
		},
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Configuration updated successfully",
	})
	return nil
}

func (h *Handler) recordAudit(r *http.Request, event audit.Event) {
	if h.auditRecorder == nil {
		return
	}
	if claims, ok := auth.ClaimsFromContext(r.Context()); ok {
		event.Actor = claims.Username
	}
	if event.RequestID == "" {
		event.RequestID = strings.TrimSpace(r.Header.Get("X-Request-Id"))
	}
	h.auditRecorder.RecordBestEffort(event)
}
