package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
	"github.com/rs/zerolog/log"
	"taeu.kr/cohesion/internal/account"
	accountStore "taeu.kr/cohesion/internal/account/store"
	"taeu.kr/cohesion/internal/auth"
	"taeu.kr/cohesion/internal/browse"
	browseHandler "taeu.kr/cohesion/internal/browse/handler"
	"taeu.kr/cohesion/internal/config"
	"taeu.kr/cohesion/internal/platform/database"
	"taeu.kr/cohesion/internal/platform/web"
	sftpserver "taeu.kr/cohesion/internal/sftp"
	"taeu.kr/cohesion/internal/spa"
	"taeu.kr/cohesion/internal/space"
	spaceHandler "taeu.kr/cohesion/internal/space/handler"
	spaceStore "taeu.kr/cohesion/internal/space/store"
	"taeu.kr/cohesion/internal/status"
	"taeu.kr/cohesion/internal/system"
	"taeu.kr/cohesion/internal/webdav"
	webdavHandler "taeu.kr/cohesion/internal/webdav/handler"
)

var (
	goEnv        = "development"
	appVersion   = "dev"
	appCommit    = "local"
	appBuildDate = ""
)

// 재시작 신호를 받기 위한 채널
var restartChan = make(chan bool, 1)
var shutdownChan = make(chan struct{}, 1)

func init() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
}

func resolveExecutableDir() (string, error) {
	executablePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	executablePath = filepath.Clean(executablePath)
	if resolvedPath, err := filepath.EvalSymlinks(executablePath); err == nil && strings.TrimSpace(resolvedPath) != "" {
		executablePath = resolvedPath
	}
	return filepath.Dir(executablePath), nil
}

func openRootLogFile(fileName string) (*os.File, string, error) {
	executableDir, err := resolveExecutableDir()
	if err != nil {
		return nil, "", err
	}
	logsDir := filepath.Join(executableDir, "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return nil, "", err
	}
	logPath := filepath.Join(logsDir, fileName)
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, "", err
	}
	return logFile, logPath, nil
}

func configureRootLogger() (*os.File, string, error) {
	logFile, logPath, err := openRootLogFile("app.log")
	if err != nil {
		return nil, "", err
	}

	if goEnv == "production" {
		zerolog.TimestampFunc = func() time.Time {
			return time.Now().UTC()
		}
		log.Logger = log.Output(logFile)
		return logFile, logPath, nil
	}

	log.Logger = log.Output(zerolog.MultiLevelWriter(
		zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339},
		logFile,
	))
	return logFile, logPath, nil
}

func writePIDFile() (func(), string, error) {
	executableDir, err := resolveExecutableDir()
	if err != nil {
		return func() {}, "", err
	}
	pidPath := filepath.Join(executableDir, "cohesion.pid")
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		return func() {}, "", err
	}
	cleanup := func() {
		_ = os.Remove(pidPath)
	}
	return cleanup, pidPath, nil
}

// createServer는 설정을 기반으로 HTTP 서버를 생성합니다
func createServer(db *sql.DB, restartChan chan bool, shutdownChan chan struct{}) (*http.Server, *sftpserver.Service, error) {
	// 의존성 주입
	accountRepo := accountStore.NewStore(db)
	accountService := account.NewService(accountRepo)
	if err := accountService.EnsureDefaultAdmin(context.Background()); err != nil {
		return nil, nil, err
	}
	authSecret, err := resolveJWTSecret()
	if err != nil {
		return nil, nil, err
	}
	accountHandler := account.NewHandler(accountService)
	authService := auth.NewService(accountService, auth.Config{
		Secret:         authSecret,
		Issuer:         "cohesion",
		AccessTokenTTL: 15 * time.Minute,
		RefreshTTL:     7 * 24 * time.Hour,
	})
	authHandler := auth.NewHandler(authService)

	spaceRepo := spaceStore.NewStore(db)
	trashRepo := spaceStore.NewTrashStore(db)
	spaceService := space.NewService(spaceRepo)
	trashService := space.NewTrashService(trashRepo)
	browseService := browse.NewService()
	spaceHandler := spaceHandler.NewHandler(spaceService, browseService, accountService, trashService)
	browseHandler := browseHandler.NewHandler(browseService, spaceService)
	webDavService := webdav.NewService(spaceService, accountService)
	webDavHandler := webdavHandler.NewHandler(webDavService, accountService)
	sftpService := sftpserver.NewService(spaceService, accountService, config.Conf.Server.SftpEnabled, config.Conf.Server.SftpPort)
	statusHandler := status.NewHandler(db, spaceService, config.Conf.Server.Port)
	configHandler := config.NewHandler()
	systemHandler := system.NewHandler(restartChan, shutdownChan, system.Meta{
		Version:   appVersion,
		Commit:    appCommit,
		BuildDate: appBuildDate,
	})

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
	accountHandler.RegisterRoutes(mux)
	authHandler.RegisterRoutes(mux)

	// WebDAV 핸들러 등록
	if config.Conf.Server.WebdavEnabled {
		mux.Handle("/dav/", web.Handler(func(w http.ResponseWriter, r *http.Request) *web.Error {
			return webDavHandler.ServeHTTP(w, r)
		}))
	}

	if goEnv == "production" {
		spaHandler, err := spa.NewSPAHandler(WebDist, "dist/web")
		if err != nil {
			return nil, nil, err
		}
		mux.HandleFunc("/", spaHandler)
	}

	// hlog.NewHanlder는 Zerolog 컨텍스트를 HTTP 핸들러에 주입
	hlogHandler := hlog.NewHandler(log.Logger)

	// hlog.AccessHandler는 요청 및 응답에 대한 로그를 기록
	finalHandler := authService.Middleware(mux)

	finalLogHandler := hlogHandler(hlog.AccessHandler(func(r *http.Request, status, size int, duration time.Duration) {
		hlog.FromRequest(r).Info().
			Str("method", r.Method).
			Str("url", r.URL.String()).
			Int("status", status).
			Int("size", size).
			Dur("duration", duration).
			Msg("Access log")
	})(finalHandler))

	port := ":" + config.Conf.Server.Port
	server := &http.Server{
		Addr:    port,
		Handler: finalLogHandler,
	}

	return server, sftpService, nil
}

