/*
 * 여러 개의 space가 있을 때, 각각의 Space를 위해 webdav.Handler를 생성하고 관리한다.
 * 파일 수정 시 충돌을 방지하는 wevdav LockSystem도 Space별로 관리한다.
 */
package webdav

import (
	"context"
	"net/http"
	"sync"

	"github.com/rs/zerolog/log"
	"golang.org/x/net/webdav"
	"taeu.kr/cohesion/internal/space"
)

type Service struct {
	spaceService *space.Service
	lockSystems  map[string]webdav.LockSystem
	mu           sync.Mutex
	rootHandler  http.Handler
}

func NewService(spaceService *space.Service) *Service {
	return &Service{
		spaceService: spaceService,
		lockSystems:  make(map[string]webdav.LockSystem),
		rootHandler: &webdav.Handler{
			Prefix:     "/dav",
			FileSystem: NewSpaceFS(spaceService),
			LockSystem: webdav.NewMemLS(),
			Logger: func(r *http.Request, err error) {
				if err != nil {
					log.Error().Err(err).Msgf("WebDAV root error: %s %s", r.Method, r.URL.Path)
				}
			},
		},
	}
}

func (s *Service) GetRootHandler() http.Handler {
	return s.rootHandler
}

func (s *Service) GetWebDAVHandler(ctx context.Context, spaceName string) (http.Handler, error) {

	// 해당 이름의 Space 조회
	spaceObj, err := s.spaceService.GetSpaceByName(ctx, spaceName)
	if err != nil {
		return nil, err
	}

	// LockSystem 가져오기
	ls := s.getLockSystem(spaceName)

	// WebDAV 핸들러 생성
	return &webdav.Handler{
		Prefix:     "/dav/" + spaceName,
		FileSystem: webdav.Dir(spaceObj.SpacePath),
		LockSystem: ls,
		Logger: func(r *http.Request, err error) {
			if err != nil {
				log.Error().Err(err).Msgf("WebDAV error: %s %s", r.Method, r.URL.Path)
			}
		},
	}, nil
}

/*
 * spaceName에 해당하는 LockSystem을 반환한다.
 * LockSystem이 존재하지 않으면 새로 생성하여 저장한 후 반환한다.
 */
func (s *Service) getLockSystem(spaceName string) webdav.LockSystem {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ls, exists := s.lockSystems[spaceName]; exists {
		return ls
	}

	ls := webdav.NewMemLS()
	s.lockSystems[spaceName] = ls
	return ls
}
