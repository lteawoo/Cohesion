package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
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
			HttpEnabled:   true,
			WebdavEnabled: true,
			FtpEnabled:    false,
			FtpPort:       2121,
			SftpEnabled:   false,
			SftpPort:      22,
		},
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
type Handler struct{}

type PublicConfigResponse struct {
	Server Server `json:"server"`
}

type UpdateConfigRequest struct {
	Server Server `json:"server"`
}

func NewHandler() *Handler {
	return &Handler{}
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
		Server: Conf.Server,
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

	if req.Server.Port == "" {
		return &web.Error{Code: http.StatusBadRequest, Message: "server.port is required"}
	}

	// 설정 업데이트 (민감정보를 포함하는 datasource는 API로 변경하지 않음)
	Conf.Server = req.Server

	// 파일에 저장
	if err := SaveConfig(); err != nil {
		return &web.Error{Err: err, Code: http.StatusInternalServerError, Message: "Failed to save config"}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Configuration updated successfully",
	})
	return nil
}
