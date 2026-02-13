package main

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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
	"taeu.kr/cohesion/internal/system"
	"taeu.kr/cohesion/internal/webdav"
	webdavHandler "taeu.kr/cohesion/internal/webdav/handler"
)

var goEnv string = "development"

// 재시작 신호를 받기 위한 채널
var restartChan = make(chan bool, 1)

func init() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
}

// createServer는 설정을 기반으로 HTTP 서버를 생성합니다
func createServer(db *sql.DB, restartChan chan bool) (*http.Server, error) {
	// 의존성 주입
	spaceStore := spaceStore.NewStore(db)
	spaceService := space.NewService(spaceStore)
	browseService := browse.NewService()
	spaceHandler := spaceHandler.NewHandler(spaceService, browseService)
	browseHandler := browseHandler.NewHandler(browseService, spaceService)
	webDavService := webdav.NewService(spaceService)
	webDavHandler := webdavHandler.NewHandler(webDavService)
	statusHandler := status.NewHandler(db, spaceService, config.Conf.Server.Port)
	configHandler := config.NewHandler()
	systemHandler := system.NewHandler(restartChan)

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
	configHandler.RegisterRoutes(mux)
	systemHandler.RegisterRoutes(mux)

	// WebDAV 핸들러 등록
	mux.Handle("/dav/", web.Handler(func(w http.ResponseWriter, r *http.Request) *web.Error {
		return webDavHandler.ServeHTTP(w, r)
	}))

	if goEnv == "production" {
		spaHandler, err := spa.NewSPAHandler(WebDist, "dist/web")
		if err != nil {
			return nil, err
		}
		mux.HandleFunc("/", spaHandler)
	}

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

	port := ":" + config.Conf.Server.Port
	server := &http.Server{
		Addr:    port,
		Handler: finalLogHandler,
	}

	return server, nil
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

	// OS 시그널 핸들링 (Ctrl+C)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 서버 시작/재시작 루프
	for {
		// 서버 생성
		server, err := createServer(db, restartChan)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create server")
		}

		port := config.Conf.Server.Port
		log.Info().Msgf("Server is running on port %s", port)
		log.Info().Msg("Press Ctrl+C to stop")

		// 서버를 별도 고루틴에서 실행
		serverErr := make(chan error, 1)
		go func() {
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				serverErr <- err
			}
		}()

		// 재시작 또는 종료 신호 대기
		select {
		case <-sigChan:
			log.Info().Msg("[Main] Shutdown signal received")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := server.Shutdown(ctx); err != nil {
				log.Error().Err(err).Msg("Server shutdown error")
			}
			log.Info().Msg("[Main] Server stopped")
			return

		case <-restartChan:
			log.Info().Msg("[Main] Restart signal received")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := server.Shutdown(ctx); err != nil {
				log.Error().Err(err).Msg("Server shutdown error")
			}
			cancel()

			// 설정 다시 로드
			log.Info().Msg("[Main] Reloading configuration...")
			config.SetConfig(goEnv)
			log.Info().Msgf("[Main] Restarting with new port: %s", config.Conf.Server.Port)
			// 루프 계속 (재시작)

		case err := <-serverErr:
			log.Fatal().Err(err).Msg("Server error")
			return
		}
	}
}
