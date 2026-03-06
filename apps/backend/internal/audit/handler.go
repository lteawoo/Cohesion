package audit

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"taeu.kr/cohesion/internal/platform/web"
)

type Handler struct {
	service               *Service
	retentionDaysProvider func() int
	actorResolver         func(*http.Request) string
	now                   func() time.Time
}

func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (h *Handler) SetRetentionDaysProvider(provider func() int) {
	h.retentionDaysProvider = provider
}

func (h *Handler) SetActorResolver(resolver func(*http.Request) string) {
	h.actorResolver = resolver
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/audit/logs", web.Handler(h.handleListLogs))
	mux.Handle("GET /api/audit/logs/export", web.Handler(h.handleExportLogs))
	mux.Handle("POST /api/audit/logs/cleanup", web.Handler(h.handleCleanupLogs))
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
	result.RetentionDays = h.currentRetentionDays()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(result); err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to encode response", Err: err}
	}
	return nil
}

func (h *Handler) handleExportLogs(w http.ResponseWriter, r *http.Request) *web.Error {
	filter, err := parseListFilter(r)
	if err != nil {
		return &web.Error{Code: http.StatusBadRequest, Message: "Invalid audit log query", Err: err}
	}

	filename := fmt.Sprintf("audit-logs-%s.csv", h.now().UTC().Format("20060102-150405"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.WriteHeader(http.StatusOK)

	writer := csv.NewWriter(w)
	if err := writer.Write([]string{"id", "occurred_at", "actor", "action", "result", "target", "request_id", "space_id", "metadata_json"}); err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to write export header", Err: err}
	}

	if err := h.service.Export(r.Context(), filter, func(item *Log) error {
		metadataJSON, err := json.Marshal(item.Metadata)
		if err != nil {
			return fmt.Errorf("failed to encode audit metadata: %w", err)
		}

		spaceID := ""
		if item.SpaceID != nil {
			spaceID = strconv.FormatInt(*item.SpaceID, 10)
		}

		if err := writer.Write([]string{
			strconv.FormatInt(item.ID, 10),
			item.OccurredAt.UTC().Format(time.RFC3339),
			item.Actor,
			item.Action,
			string(item.Result),
			item.Target,
			item.RequestID,
			spaceID,
			string(metadataJSON),
		}); err != nil {
			return fmt.Errorf("failed to write export row: %w", err)
		}
		return nil
	}); err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to export audit logs", Err: err}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to flush export", Err: err}
	}
	return nil
}

func (h *Handler) handleCleanupLogs(w http.ResponseWriter, r *http.Request) *web.Error {
	retentionDays := h.currentRetentionDays()
	if retentionDays <= 0 {
		return &web.Error{Code: http.StatusBadRequest, Message: "Audit log retention policy is disabled"}
	}

	cutoff := h.now().UTC().AddDate(0, 0, -retentionDays)
	deletedCount, err := h.service.CleanupOlderThan(r.Context(), cutoff)
	if err != nil {
		h.recordCleanupAudit(r, ResultFailure, retentionDays, cutoff, 0)
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to cleanup audit logs", Err: err}
	}

	h.recordCleanupAudit(r, ResultSuccess, retentionDays, cutoff, deletedCount)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(CleanupResult{
		DeletedCount:  deletedCount,
		RetentionDays: retentionDays,
		Cutoff:        cutoff,
	}); err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to encode cleanup response", Err: err}
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

func (h *Handler) currentRetentionDays() int {
	if h.retentionDaysProvider == nil {
		return 0
	}
	return h.retentionDaysProvider()
}

func (h *Handler) recordCleanupAudit(r *http.Request, result Result, retentionDays int, cutoff time.Time, deletedCount int64) {
	if h.service == nil {
		return
	}

	actor := ""
	if h.actorResolver != nil {
		actor = strings.TrimSpace(h.actorResolver(r))
	}

	requestID := strings.TrimSpace(r.Header.Get("X-Request-Id"))
	h.service.RecordBestEffort(Event{
		Actor:     actor,
		Action:    "audit.logs.cleanup",
		Result:    result,
		Target:    "audit_logs",
		RequestID: requestID,
		Metadata: map[string]any{
			"retentionDays": retentionDays,
			"deletedCount":  deletedCount,
			"cutoff":        cutoff.UTC().Format(time.RFC3339),
		},
	})
}
