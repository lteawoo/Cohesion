package main

import (
	"embed"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
)

//go:embed web/dist
var webDist embed.FS

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

func main() {
	// 로거 설정
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// web/dist 추출
	distFS, err := fs.Sub(webDist, "web/dist")
	if err != nil {
		log.Fatal("Failed to load web/dist:", err)
	}

	// 라우터 생성
	mux := http.NewServeMux()

	// API 핸들러 설정
	mux.HandleFunc("/api/", handleAPI)

	// production에서는 SPA 핸들링
	if os.Getenv("GO_ENV") == "production" {
		// SPA 핸들러 설정
		mux.HandleFunc("/", handleSPA(distFS))
	}

	port := ":3000"
	log.Printf("Server is running on port %s", port)
	log.Println("\nPress Ctrl+C to stop")

	if err := http.ListenAndServe(port, Logger(mux)); err != nil {
		log.Fatal("Server failed:", err)
	}
}

// API 핸들러
func handleAPI(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/api/health":
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{\"status\": \"ok\"}"))
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("{\"error\": \"Not Found\"}"))
	}
}

// SPA 핸들러
func handleSPA(distFS fs.FS) http.HandlerFunc {
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

			// index.html은 캐시하지 않음
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)

			if _, err := io.Copy(w, file); err != nil {
				log.Printf("Error Serving index.html: %v", err)
			}
		}
	}
}

// Logger 미들웨어
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL)
		next.ServeHTTP(w, r)
	})
}
