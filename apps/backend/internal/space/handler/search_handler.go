package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"taeu.kr/cohesion/internal/account"
	"taeu.kr/cohesion/internal/auth"
	"taeu.kr/cohesion/internal/platform/web"
	"taeu.kr/cohesion/internal/space"
)

const (
	defaultSearchResultLimit = 80
	maxSearchResultLimit     = 200
)

var errSearchLimitReached = errors.New("search limit reached")

type fileSearchResult struct {
	SpaceID    int64     `json:"spaceId"`
	SpaceName  string    `json:"spaceName"`
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	ParentPath string    `json:"parentPath"`
	IsDir      bool      `json:"isDir"`
	Size       int64     `json:"size"`
	ModTime    time.Time `json:"modTime"`
}

func (h *Handler) handleSearchFiles(w http.ResponseWriter, r *http.Request) *web.Error {
	if r.Method != http.MethodGet {
		return &web.Error{
			Code:    http.StatusMethodNotAllowed,
			Message: "Method not allowed",
		}
	}

	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		return &web.Error{
			Code:    http.StatusUnauthorized,
			Message: "Unauthorized",
		}
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		return writeSearchResponse(w, []fileSearchResult{})
	}

	limit, parseErr := parseSearchLimit(r.URL.Query().Get("limit"))
	if parseErr != nil {
		return &web.Error{
			Code:    http.StatusBadRequest,
			Message: parseErr.Error(),
			Err:     parseErr,
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

	queryLower := strings.ToLower(query)
	results := make([]fileSearchResult, 0, min(limit, len(spaces)))

	for _, item := range spaces {
		allowed, accessErr := h.accountService.CanAccessSpaceByID(r.Context(), claims.Username, item.ID, account.PermissionRead)
		if accessErr != nil {
			return &web.Error{
				Code:    http.StatusInternalServerError,
				Message: "Failed to evaluate space access",
				Err:     accessErr,
			}
		}
		if !allowed {
			continue
		}

		spaceResults, searchErr := searchFilesInSpace(item, queryLower, limit-len(results))
		if searchErr != nil {
			return &web.Error{
				Code:    http.StatusInternalServerError,
				Message: "Failed to search files",
				Err:     searchErr,
			}
		}
		results = append(results, spaceResults...)
		if len(results) >= limit {
			break
		}
	}

	sortSearchResults(results, queryLower)
	return writeSearchResponse(w, results)
}

func parseSearchLimit(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return defaultSearchResultLimit, nil
	}

	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return 0, errors.New("limit must be a positive integer")
	}
	if parsed > maxSearchResultLimit {
		return maxSearchResultLimit, nil
	}
	return parsed, nil
}

func writeSearchResponse(w http.ResponseWriter, results []fileSearchResult) *web.Error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(results); err != nil {
		return &web.Error{
			Code:    http.StatusInternalServerError,
			Message: "Failed to encode response",
			Err:     err,
		}
	}
	return nil
}

func searchFilesInSpace(spaceData *space.Space, queryLower string, limit int) ([]fileSearchResult, error) {
	if limit <= 0 {
		return nil, nil
	}

	results := make([]fileSearchResult, 0, min(limit, 32))
	err := filepath.WalkDir(spaceData.SpacePath, func(currentPath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsPermission(walkErr) {
				if entry != nil && entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			return walkErr
		}
		if entry == nil {
			return nil
		}

		if currentPath == spaceData.SpacePath {
			return nil
		}

		// 숨김 파일/폴더는 검색 대상에서 제외합니다.
		if strings.HasPrefix(entry.Name(), ".") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		relativePath, relErr := filepath.Rel(spaceData.SpacePath, currentPath)
		if relErr != nil {
			return relErr
		}

		relativePath = filepath.ToSlash(relativePath)
		if !isSearchHit(queryLower, entry.Name()) {
			return nil
		}

		info, infoErr := entry.Info()
		if infoErr != nil {
			return nil
		}

		parentPath := filepath.ToSlash(filepath.Dir(relativePath))
		if parentPath == "." {
			parentPath = ""
		}

		results = append(results, fileSearchResult{
			SpaceID:    spaceData.ID,
			SpaceName:  spaceData.SpaceName,
			Name:       entry.Name(),
			Path:       relativePath,
			ParentPath: parentPath,
			IsDir:      entry.IsDir(),
			Size:       info.Size(),
			ModTime:    info.ModTime(),
		})

		if len(results) >= limit {
			return errSearchLimitReached
		}

		return nil
	})
	if errors.Is(err, errSearchLimitReached) {
		return results, nil
	}
	if err != nil {
		return nil, err
	}
	return results, nil
}

func isSearchHit(queryLower, name string) bool {
	nameLower := strings.ToLower(name)
	return strings.Contains(nameLower, queryLower)
}

func sortSearchResults(results []fileSearchResult, queryLower string) {
	sort.Slice(results, func(i, j int) bool {
		left := results[i]
		right := results[j]

		leftRank := searchRank(left, queryLower)
		rightRank := searchRank(right, queryLower)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if left.SpaceName != right.SpaceName {
			return left.SpaceName < right.SpaceName
		}
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		return left.Name < right.Name
	})
}

func searchRank(item fileSearchResult, queryLower string) int {
	nameLower := strings.ToLower(item.Name)

	switch {
	case nameLower == queryLower:
		return 0
	case strings.HasPrefix(nameLower, queryLower):
		return 1
	case strings.Contains(nameLower, queryLower):
		return 2
	default:
		return 3
	}
}
