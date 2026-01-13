package webdav

import (
	"sync"

	"golang.org/x/net/webdav"
	"taeu.kr/cohesion/internal/space"
)

/*
여러 개의 space가 있을 때, 각각의 Space를 위해 webdav.Handler를 생성하고 관리한다.
파일 수정 시 충돌을 방지하는 wevdav LockSystem도 Space별로 관리한다.
*/

type Service struct {
	spaceService *space.Service
	lockSystems  map[string]webdav.LockSystem
	mu           sync.Mutex
}
