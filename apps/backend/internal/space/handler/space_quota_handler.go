package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"taeu.kr/cohesion/internal/account"
	"taeu.kr/cohesion/internal/auth"
	"taeu.kr/cohesion/internal/platform/web"
)

type spaceUsageResponse struct {
	SpaceID    int64  `json:"spaceId"`
	SpaceName  string `json:"spaceName"`
	UsedBytes  int64  `json:"usedBytes"`
	QuotaBytes *int64 `json:"quotaBytes,omitempty"`
	OverQuota  bool   `json:"overQuota"`
	ScannedAt  string `json:"scannedAt"`
}

func (h *Handler) handleSpaceUsage(w http.ResponseWriter, r *http.Request) *web.Error {
	if r.Method != http.MethodGet {
		return &web.Error{Code: http.StatusMethodNotAllowed, Message: "Method not allowed"}
	}

	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		return &web.Error{Code: http.StatusUnauthorized, Message: "Unauthorized"}
	}

	spaces, err := h.spaceService.GetAllSpaces(r.Context())
	if err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to get spaces", Err: err}
	}

	items := make([]spaceUsageResponse, 0, len(spaces))
	for _, item := range spaces {
		allowed, accessErr := h.accountService.CanAccessSpaceByID(r.Context(), claims.Username, item.ID, account.PermissionRead)
		if accessErr != nil {
			return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to evaluate space access", Err: accessErr}
		}
		if !allowed {
			continue
		}

		usage, usageErr := h.quotaService.GetSpaceUsage(r.Context(), item.ID)
		if usageErr != nil {
			return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to get space usage", Err: usageErr}
		}

		items = append(items, spaceUsageResponse{
			SpaceID:    usage.SpaceID,
			SpaceName:  usage.SpaceName,
			UsedBytes:  usage.UsedBytes,
			QuotaBytes: usage.QuotaBytes,
			OverQuota:  usage.OverQuota,
			ScannedAt:  usage.ScannedAt.UTC().Format(http.TimeFormat),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].SpaceName != items[j].SpaceName {
			return strings.ToLower(items[i].SpaceName) < strings.ToLower(items[j].SpaceName)
		}
		return items[i].SpaceID < items[j].SpaceID
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(items); err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to encode response", Err: err}
	}
	return nil
}

func (h *Handler) handleSpaceQuota(w http.ResponseWriter, r *http.Request, spaceID int64) *web.Error {
	if r.Method != http.MethodPatch {
		return &web.Error{Code: http.StatusMethodNotAllowed, Message: "Method not allowed"}
	}

	var req struct {
		QuotaBytes *int64 `json:"quotaBytes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return &web.Error{Code: http.StatusBadRequest, Message: "Invalid request body", Err: err}
	}
	if req.QuotaBytes != nil && *req.QuotaBytes < 0 {
		return &web.Error{Code: http.StatusBadRequest, Message: "quotaBytes must be greater than or equal to 0"}
	}

	updatedSpace, err := h.spaceService.UpdateSpaceQuota(r.Context(), spaceID, req.QuotaBytes)
	if err != nil {
		statusCode := http.StatusInternalServerError
		message := "Failed to update space quota"
		switch {
		case strings.Contains(err.Error(), "not found"):
			statusCode = http.StatusNotFound
			message = "Space not found"
		case strings.Contains(err.Error(), "invalid"):
			statusCode = http.StatusBadRequest
			message = "Invalid quota request"
		}
		return &web.Error{Code: statusCode, Message: message, Err: err}
	}

	h.quotaService.Invalidate(spaceID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"id":         updatedSpace.ID,
		"quotaBytes": updatedSpace.QuotaBytes,
		"message":    fmt.Sprintf("Space quota updated for '%s'", updatedSpace.SpaceName),
	}); err != nil {
		return &web.Error{Code: http.StatusInternalServerError, Message: "Failed to encode response", Err: err}
	}
	return nil
}
