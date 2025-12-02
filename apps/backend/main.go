package main

import (
	"log"
	"net/http"

	"taeu.kr/cohesion/internal/config"
	"taeu.kr/cohesion/internal/platform/database"
	"taeu.kr/cohesion/internal/spa"
)

var goEnv string = "development"

func init() {

}

func main() {
	// 로거 설정
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	log.Println("[Main] Starting Server...")
	log.Println("[Main] environment:", goEnv)

	// 설정 로드
	config.SetConfig(goEnv)

	// 라우터 생성
	mux := http.NewServeMux()

	// API 핸들러 설정
	mux.HandleFunc("/api/", handleAPI)

	if goEnv == "production" {
		spaHandler, err := spa.NewSPAHandler(WebDist, "dist/web")
		if err != nil {
			log.Fatalf("Failed to create SPA Handler: %v", err)
		}

		// SPA 핸들러 설정
		mux.HandleFunc("/", spaHandler)
	}

	// 데이터베이스 연결 설정
	db, err := database.NewDB()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("Database connected successfully.")

	port := ":" + config.Conf.Server.Port
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

// Logger 미들웨어
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL)
		next.ServeHTTP(w, r)
	})
}
