package main

import (
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
	"github.com/rs/zerolog/log"
	"taeu.kr/cohesion/internal/browse"
	browseHandler "taeu.kr/cohesion/internal/browse/handler"
	"taeu.kr/cohesion/internal/config"
	"taeu.kr/cohesion/internal/platform/database"
	"taeu.kr/cohesion/internal/platform/web"
	"taeu.kr/cohesion/internal/spa"
	"taeu.kr/cohesion/internal/space"
	spaceHandler "taeu.kr/cohesion/internal/space/handler"
	spaceStore "taeu.kr/cohesion/internal/space/store"
	"taeu.kr/cohesion/internal/status"
	"taeu.kr/cohesion/internal/webdav"
	webdavHandler "taeu.kr/cohesion/internal/webdav/handler"
)

var goEnv string = "development"

func init() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
}

func main() {
	// Zerolog 전역 로거 설정
	if goEnv == "production" {
		zerolog.TimestampFunc = func() time.Time {
			return time.Now().UTC()
		}
	} else {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	}

	log.Info().Msg("[Main] Starting Server...")
	log.Info().Msgf("[Main] environment: %s", goEnv)

	// 설정 로드
	config.SetConfig(goEnv)

	// 데이터베이스 연결 설정
	db, err := database.NewDB()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()
	log.Info().Msg("Database connected successfully.")

	// 의존성 주입
	spaceStore := spaceStore.NewStore(db)
	spaceService := space.NewService(spaceStore)
	spaceHandler := spaceHandler.NewHandler(spaceService)
	browseService := browse.NewService()
	browseHandler := browseHandler.NewHandler(browseService, spaceService)
	webDavService := webdav.NewService(spaceService)
	webDavHandler := webdavHandler.NewHandler(webDavService)
	statusHandler := status.NewHandler(db, spaceService)

	// 라우터 생성
	mux := http.NewServeMux()

	// API 핸들러 설정
	mux.Handle("/api/health", web.Handler(func(w http.ResponseWriter, r *http.Request) *web.Error {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{\"status\": \"ok\"}"))
		return nil
	}))

	// Api 핸들러 등록
	spaceHandler.RegisterRoutes(mux)
	browseHandler.RegisterRoutes(mux)
	statusHandler.RegisterRoutes(mux)

	// WebDAV 핸들러 등록
	mux.Handle("/dav/", web.Handler(func(w http.ResponseWriter, r *http.Request) *web.Error {
		return webDavHandler.ServeHTTP(w, r)
	}))

	if goEnv == "production" {
		spaHandler, err := spa.NewSPAHandler(WebDist, "dist/web")
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create SPA Handler")
		}

		// SPA 핸들러 설정
		mux.HandleFunc("/", spaHandler)
	}

	port := ":" + config.Conf.Server.Port
	log.Info().Msgf("Server is running on port %s", port)
	log.Info().Msg("\nPress Ctrl+C to stop")

	// hlog.NewHanlder는 Zerolog 컨텍스트를 HTTP 핸들러에 주입
	hlogHandler := hlog.NewHandler(log.Logger)

	// hlog.AccessHandler는 요청 및 응답에 대한 로그를 기록
	finalLogHandler := hlogHandler(hlog.AccessHandler(func(r *http.Request, status, size int, duration time.Duration) {
		hlog.FromRequest(r).Info().
			Str("method", r.Method).
			Str("url", r.URL.String()).
			Int("status", status).
			Int("size", size).
			Dur("duration", duration).
			Msg("Access log")
	})(mux))

	if err := http.ListenAndServe(port, finalLogHandler); err != nil {
		log.Fatal().Err(err).Msg("Server failed")
	}
}
