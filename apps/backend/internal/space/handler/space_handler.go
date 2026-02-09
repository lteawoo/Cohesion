package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

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
	mux.Handle("/api/spaces", web.Handler(h.handleSpaces))
	mux.Handle("/api/spaces/", web.Handler(h.handleSpaceByID))
}

// handleSpaces는 HTTP 메서드에 따라 요청을 라우팅합니다
func (h *Handler) handleSpaces(w http.ResponseWriter, r *http.Request) *web.Error {
	switch r.Method {
	case http.MethodGet:
		return h.handleGetSpaces(w, r)
	case http.MethodPost:
		return h.handleCreateSpace(w, r)
	default:
		return &web.Error{
			Code:    http.StatusMethodNotAllowed,
			Message: "Method not allowed",
			Err:     nil,
		}
	}
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

// handleCreateSpace는 새로운 Space를 생성합니다
func (h *Handler) handleCreateSpace(w http.ResponseWriter, r *http.Request) *web.Error {
	// 요청 본문 파싱
	var req space.CreateSpaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return &web.Error{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
			Err:     err,
		}
	}
	defer r.Body.Close()

	// 서비스 호출
	createdSpace, err := h.spaceService.CreateSpace(r.Context(), &req)
	if err != nil {
		// 에러 타입에 따라 상태 코드 결정
		statusCode := http.StatusInternalServerError
		if strings.Contains(err.Error(), "validation failed") {
			statusCode = http.StatusBadRequest
		} else if strings.Contains(err.Error(), "already exists") {
			statusCode = http.StatusConflict
		} else if strings.Contains(err.Error(), "does not exist") {
			statusCode = http.StatusBadRequest
		}

		return &web.Error{
			Code:    statusCode,
			Message: err.Error(),
			Err:     err,
		}
	}

	// 응답 생성
	response := space.CreateSpaceResponse{
		ID:        createdSpace.ID,
		SpaceName: createdSpace.SpaceName,
		SpacePath: createdSpace.SpacePath,
		Message:   "Space created successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)

	return nil
}

// handleSpaceByID는 /api/spaces/{id} 요청을 처리합니다
func (h *Handler) handleSpaceByID(w http.ResponseWriter, r *http.Request) *web.Error {
	// URL에서 ID 추출: /api/spaces/123 -> 123
	idStr := strings.TrimPrefix(r.URL.Path, "/api/spaces/")

	// 빈 ID 체크 (예: /api/spaces/)
	if idStr == "" {
		return &web.Error{
			Code:    http.StatusBadRequest,
			Message: "Space ID is required",
			Err:     nil,
		}
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return &web.Error{
			Code:    http.StatusBadRequest,
			Message: "Invalid space ID format",
			Err:     err,
		}
	}

	switch r.Method {
	case http.MethodDelete:
		return h.handleDeleteSpace(w, r, id)
	default:
		return &web.Error{
			Code:    http.StatusMethodNotAllowed,
			Message: "Method not allowed",
			Err:     nil,
		}
	}
}

// handleDeleteSpace는 Space를 삭제합니다
func (h *Handler) handleDeleteSpace(w http.ResponseWriter, r *http.Request, id int64) *web.Error {
	if err := h.spaceService.DeleteSpace(r.Context(), id); err != nil {
		statusCode := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
		} else if strings.Contains(err.Error(), "invalid") {
			statusCode = http.StatusBadRequest
		}

		return &web.Error{
			Code:    statusCode,
			Message: err.Error(),
			Err:     err,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Space deleted successfully",
	})

	return nil
}
