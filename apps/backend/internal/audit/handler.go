package audit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"taeu.kr/cohesion/internal/platform/web"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/audit/logs", web.Handler(h.handleListLogs))
	mux.Handle("GET /api/audit/logs/", web.Handler(h.handleLogByID))
}

func (h *Handler) handleListLogs(w http.ResponseWriter, r *http.Request) *web.Error {
	filter, err := parseListFilter(r)
	if err != nil {
		return &web.Error{Code: http.StatusBadRequest, Message: "Invalid audit log query", Err: err}
	}

	result, err := h.service.List(r.Context(), filter)
	if err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to list audit logs", Err: err}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(result); err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to encode response", Err: err}
	}
	return nil
}

func (h *Handler) handleLogByID(w http.ResponseWriter, r *http.Request) *web.Error {
	idRaw := strings.TrimPrefix(r.URL.Path, "/api/audit/logs/")
	idRaw = strings.TrimSpace(idRaw)
	if idRaw == "" {
		return &web.Error{Code: http.StatusBadRequest, Message: "Audit log id is required"}
	}

	id, err := strconv.ParseInt(idRaw, 10, 64)
	if err != nil || id <= 0 {
		return &web.Error{Code: http.StatusBadRequest, Message: "Invalid audit log id", Err: err}
	}

	logItem, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return &web.Error{Code: http.StatusNotFound, Message: "Audit log not found", Err: err}
		}
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to get audit log", Err: err}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(logItem); err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to encode response", Err: err}
	}
	return nil
}

func parseListFilter(r *http.Request) (ListFilter, error) {
	query := r.URL.Query()

	filter := ListFilter{
		Page:     1,
		PageSize: 20,
		User:     strings.TrimSpace(query.Get("user")),
		Action:   strings.TrimSpace(query.Get("action")),
	}

	if pageRaw := strings.TrimSpace(query.Get("page")); pageRaw != "" {
		page, err := strconv.Atoi(pageRaw)
		if err != nil || page <= 0 {
			return ListFilter{}, fmt.Errorf("page must be a positive integer")
		}
		filter.Page = page
	}

	if pageSizeRaw := strings.TrimSpace(query.Get("pageSize")); pageSizeRaw != "" {
		pageSize, err := strconv.Atoi(pageSizeRaw)
		if err != nil || pageSize <= 0 {
			return ListFilter{}, fmt.Errorf("pageSize must be a positive integer")
		}
		filter.PageSize = pageSize
	}

	if resultRaw := strings.TrimSpace(query.Get("result")); resultRaw != "" {
		result := Result(resultRaw)
		if !IsValidResult(result) {
			return ListFilter{}, fmt.Errorf("invalid result filter")
		}
		filter.Result = result
	}

	if spaceIDRaw := strings.TrimSpace(query.Get("spaceId")); spaceIDRaw != "" {
		spaceID, err := strconv.ParseInt(spaceIDRaw, 10, 64)
		if err != nil || spaceID <= 0 {
			return ListFilter{}, fmt.Errorf("invalid spaceId")
		}
		filter.SpaceID = &spaceID
	}

	if fromRaw := strings.TrimSpace(query.Get("from")); fromRaw != "" {
		parsed, err := time.Parse(time.RFC3339, fromRaw)
		if err != nil {
			return ListFilter{}, fmt.Errorf("invalid from timestamp")
		}
		from := parsed.UTC()
		filter.From = &from
	}

	if toRaw := strings.TrimSpace(query.Get("to")); toRaw != "" {
		parsed, err := time.Parse(time.RFC3339, toRaw)
		if err != nil {
			return ListFilter{}, fmt.Errorf("invalid to timestamp")
		}
		to := parsed.UTC()
		filter.To = &to
	}

	if filter.From != nil && filter.To != nil && filter.From.After(*filter.To) {
		return ListFilter{}, fmt.Errorf("from must be before to")
	}

	return filter, nil
}
