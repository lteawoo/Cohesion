package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"io"
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
	"taeu.kr/cohesion/internal/audit"
	auditStore "taeu.kr/cohesion/internal/audit/store"
	"taeu.kr/cohesion/internal/auth"
	"taeu.kr/cohesion/internal/browse"
	browseHandler "taeu.kr/cohesion/internal/browse/handler"
	"taeu.kr/cohesion/internal/config"
	"taeu.kr/cohesion/internal/ftp"
	"taeu.kr/cohesion/internal/platform/database"
	"taeu.kr/cohesion/internal/platform/logging"
	"taeu.kr/cohesion/internal/platform/web"
	sftpserver "taeu.kr/cohesion/internal/sftp"
	"taeu.kr/cohesion/internal/smb"
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
	accessLogger = zerolog.New(io.Discard).
			With().
			Timestamp().
			Str(logging.FieldComponent, logging.ComponentAccess).
			Logger()
)

// 재시작 신호를 받기 위한 채널
var restartChan = make(chan bool, 1)
var shutdownChan = make(chan struct{}, 1)

func init() {
	zerolog.TimeFieldFormat = time.RFC3339
	zerolog.TimestampFieldName = logging.FieldTimestamp
	zerolog.MessageFieldName = logging.FieldMessage
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

func configureRootLoggers() (*os.File, *os.File, string, string, error) {
	appLogFile, appLogPath, err := openRootLogFile("app.log")
	if err != nil {
		return nil, nil, "", "", err
	}
	accessLogFile, accessLogPath, err := openRootLogFile("access.log")
	if err != nil {
		_ = appLogFile.Close()
		return nil, nil, "", "", err
	}

	operationalWriter := logging.NewOperationalWriter(appLogFile, os.Stderr)
	log.Logger = newOperationalLogger(operationalWriter)
	accessLogger = newAccessLogger(accessLogFile)

	return appLogFile, accessLogFile, appLogPath, accessLogPath, nil
}

func configureFallbackLoggers() {
	// Keep terminal operational policy even when file sinks cannot be initialized.
	log.Logger = newOperationalLogger(logging.NewOperationalWriter(io.Discard, os.Stderr))
	// Do not drop access logs on sink initialization failures.
	accessLogger = newAccessLogger(os.Stderr)
}

func newOperationalLogger(out io.Writer) zerolog.Logger {
	return zerolog.New(out).With().Timestamp().Logger()
}

func newAccessLogger(out io.Writer) zerolog.Logger {
	return zerolog.New(logging.NewKeyValueWriter(out)).
		With().
		Timestamp().
		Str(logging.FieldComponent, logging.ComponentAccess).
		Logger()
}

func emitAccessLog(r *http.Request, status, size int, duration time.Duration) {
	event := accessLogger.Info().
		Str(logging.FieldEvent, logging.EventHTTPAccess).
		Str("method", r.Method).
		Str("path", r.URL.Path)

	if rawQuery := strings.TrimSpace(r.URL.RawQuery); rawQuery != "" {
		event = event.Str("query", rawQuery)
	}

	event.
		Int("status", status).
		Int("size", size).
		Int64("duration_ms", duration.Milliseconds()).
		Msg("http request served")
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

func registerWebDAVRoutes(mux *http.ServeMux, handler http.Handler) {
	mux.Handle("/dav", handler)
	mux.Handle("/dav/", handler)
}

// createServer는 설정을 기반으로 HTTP 서버를 생성합니다
func createServer(db *sql.DB, restartChan chan bool, shutdownChan chan struct{}) (*http.Server, *ftp.Service, *sftpserver.Service, *smb.Service, *audit.Service, error) {
	if err := configureSMBMaterialKeyPolicy(config.Conf.Server.SmbEnabled); err != nil {
		return nil, nil, nil, nil, nil, err
	}

	// 의존성 주입
	accountRepo := accountStore.NewStore(db)
	accountService := account.NewService(accountRepo)
	if err := accountService.EnsureDefaultAdmin(context.Background()); err != nil {
		return nil, nil, nil, nil, nil, err
	}
	authSecret, err := resolveJWTSecret()
	if err != nil {
		return nil, nil, nil, nil, nil, err
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
	auditRepo := auditStore.NewStore(db)
	spaceService := space.NewService(spaceRepo)
	trashService := space.NewTrashService(trashRepo)
	auditService := audit.NewService(auditRepo, audit.Config{BufferSize: 512})
	browseService := browse.NewService()
	spaceHandler := spaceHandler.NewHandler(spaceService, browseService, accountService, trashService)
	browseHandler := browseHandler.NewHandler(browseService, spaceService)
	auditHandler := audit.NewHandler(auditService)
	webDavService := webdav.NewService(spaceService, accountService)
	webDavHandler := webdavHandler.NewHandler(webDavService, accountService)
	ftpService := ftp.NewService(spaceService, accountService, config.Conf.Server.FtpEnabled, config.Conf.Server.FtpPort)
	sftpService := sftpserver.NewService(spaceService, accountService, config.Conf.Server.SftpEnabled, config.Conf.Server.SftpPort)
	smbService := smb.NewService(
		spaceService,
		accountService,
		config.Conf.Server.SmbEnabled,
		config.Conf.Server.SmbPort,
		config.Conf.Server.SmbRolloutPhase,
	)
	statusHandler := status.NewHandler(db, spaceService, smbService, config.Conf.Server.Port)
	configHandler := config.NewHandler()
	systemHandler := system.NewHandler(restartChan, shutdownChan, system.Meta{
		Version:   appVersion,
		Commit:    appCommit,
		BuildDate: appBuildDate,
	})
	authService.SetAuditRecorder(auditService)
	accountHandler.SetAuditRecorder(auditService)
	spaceHandler.SetAuditRecorder(auditService)
	configHandler.SetAuditRecorder(auditService)
	systemHandler.SetAuditRecorder(auditService)

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
	auditHandler.RegisterRoutes(mux)

	// WebDAV 핸들러 등록
	if config.Conf.Server.WebdavEnabled {
		// 일부 DAV 클라이언트는 "/dav" -> "/dav/" 리다이렉트 시 메서드를 보존하지 않아
		// PROPFIND/OPTIONS 대신 GET으로 재요청할 수 있다. (결과: 405)
		// 두 경로를 모두 직접 등록해 리다이렉트를 피한다.
		registerWebDAVRoutes(mux, web.Handler(func(w http.ResponseWriter, r *http.Request) *web.Error {
			return webDavHandler.ServeHTTP(w, r)
		}))
	}

	if goEnv == "production" {
		spaHandler, err := spa.NewSPAHandler(WebDist, "dist/web")
		if err != nil {
			return nil, nil, nil, nil, nil, err
		}
		mux.HandleFunc("/", spaHandler)
	}

	// hlog.NewHanlder는 Zerolog 컨텍스트를 HTTP 핸들러에 주입
	hlogHandler := hlog.NewHandler(log.Logger)

	// hlog.AccessHandler는 요청 및 응답에 대한 로그를 기록
	finalHandler := authService.Middleware(mux)

	finalLogHandler := hlogHandler(hlog.AccessHandler(func(r *http.Request, status, size int, duration time.Duration) {
		emitAccessLog(r, status, size, duration)
	})(finalHandler))

	port := ":" + config.Conf.Server.Port
	server := &http.Server{
		Addr:    port,
		Handler: finalLogHandler,
	}

	return server, ftpService, sftpService, smbService, auditService, nil
}

func main() {
	appLogFile, accessLogFile, appLogPath, accessLogPath, loggerErr := configureRootLoggers()
	if loggerErr != nil {
		configureFallbackLoggers()
		logging.Event(log.Error(), logging.ComponentMain, "error.logger.init_failed").
			Err(loggerErr).
			Msg("failed to initialize file log sinks; using stderr fallback")
	} else {
		defer appLogFile.Close()
		defer accessLogFile.Close()
		logging.Event(log.Info(), logging.ComponentMain, logging.EventServiceReady).
			Str("service", "app-log").
			Str("path", appLogPath).
			Msg("service log sink ready")
		logging.Event(log.Info(), logging.ComponentMain, logging.EventServiceReady).
			Str("service", "access-log").
			Str("path", accessLogPath).
			Msg("service log sink ready")
	}

	pidCleanup, pidPath, pidErr := writePIDFile()
	if pidErr != nil {
		logging.Event(log.Warn(), logging.ComponentMain, "warn.pid.write_failed").
			Err(pidErr).
			Msg("failed to write pid file")
	} else {
		defer pidCleanup()
		logging.Event(log.Info(), logging.ComponentMain, logging.EventServiceReady).
			Str("service", "pid-file").
			Str("path", pidPath).
			Int("pid", os.Getpid()).
			Msg("service pid file ready")
	}

	logging.Event(log.Info(), logging.ComponentMain, logging.EventBootStart).
		Str("environment", goEnv).
		Str("version", appVersion).
		Str("commit", appCommit).
		Str("build_date", appBuildDate).
		Msg("server booting")

	// 설정 로드
	config.SetConfig(goEnv)
	logging.Event(log.Info(), logging.ComponentConfig, logging.EventConfigLoaded).
		Str("environment", goEnv).
		Str("config_dir", config.ConfigDir()).
		Msg("configuration loaded")

	// 데이터베이스 연결 설정
	db, err := database.NewDB()
	if err != nil {
		logging.Event(log.Fatal(), logging.ComponentDB, "fatal.db.connect_failed").
			Err(err).
			Msg("failed to connect database")
	}
	defer db.Close()
	logging.Event(log.Info(), logging.ComponentDB, logging.EventDatabaseReady).
		Str("datasource_url", config.Conf.Datasource.URL).
		Msg("database connected")

	// OS 시그널 핸들링 (Ctrl+C)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 서버 시작/재시작 루프
	for {
		// 서버 생성
		server, ftpService, sftpService, smbService, auditService, err := createServer(db, restartChan, shutdownChan)
		if err != nil {
			logging.Event(log.Fatal(), logging.ComponentServer, "fatal.server.create_failed").
				Err(err).
				Msg("failed to create server")
		}
		if err := ftpService.Start(); err != nil {
			logging.Event(log.Fatal(), logging.ComponentServer, "fatal.service.start_failed").
				Str("service", "ftp").
				Int("port", ftpService.Port()).
				Err(err).
				Msg("failed to start service")
		}
		logging.Event(log.Info(), logging.ComponentServer, logging.EventServiceReady).
			Str("service", "ftp").
			Bool("enabled", ftpService.Enabled()).
			Int("port", ftpService.Port()).
			Msg("service status updated")
		if err := sftpService.Start(); err != nil {
			if stopErr := ftpService.Stop(); stopErr != nil {
				logging.Event(log.Error(), logging.ComponentServer, "error.service.stop_failed").
					Str("service", "ftp").
					Err(stopErr).
					Msg("failed to stop service")
			}
			logging.Event(log.Fatal(), logging.ComponentServer, "fatal.service.start_failed").
				Str("service", "sftp").
				Int("port", sftpService.Port()).
				Err(err).
				Msg("failed to start service")
		}
		if err := smbService.Start(); err != nil {
			logging.Event(log.Error(), logging.ComponentServer, "error.service.start_failed").
				Str("service", "smb").
				Int("port", smbService.Port()).
				Err(err).
				Msg("failed to start service; continuing without smb runtime")
		}
		logging.Event(log.Info(), logging.ComponentServer, logging.EventServiceReady).
			Str("service", "smb").
			Bool("enabled", smbService.Enabled()).
			Int("port", smbService.Port()).
			Str("endpoint_mode", smbService.EndpointMode()).
			Str("min_version", smbService.MinVersion()).
			Str("max_version", smbService.MaxVersion()).
			Msg("service status updated")
		logging.Event(log.Info(), logging.ComponentServer, logging.EventServiceReady).
			Str("service", "sftp").
			Bool("enabled", sftpService.Enabled()).
			Int("port", sftpService.Port()).
			Msg("service status updated")

		port := config.Conf.Server.Port
		logging.Event(log.Info(), logging.ComponentServer, logging.EventServiceReady).
			Str("service", "http").
			Str("port", port).
			Msg("service status updated")
		logging.Event(log.Info(), logging.ComponentServer, logging.EventServiceReady).
			Str("service", "webdav").
			Bool("enabled", config.Conf.Server.WebdavEnabled).
			Msg("service status updated")
		logging.Event(log.Info(), logging.ComponentServer, logging.EventServerReady).
			Str("port", port).
			Msg("server ready")

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
			logging.Event(log.Info(), logging.ComponentServer, logging.EventServerShutdownSignal).
				Str("source", "signal").
				Msg("shutdown requested")
			if err := sftpService.Stop(); err != nil {
				logging.Event(log.Error(), logging.ComponentServer, "error.service.stop_failed").
					Str("service", "sftp").
					Err(err).
					Msg("failed to stop service")
			}
			if err := smbService.Stop(); err != nil {
				logging.Event(log.Error(), logging.ComponentServer, "error.service.stop_failed").
					Str("service", "smb").
					Err(err).
					Msg("failed to stop service")
			}
			if err := ftpService.Stop(); err != nil {
				logging.Event(log.Error(), logging.ComponentServer, "error.service.stop_failed").
					Str("service", "ftp").
					Err(err).
					Msg("failed to stop service")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := server.Shutdown(ctx); err != nil {
				logging.Event(log.Error(), logging.ComponentServer, "error.server.shutdown_failed").
					Err(err).
					Msg("failed to shutdown server")
			}
			closeAuditService(auditService)
			logging.Event(log.Info(), logging.ComponentServer, logging.EventServerShutdownDone).
				Str("source", "signal").
				Msg("server shutdown completed")
			return

		case <-restartChan:
			logging.Event(log.Info(), logging.ComponentServer, logging.EventServerRestartRequest).
				Msg("restart requested")
			if err := sftpService.Stop(); err != nil {
				logging.Event(log.Error(), logging.ComponentServer, "error.service.stop_failed").
					Str("service", "sftp").
					Err(err).
					Msg("failed to stop service")
			}
			if err := smbService.Stop(); err != nil {
				logging.Event(log.Error(), logging.ComponentServer, "error.service.stop_failed").
					Str("service", "smb").
					Err(err).
					Msg("failed to stop service")
			}
			if err := ftpService.Stop(); err != nil {
				logging.Event(log.Error(), logging.ComponentServer, "error.service.stop_failed").
					Str("service", "ftp").
					Err(err).
					Msg("failed to stop service")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := server.Shutdown(ctx); err != nil {
				logging.Event(log.Error(), logging.ComponentServer, "error.server.shutdown_failed").
					Err(err).
					Msg("failed to shutdown server")
			}
			cancel()
			closeAuditService(auditService)

			// 설정 다시 로드
			config.SetConfig(goEnv)
			logging.Event(log.Info(), logging.ComponentConfig, logging.EventConfigLoaded).
				Str("environment", goEnv).
				Str("config_dir", config.ConfigDir()).
				Msg("configuration loaded")
			logging.Event(log.Info(), logging.ComponentServer, logging.EventServerRestarted).
				Str("port", config.Conf.Server.Port).
				Msg("restart completed")
		// 루프 계속 (재시작)

		case <-shutdownChan:
			logging.Event(log.Info(), logging.ComponentServer, logging.EventServerShutdownSignal).
				Str("source", "updater").
				Msg("shutdown requested")
			if err := sftpService.Stop(); err != nil {
				logging.Event(log.Error(), logging.ComponentServer, "error.service.stop_failed").
					Str("service", "sftp").
					Err(err).
					Msg("failed to stop service")
			}
			if err := smbService.Stop(); err != nil {
				logging.Event(log.Error(), logging.ComponentServer, "error.service.stop_failed").
					Str("service", "smb").
					Err(err).
					Msg("failed to stop service")
			}
			if err := ftpService.Stop(); err != nil {
				logging.Event(log.Error(), logging.ComponentServer, "error.service.stop_failed").
					Str("service", "ftp").
					Err(err).
					Msg("failed to stop service")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := server.Shutdown(ctx); err != nil {
				logging.Event(log.Error(), logging.ComponentServer, "error.server.shutdown_failed").
					Err(err).
					Msg("failed to shutdown server")
			}
			closeAuditService(auditService)
			logging.Event(log.Info(), logging.ComponentServer, logging.EventServerShutdownDone).
				Str("source", "updater").
				Msg("server shutdown completed")
			return

		case err := <-serverErr:
			if stopErr := sftpService.Stop(); stopErr != nil {
				logging.Event(log.Error(), logging.ComponentServer, "error.service.stop_failed").
					Str("service", "sftp").
					Err(stopErr).
					Msg("failed to stop service")
			}
			if stopErr := smbService.Stop(); stopErr != nil {
				logging.Event(log.Error(), logging.ComponentServer, "error.service.stop_failed").
					Str("service", "smb").
					Err(stopErr).
					Msg("failed to stop service")
			}
			if stopErr := ftpService.Stop(); stopErr != nil {
				logging.Event(log.Error(), logging.ComponentServer, "error.service.stop_failed").
					Str("service", "ftp").
					Err(stopErr).
					Msg("failed to stop service")
			}
			closeAuditService(auditService)
			logging.Event(log.Fatal(), logging.ComponentServer, "fatal.server.runtime_failed").
				Err(err).
				Msg("server runtime failure")
			return
		}
	}
}

func closeAuditService(auditService *audit.Service) {
	if auditService == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := auditService.Close(ctx); err != nil {
		logging.Event(log.Warn(), logging.ComponentAudit, "warn.audit.shutdown_timeout").
			Err(err).
			Msg("audit service shutdown timed out")
	}
}

func readEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func configureSMBMaterialKeyPolicy(smbEnabled bool) error {
	requireSMBMaterialKey := goEnv == "production" && smbEnabled
	account.SetSMBMaterialKeyRequired(requireSMBMaterialKey)

	if err := account.ValidateSMBMaterialKeyConfiguration(); err != nil {
		logging.Event(log.Error(), logging.ComponentAuth, "error.smb.material_key_invalid").
			Err(err).
			Str("environment", goEnv).
			Bool("smb_enabled", smbEnabled).
			Bool("key_required", requireSMBMaterialKey).
			Msg("invalid smb material key configuration")
		return err
	}

	logging.Event(log.Info(), logging.ComponentAuth, logging.EventServiceReady).
		Str("service", "smb-material-key-boundary").
		Str("environment", goEnv).
		Bool("smb_enabled", smbEnabled).
		Bool("key_required", requireSMBMaterialKey).
		Str("key_source", account.CurrentSMBMaterialKeySource()).
		Msg("service status updated")
	return nil
}

func resolveJWTSecret() (string, error) {
	secret := strings.TrimSpace(os.Getenv("COHESION_JWT_SECRET"))
	if secret != "" {
		if goEnv == "production" && len(secret) < 32 {
			return "", errors.New("COHESION_JWT_SECRET must be at least 32 characters in production")
		}
		logging.Event(log.Info(), logging.ComponentAuth, logging.EventServiceReady).
			Str("service", "jwt-secret").
			Str("source", "env:COHESION_JWT_SECRET").
			Msg("service status updated")
		return secret, nil
	}

	secretFilePath, err := resolveJWTSecretPath()
	if err != nil {
		return "", err
	}

	secret, created, err := loadOrCreateJWTSecret(secretFilePath, goEnv != "production")
	if err != nil {
		return "", err
	}

	if goEnv == "production" && len(secret) < 32 {
		return "", errors.New("COHESION_JWT_SECRET must be at least 32 characters in production")
	}

	source := "file:existing"
	if created {
		source = "file:generated"
	}
	logging.Event(log.Info(), logging.ComponentAuth, logging.EventServiceReady).
		Str("service", "jwt-secret").
		Str("source", source).
		Str("path", secretFilePath).
		Msg("service status updated")
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

func loadOrCreateJWTSecret(path string, allowCreate bool) (string, bool, error) {
	content, err := os.ReadFile(path)
	if err == nil {
		secret := strings.TrimSpace(string(content))
		if secret != "" {
			return secret, false, nil
		}
		if !allowCreate {
			return "", false, errors.New("COHESION_JWT_SECRET is required in production (secret file is empty)")
		}
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", false, err
	}
	if !allowCreate {
		return "", false, errors.New("COHESION_JWT_SECRET is required in production (set env or provide secret file)")
	}

	secret, err := generateRandomSecret(48)
	if err != nil {
		return "", false, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", false, err
	}
	if err := os.WriteFile(path, []byte(secret+"\n"), 0600); err != nil {
		return "", false, err
	}

	return secret, true, nil
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
