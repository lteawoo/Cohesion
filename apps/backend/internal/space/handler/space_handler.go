package handler

import (
	"encoding/json"
	"net/http"

	"taeu.kr/cohesion/internal/platform/web"
	"taeu.kr/cohesion/internal/space"
)

type Handler struct {
	spaceService *space.Service
}

// 의존성 주입 생성자 생성
func NewHandler(spaceService *space.Service) *Handler {
	return &Handler{
		spaceService: spaceService,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("/api/spaces", web.Handler(h.handleGetSpaces))
}

// Space 목록 조회
func (h *Handler) handleGetSpaces(w http.ResponseWriter, r *http.Request) *web.Error {
	spaces, err := h.spaceService.GetAllSpaces(r.Context())

	if err != nil {
		return &web.Error{
			Code:    http.StatusInternalServerError,
			Message: "Failed to get spaces",
			Err:     err,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(spaces)

	return nil
}
