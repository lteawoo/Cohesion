package system

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
	"taeu.kr/cohesion/internal/config"
	"taeu.kr/cohesion/internal/platform/web"
)

// Handler는 system API 핸들러입니다
type Handler struct {
	restartChan chan bool
}

func NewHandler(restartChan chan bool) *Handler {
	return &Handler{
		restartChan: restartChan,
	}
}

// RegisterRoutes는 라우트를 등록합니다
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("POST /api/system/restart", web.Handler(h.RestartServer))
}

// RestartServer는 서버를 재시작합니다
func (h *Handler) RestartServer(w http.ResponseWriter, r *http.Request) *web.Error {
	log.Info().Msg("[System] Restart request received")

	// 응답 먼저 전송
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := map[string]interface{}{
		"message":  "Server is restarting...",
		"new_port": config.Conf.Server.Port,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("Failed to encode response")
	}

	// 응답을 즉시 flush (클라이언트에게 전송)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	// 약간의 지연 후 재시작 신호 전송 (응답이 완전히 전송될 시간 확보)
	go func() {
		time.Sleep(500 * time.Millisecond)
		log.Info().Msg("[System] Sending restart signal...")
		h.restartChan <- true
	}()

	return nil
}
