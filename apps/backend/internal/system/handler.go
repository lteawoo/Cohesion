package system

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"taeu.kr/cohesion/internal/audit"
	"taeu.kr/cohesion/internal/auth"
	"taeu.kr/cohesion/internal/config"
	"taeu.kr/cohesion/internal/platform/web"
)

type Meta struct {
	Version   string
	Commit    string
	BuildDate string
}

// Handler는 system API 핸들러입니다
type Handler struct {
	restartChan   chan RestartRequest
	shutdownChan  chan struct{}
	meta          Meta
	statusStore   *StatusStore
	updateChecker *UpdateChecker
	updateManager *SelfUpdateManager
	auditRecorder audit.Recorder
}

func NewHandler(restartChan chan RestartRequest, shutdownChan chan struct{}, meta Meta, statusStore *StatusStore) *Handler {
	version := strings.TrimSpace(meta.Version)
	if version == "" {
		version = "dev"
	}
	if statusStore == nil {
		statusStore = NewStatusStore()
	}

	return &Handler{
		restartChan:  restartChan,
		shutdownChan: shutdownChan,
		statusStore:  statusStore,
		meta: Meta{
			Version:   version,
			Commit:    strings.TrimSpace(meta.Commit),
			BuildDate: strings.TrimSpace(meta.BuildDate),
		},
		updateChecker: NewUpdateChecker(UpdateCheckerConfig{
			RepoOwner:      "lteawoo",
			RepoName:       "Cohesion",
			CacheTTL:       10 * time.Minute,
			RequestTimeout: 3 * time.Second,
		}),
		updateManager: NewSelfUpdateManager(SelfUpdateManagerConfig{
			RepoOwner:      "lteawoo",
			RepoName:       "Cohesion",
			RequestTimeout: 30 * time.Second,
		}, shutdownChan, statusStore),
	}
}

func (h *Handler) SetAuditRecorder(recorder audit.Recorder) {
	h.auditRecorder = recorder
}

// RegisterRoutes는 라우트를 등록합니다
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/system/version", web.Handler(h.GetVersion))
	mux.Handle("GET /api/system/update-check", web.Handler(h.GetUpdateCheck))
	mux.Handle("GET /api/system/update/status", web.Handler(h.GetUpdateStatus))
	mux.Handle("POST /api/system/update/start", web.Handler(h.StartUpdate))
	mux.Handle("POST /api/system/restart", web.Handler(h.RestartServer))
}

func (h *Handler) GetVersion(w http.ResponseWriter, r *http.Request) *web.Error {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"version":   h.meta.Version,
		"commit":    h.meta.Commit,
		"buildDate": h.meta.BuildDate,
	}); err != nil {
		return &web.Error{Err: err, Code: http.StatusInternalServerError, Message: "Failed to encode version response"}
	}
	return nil
}

func (h *Handler) GetUpdateCheck(w http.ResponseWriter, r *http.Request) *web.Error {
	w.Header().Set("Content-Type", "application/json")

	result, err := h.updateChecker.Check(r.Context(), h.meta.Version)
	if err != nil {
		log.Warn().Err(err).Msg("[System] update check failed")
		result.Error = "Failed to check latest release"
	}

	if err := json.NewEncoder(w).Encode(result); err != nil {
		return &web.Error{Err: err, Code: http.StatusInternalServerError, Message: "Failed to encode update check response"}
	}
	return nil
}

func (h *Handler) GetUpdateStatus(w http.ResponseWriter, r *http.Request) *web.Error {
	w.Header().Set("Content-Type", "application/json")
	status, err := h.statusStore.Load()
	if err != nil {
		log.Warn().Err(err).Msg("[System] failed to load persisted lifecycle status")
		status = h.updateManager.GetStatus()
	}
	if err := json.NewEncoder(w).Encode(status); err != nil {
		return &web.Error{Err: err, Code: http.StatusInternalServerError, Message: "Failed to encode update status response"}
	}
	return nil
}

func (h *Handler) StartUpdate(w http.ResponseWriter, r *http.Request) *web.Error {
	w.Header().Set("Content-Type", "application/json")
	force := false
	switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get("force"))) {
	case "1", "true", "yes", "y":
		force = true
	}

	if err := h.updateManager.Start(h.meta.Version, force); err != nil {
		h.recordAudit(r, audit.Event{
			Action: "system.update.start",
			Result: audit.ResultFailure,
			Target: "self-update",
			Metadata: map[string]any{
				"force":  force,
				"reason": "start_failed",
			},
		})
		switch {
		case errors.Is(err, ErrSelfUpdateUnsupportedBuild):
			return &web.Error{Err: err, Code: http.StatusBadRequest, Message: "Self-update is only available in release builds"}
		case errors.Is(err, ErrSelfUpdateAlreadyRunning):
			return &web.Error{Err: err, Code: http.StatusConflict, Message: "Update is already in progress"}
		default:
			return &web.Error{Err: err, Code: http.StatusInternalServerError, Message: "Failed to start update"}
		}
	}
	h.recordAudit(r, audit.Event{
		Action: "system.update.start",
		Result: audit.ResultSuccess,
		Target: "self-update",
		Metadata: map[string]any{
			"force": force,
		},
	})

	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"message": "Update started",
	}); err != nil {
		return &web.Error{Err: err, Code: http.StatusInternalServerError, Message: "Failed to encode update start response"}
	}
	return nil
}

// RestartServer는 서버를 재시작합니다
func (h *Handler) RestartServer(w http.ResponseWriter, r *http.Request) *web.Error {
	log.Info().Msg("[System] Restart request received")

	actor := ""
	if claims, ok := auth.ClaimsFromContext(r.Context()); ok {
		actor = claims.Username
	}
	restartRequest := RestartRequest{
		Actor:     actor,
		RequestID: strings.TrimSpace(r.Header.Get("X-Request-Id")),
		Port:      strings.TrimSpace(config.Conf.Server.Port),
	}
	if _, err := h.statusStore.MarkRestartAccepted(restartRequest, h.meta.Version); err != nil {
		switch {
		case errors.Is(err, ErrRestartAlreadyInProgress):
			return &web.Error{Err: err, Code: http.StatusConflict, Message: "Restart is already in progress"}
		default:
			return &web.Error{Err: err, Code: http.StatusInternalServerError, Message: "Failed to persist restart request"}
		}
	}

	h.recordAudit(r, audit.Event{
		Action: "system.restart.accepted",
		Result: audit.ResultSuccess,
		Target: "server",
		Metadata: map[string]any{
			"port": config.Conf.Server.Port,
		},
	})

	// 응답 먼저 전송
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)

	response := map[string]interface{}{
		"status":   "accepted",
		"message":  "Restart request accepted",
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
		h.restartChan <- restartRequest
	}()

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
