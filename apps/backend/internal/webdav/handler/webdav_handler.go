package webdav

import (
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
	"taeu.kr/cohesion/internal/account"
	"taeu.kr/cohesion/internal/platform/web"
	"taeu.kr/cohesion/internal/webdav"
)

type Handler struct {
	webDavService  *webdav.Service
	accountService *account.Service
}

func NewHandler(webDavService *webdav.Service, accountService *account.Service) *Handler {
	return &Handler{
		webDavService:  webDavService,
		accountService: accountService,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) *web.Error {
	// 루트 OPTIONS(/dav, /dav/)는 무인증으로 허용해 DAV 핸드셰이크를 통과시킨다.
	// 하위 경로 OPTIONS는 기존 인증 경로를 유지한다.
	if r.Method == http.MethodOptions && isWebDAVRootPath(r.URL.Path) {
		if h.webDavService != nil {
			h.webDavService.GetRootHandler().ServeHTTP(w, r)
			return nil
		}
		writeWebDAVRootOptionsFallback(w)
		return nil
	}

	username, password, ok := r.BasicAuth()
	if !ok {
		writeWebDAVUnauthorized(w)
		return nil
	}
	authed, err := h.accountService.Authenticate(r.Context(), username, password)
	if err != nil {
		return &web.Error{
			Code:    http.StatusInternalServerError,
			Message: "Failed to authenticate WebDAV user",
			Err:     err,
		}
	}
	if !authed {
		writeWebDAVUnauthorized(w)
		return nil
	}

	normalizePROPFINDDepth(r)

	ctx := webdav.WithUsername(r.Context(), username)

	// URL 경로에서 space 이름 추출
	spaceName, _ := webdav.ResolvePath(r.URL.Path)

	log.Debug().Msgf("WebDAV request for space: %s, path: %s", spaceName, r.URL.Path)

	// spaceName이 없으면 루트 핸들러로 위임 (Space 목록 표시)
	if spaceName == "" {
		h.webDavService.GetRootHandler().ServeHTTP(w, r.WithContext(ctx))
		return nil
	}

	spaceObj, err := h.webDavService.GetSpaceByName(ctx, spaceName)
	if err != nil {
		return &web.Error{
			Code:    http.StatusNotFound,
			Message: "Space not found",
			Err:     err,
		}
	}

	required := requiredPermissionForMethod(r.Method)
	allowed, err := h.accountService.CanAccessSpaceByID(ctx, username, spaceObj.ID, required)
	if err != nil {
		return &web.Error{
			Code:    http.StatusInternalServerError,
			Message: "Failed to evaluate space access",
			Err:     err,
		}
	}
	if !allowed {
		return &web.Error{
			Code:    http.StatusForbidden,
			Message: "Forbidden",
		}
	}

	// 해당 space에 대한 WebDAV 핸들러 가져오기
	webDavHandler, err := h.webDavService.GetWebDAVHandler(ctx, spaceName)
	if err != nil {
		return &web.Error{
			Code:    http.StatusInternalServerError,
			Message: "Failed to get WebDAV handler",
			Err:     err,
		}
	}

	// WebDAV 핸들러로 요청 처리 위임
	webDavHandler.ServeHTTP(w, r.WithContext(ctx))
	return nil
}

func requiredPermissionForMethod(method string) account.Permission {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, "PROPFIND":
		return account.PermissionRead
	default:
		return account.PermissionWrite
	}
}

func writeWebDAVUnauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="Cohesion WebDAV"`)
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte("Unauthorized"))
}

func isWebDAVRootPath(path string) bool {
	trimmed := strings.TrimRight(strings.TrimSpace(path), "/")
	return trimmed == "/dav"
}

func writeWebDAVRootOptionsFallback(w http.ResponseWriter) {
	w.Header().Set("DAV", "1, 2")
	w.Header().Set("MS-Author-Via", "DAV")
	w.WriteHeader(http.StatusOK)
}

func normalizePROPFINDDepth(r *http.Request) {
	if r.Method != "PROPFIND" {
		return
	}

	depth := strings.ToLower(strings.TrimSpace(r.Header.Get("Depth")))
	switch depth {
	case "0", "1":
		return
	default:
		// 무한/생략/비정상 Depth는 1로 고정해 재귀 전체 스캔을 방지한다.
		r.Header.Set("Depth", "1")
	}
}
