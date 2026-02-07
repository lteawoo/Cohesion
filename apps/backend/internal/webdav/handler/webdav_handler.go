package webdav

import (
	"net/http"

	"github.com/rs/zerolog/log"
	"taeu.kr/cohesion/internal/platform/web"
	"taeu.kr/cohesion/internal/webdav"
)

type Handler struct {
	webDavService *webdav.Service
}

func NewHandler(webDavService *webdav.Service) *Handler {
	return &Handler{
		webDavService: webDavService,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) *web.Error {
	// URL 경로에서 space 이름 추출
	spaceName, _ := webdav.ResolvePath(r.URL.Path)

	// elqjrm
	log.Debug().Msgf("WebDAV request for space: %s, path: %s", spaceName, r.URL.Path)

	// spaceName이 없으면 루트 핸들러로 위임 (Space 목록 표시)
	if spaceName == "" {
		h.webDavService.GetRootHandler().ServeHTTP(w, r)
		return nil
	}

	// 해당 space에 대한 WebDAV 핸들러 가져오기
	webDavHandler, err := h.webDavService.GetWebDAVHandler(r.Context(), spaceName)
	if err != nil {
		return &web.Error{
			Code:    http.StatusInternalServerError,
			Message: "Failed to get WebDAV handler",
			Err:     err,
		}
	}

	// WebDAV 핸들러로 요청 처리 위임
	webDavHandler.ServeHTTP(w, r)
	return nil
}
