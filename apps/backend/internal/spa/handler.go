package spa

import (
	"embed"
	"io"
	"io/fs"
	"log"
	"net/http"
)

type spaResponseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

// WriteHeader SPA 응답을 위한 WriteHeader 메서드
func (w *spaResponseWriter) WriteHeader(status int) {
	w.status = status
	w.wroteHeader = true

	// 404가 아닌 경우 바로 전달
	if status != http.StatusNotFound {
		w.ResponseWriter.WriteHeader(status)
	}
}

// Write SPA 응답을 위한 Write 메서드
func (w *spaResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}

	// 404인 경우 버림
	if w.status == http.StatusNotFound {
		return len(b), nil
	}

	return w.ResponseWriter.Write(b)
}

// NewSPAHandler SPA 핸들러 생성 함수
func NewSPAHandler(assets embed.FS, embedDir string) (http.HandlerFunc, error) {
	distFS, err := fs.Sub(assets, embedDir)
	if err != nil {
		return nil, err
	}

	// 정적 파일 핸들러 설정
	fileServer := http.FileServer(http.FS(distFS))

	return func(w http.ResponseWriter, r *http.Request) {
		// Wrapper 생성
		wrapper := &spaResponseWriter{
			ResponseWriter: w,
			status:         http.StatusOK,
		}

		// FileServer 위임
		fileServer.ServeHTTP(wrapper, r)

		// 404인 경우 index.html 서빙
		if wrapper.status == http.StatusNotFound {
			file, err := distFS.Open("index.html")
			if err != nil {
				log.Printf("Failed to open index.html: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			defer file.Close()

			wrapper.Header().Set("Content-Type", "text/html; charset=utf-8")
			wrapper.WriteHeader(http.StatusOK)

			if _, err := io.Copy(wrapper, file); err != nil {
				log.Printf("Error Serving index.html: %v", err)
			}
		}
	}, nil
}
