package config

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
	"taeu.kr/cohesion/internal/platform/web"
)

var configFileName string
var configFilePath string

func SetConfig(goEnv string) {
	log.Info().Msgf("Loading configuration for environment: %s", goEnv)

	viper.AddConfigPath("config")
	viper.SetConfigType("yaml")

	if goEnv == "production" {
		configFileName = "config.prod"
	} else {
		configFileName = "config.dev"
	}
	viper.SetConfigName(configFileName)

	err := viper.ReadInConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to read config file")
	}

	configFilePath = viper.ConfigFileUsed()
	log.Info().Msgf("Config file loaded: %s", configFilePath)

	err = viper.Unmarshal(&Conf)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to unmarshal config")
	}
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
	if err := json.NewEncoder(w).Encode(Conf); err != nil {
		return &web.Error{Err: err, Code: http.StatusInternalServerError, Message: "Failed to encode config"}
	}
	return nil
}

// UpdateConfig는 설정을 업데이트합니다
func (h *Handler) UpdateConfig(w http.ResponseWriter, r *http.Request) *web.Error {
	var newConfig Config
	if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
		return &web.Error{Err: err, Code: http.StatusBadRequest, Message: "Invalid config format"}
	}

	// 설정 업데이트
	Conf = newConfig

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
