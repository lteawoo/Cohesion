package web

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"
)

// Error는 웹 계층의 커스텀 에러 타입을 정의
type Error struct {
	Code    int
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

// Hanlder는 에러를 반환하는 웹 계층의 커스텀 핸들러 타입을 정의
type Handler func(w http.ResponseWriter, r *http.Request) *Error

func (fn Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := fn(w, r); err != nil {
		log.Error().
			Err(err.Err).            // 레벨: Error
			Str("method", r.Method). // HTTP 메서드
			Str("path", r.URL.Path). // 요청 경로
			Int("status", err.Code). // 상태 코드
			Msg(err.Message)         // 메시지

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(err.Code)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Message})
	}
}