func main() {
	logFile, logPath, loggerErr := configureRootLogger()
	if loggerErr != nil {
		fmt.Fprintf(os.Stderr, "[Main] failed to initialize file logger: %v\n", loggerErr)
	} else {
		defer logFile.Close()
		log.Info().Str("path", logPath).Msg("[Main] App log file initialized")
	}

	pidCleanup, pidPath, pidErr := writePIDFile()
	if pidErr != nil {
		log.Warn().Err(pidErr).Msg("[Main] Failed to write PID file")
	} else {
		defer pidCleanup()
		log.Info().Str("path", pidPath).Int("pid", os.Getpid()).Msg("[Main] PID file initialized")
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
		server, sftpService, err := createServer(db, restartChan, shutdownChan)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create server")
		}
		if err := sftpService.Start(); err != nil {
			log.Fatal().Err(err).Msg("Failed to start SFTP server")
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
			if err := sftpService.Stop(); err != nil {
				log.Error().Err(err).Msg("SFTP server shutdown error")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := server.Shutdown(ctx); err != nil {
				log.Error().Err(err).Msg("Server shutdown error")
			}
			log.Info().Msg("[Main] Server stopped")
			return

		case <-restartChan:
			log.Info().Msg("[Main] Restart signal received")
			if err := sftpService.Stop(); err != nil {
				log.Error().Err(err).Msg("SFTP server shutdown error")
			}
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

		case <-shutdownChan:
			log.Info().Msg("[Main] Shutdown signal received from updater")
			if err := sftpService.Stop(); err != nil {
				log.Error().Err(err).Msg("SFTP server shutdown error")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := server.Shutdown(ctx); err != nil {
				log.Error().Err(err).Msg("Server shutdown error")
			}
			log.Info().Msg("[Main] Server stopped for self-update")
			return

		case err := <-serverErr:
			if stopErr := sftpService.Stop(); stopErr != nil {
				log.Error().Err(stopErr).Msg("SFTP server shutdown error")
			}
			log.Fatal().Err(err).Msg("Server error")
			return
		}
	}
}

func readEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func resolveJWTSecret() (string, error) {
	secret := strings.TrimSpace(os.Getenv("COHESION_JWT_SECRET"))
	if secret != "" {
		if goEnv == "production" && len(secret) < 32 {
			return "", errors.New("COHESION_JWT_SECRET must be at least 32 characters in production")
		}
		return secret, nil
	}

	secretFilePath, err := resolveJWTSecretPath()
	if err != nil {
		return "", err
	}

	secret, err = loadOrCreateJWTSecret(secretFilePath)
	if err != nil {
		return "", err
	}

	if goEnv == "production" && len(secret) < 32 {
		return "", errors.New("COHESION_JWT_SECRET must be at least 32 characters in production")
	}

	log.Info().Str("path", secretFilePath).Msg("JWT secret loaded from file")
	return secret, nil
}

func resolveJWTSecretPath() (string, error) {
	if customPath := strings.TrimSpace(os.Getenv("COHESION_JWT_SECRET_FILE")); customPath != "" {
		return customPath, nil
	}

	userConfigDir, err := os.UserConfigDir()
	if err == nil && strings.TrimSpace(userConfigDir) != "" {
		return filepath.Join(userConfigDir, "Cohesion", "secrets", "jwt_secret"), nil
	}

	executablePath, err := os.Executable()
	if err != nil {
		return "", errors.New("failed to resolve jwt secret path")
	}
	return filepath.Join(filepath.Dir(executablePath), "data", "jwt_secret"), nil
}

func loadOrCreateJWTSecret(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err == nil {
		secret := strings.TrimSpace(string(content))
		if secret != "" {
			return secret, nil
		}
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}

	secret, err := generateRandomSecret(48)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(secret+"\n"), 0600); err != nil {
		return "", err
	}

	return secret, nil
}

func generateRandomSecret(size int) (string, error) {
	if size < 32 {
		return "", errors.New("secret size must be at least 32 bytes")
	}

	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
