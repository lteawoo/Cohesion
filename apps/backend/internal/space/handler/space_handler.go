package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"taeu.kr/cohesion/internal/account"
	"taeu.kr/cohesion/internal/auth"
	"taeu.kr/cohesion/internal/browse"
	"taeu.kr/cohesion/internal/platform/web"
	"taeu.kr/cohesion/internal/space"
)

// BrowseService 인터페이스 정의 (browse handler 의존성)
type BrowseService interface {
	ListDirectory(onlyDir bool, path string) ([]browse.FileInfo, error)
}

type SpaceAccessService interface {
	CanAccessSpaceByID(ctx context.Context, username string, spaceID int64, required account.Permission) (bool, error)
}

type Handler struct {
	spaceService      *space.Service
	trashService      *space.TrashService
	browseService     BrowseService
	accountService    SpaceAccessService
	ticketMu          sync.Mutex
	downloadTickets   map[string]downloadTicket
	downloadTicketTTL time.Duration
}

// 의존성 주입 생성자 생성
func NewHandler(spaceService *space.Service, browseService BrowseService, accountService SpaceAccessService, trashService ...*space.TrashService) *Handler {
	var resolvedTrashService *space.TrashService
	if len(trashService) > 0 {
		resolvedTrashService = trashService[0]
	}

	return &Handler{
		spaceService:      spaceService,
		trashService:      resolvedTrashService,
		browseService:     browseService,
		accountService:    accountService,
		downloadTickets:   make(map[string]downloadTicket),
		downloadTicketTTL: 5 * time.Minute,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("/api/spaces", web.Handler(h.handleSpaces))
	mux.Handle("/api/spaces/", web.Handler(h.handleSpaceByID))
	mux.Handle("/api/search/files", web.Handler(h.handleSearchFiles))
	mux.Handle("/api/downloads/", web.Handler(h.handleDownloadByTicket))
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
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		return &web.Error{
			Code:    http.StatusUnauthorized,
			Message: "Unauthorized",
		}
	}

	spaces, err := h.spaceService.GetAllSpaces(r.Context())

	if err != nil {
		return &web.Error{
			Code:    http.StatusInternalServerError,
			Message: "Failed to get spaces",
			Err:     err,
		}
	}

	filteredSpaces := make([]*space.Space, 0, len(spaces))
	for _, item := range spaces {
		allowed, err := h.accountService.CanAccessSpaceByID(r.Context(), claims.Username, item.ID, account.PermissionRead)
		if err != nil {
			return &web.Error{
				Code:    http.StatusInternalServerError,
				Message: "Failed to evaluate space access",
				Err:     err,
			}
		}
		if allowed {
			filteredSpaces = append(filteredSpaces, item)
		}
	}

	type listSpaceResponse struct {
		ID            int64   `json:"id"`
		SpaceName     string  `json:"space_name"`
		SpaceDesc     *string `json:"space_desc,omitempty"`
		Icon          *string `json:"icon,omitempty"`
		SpaceCategory *string `json:"space_category,omitempty"`
	}
	response := make([]listSpaceResponse, 0, len(filteredSpaces))
	for _, item := range filteredSpaces {
		response = append(response, listSpaceResponse{
			ID:            item.ID,
			SpaceName:     item.SpaceName,
			SpaceDesc:     item.SpaceDesc,
			Icon:          item.Icon,
			SpaceCategory: item.SpaceCategory,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)

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
		message := "Failed to create Space"
		if strings.Contains(err.Error(), "validation failed") {
			statusCode = http.StatusBadRequest
			message = "Invalid Space request"
		} else if strings.Contains(err.Error(), "already exists") {
			statusCode = http.StatusConflict
			message = "Space already exists"
		} else if strings.Contains(err.Error(), "does not exist") {
			statusCode = http.StatusBadRequest
			message = "Space path does not exist"
		}

		return &web.Error{
			Code:    statusCode,
			Message: message,
			Err:     err,
		}
	}

	// 응답 생성
	response := space.CreateSpaceResponse{
		ID:        createdSpace.ID,
		SpaceName: createdSpace.SpaceName,
		Message:   "Space created successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)

	return nil
}

// handleSpaceByID는 /api/spaces/{id} 요청을 처리합니다
func (h *Handler) handleSpaceByID(w http.ResponseWriter, r *http.Request) *web.Error {
	// /api/spaces (슬래시 없음) 요청은 handleSpaces로 리다이렉트
	if r.URL.Path == "/api/spaces" {
		return h.handleSpaces(w, r)
	}

	// URL 파싱: /api/spaces/{id} 또는 /api/spaces/{id}/browse
	pathParts := strings.TrimPrefix(r.URL.Path, "/api/spaces/")
	parts := strings.Split(pathParts, "/")

	// 빈 ID 체크
	if len(parts) == 0 || parts[0] == "" {
		return &web.Error{
			Code:    http.StatusBadRequest,
			Message: "Space ID is required",
			Err:     nil,
		}
	}

	// Space ID 파싱
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return &web.Error{
			Code:    http.StatusBadRequest,
			Message: "Invalid space ID format",
			Err:     err,
		}
	}

	// 액션 확인 (/api/spaces/{id}/browse)
	if len(parts) > 1 && parts[1] == "browse" {
		return h.handleSpaceBrowse(w, r, id)
	}

	// 파일 작업 (/api/spaces/{id}/files/{action})
	if len(parts) > 2 && parts[1] == "files" {
		return h.handleSpaceFiles(w, r, id, parts[2])
	}

	// 기존 로직 (/api/spaces/{id})
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
		message := "Failed to delete Space"
		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			message = "Space not found"
		} else if strings.Contains(err.Error(), "invalid") {
			statusCode = http.StatusBadRequest
			message = "Invalid Space request"
		}

		return &web.Error{
			Code:    statusCode,
			Message: message,
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

// isPathWithinSpace는 경로가 Space 내부에 있는지 검증 (디렉토리 트래버셜 방지)
func isPathWithinSpace(path, spacePath string) bool {
	cleanPath := filepath.Clean(path)
	cleanSpace := filepath.Clean(spacePath)

	// 상대 경로 계산
	rel, err := filepath.Rel(cleanSpace, cleanPath)
	if err != nil {
		return false
	}

	// ".."로 시작하면 Space 외부 경로
	return !strings.HasPrefix(rel, "..")
}

// handleSpaceBrowse는 Space 내부 디렉토리 탐색을 처리
func (h *Handler) handleSpaceBrowse(w http.ResponseWriter, r *http.Request, spaceID int64) *web.Error {
	// Space 조회
	ctx := r.Context()
	spaceData, err := h.spaceService.GetSpaceByID(ctx, spaceID)
	if err != nil {
		return &web.Error{
			Code:    http.StatusNotFound,
			Message: fmt.Sprintf("Space not found: %v", err),
			Err:     err,
		}
	}

	// 상대 경로 가져오기
	relativePath := r.URL.Query().Get("path")
	if err := ensurePathOutsideTrash(relativePath); err != nil {
		return &web.Error{
			Code:    http.StatusForbidden,
			Message: "Access denied: invalid path",
			Err:     err,
		}
	}

	// 절대 경로 계산
	absolutePath := filepath.Join(spaceData.SpacePath, relativePath)

	// 경로 검증 (디렉토리 트래버셜 방지)
	if !isPathWithinSpace(absolutePath, spaceData.SpacePath) {
		return &web.Error{
			Code:    http.StatusForbidden,
			Message: "Access denied: path is outside of Space",
		}
	}

	// 디렉토리 목록 조회
	files, err := h.browseService.ListDirectory(false, absolutePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &web.Error{
				Code:    http.StatusNotFound,
				Message: "Directory not found",
				Err:     err,
			}
		}
		if os.IsPermission(err) {
			return &web.Error{
				Code:    http.StatusForbidden,
				Message: "Permission denied",
				Err:     err,
			}
		}
		return &web.Error{
			Code:    http.StatusInternalServerError,
			Message: "Failed to list directory",
			Err:     err,
		}
	}

	for i := range files {
		relative, relErr := filepath.Rel(spaceData.SpacePath, files[i].Path)
		if relErr != nil {
			return &web.Error{
				Code:    http.StatusInternalServerError,
				Message: "Failed to normalize browse path",
				Err:     relErr,
			}
		}
		files[i].Path = filepath.ToSlash(relative)
	}

	// JSON 응답
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(files); err != nil {
		return &web.Error{
			Code:    http.StatusInternalServerError,
			Message: "Failed to encode response",
			Err:     err,
		}
	}

	return nil
}
