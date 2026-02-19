package sftp

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	gliderssh "github.com/gliderlabs/ssh"
	pkgsftp "github.com/pkg/sftp"
	"github.com/rs/zerolog/log"
	xssh "golang.org/x/crypto/ssh"
	"taeu.kr/cohesion/internal/account"
	"taeu.kr/cohesion/internal/config"
	"taeu.kr/cohesion/internal/space"
)

const (
	defaultSFTPPort          = 2222
	sftpKeyDirPermission     = 0700
	sftpKeyFilePermission    = 0600
	defaultSFTPHostKeyName   = "sftp_host_ed25519_key"
	sftpHostKeyFilePathEnv   = "COHESION_SFTP_HOST_KEY_FILE"
	sftpServerShutdownTimout = 3 * time.Second
)

type Service struct {
	spaceService   *space.Service
	accountService *account.Service
	server         *gliderssh.Server
	enabled        bool
	port           int
	running        bool
	mu             sync.RWMutex
}

func NewService(spaceService *space.Service, accountService *account.Service, enabled bool, port int) *Service {
	if port <= 0 {
		port = defaultSFTPPort
	}

	return &Service{
		spaceService:   spaceService,
		accountService: accountService,
		enabled:        enabled,
		port:           port,
	}
}

func (s *Service) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.enabled {
		s.running = false
		return nil
	}
	if s.running {
		return nil
	}

	hostSigner, hostKeyPath, err := loadOrCreateHostSigner()
	if err != nil {
		return err
	}

	sftpServer := &gliderssh.Server{
		Addr: fmt.Sprintf("0.0.0.0:%d", s.port),
		Handler: func(session gliderssh.Session) {
			_, _ = io.WriteString(session, "This endpoint supports SFTP subsystem only.\n")
			_ = session.Exit(1)
		},
		PasswordHandler: s.passwordHandler,
		SubsystemHandlers: map[string]gliderssh.SubsystemHandler{
			"sftp": s.handleSFTPSubsystem,
		},
		IdleTimeout: 5 * time.Minute,
	}
	sftpServer.AddHostKey(hostSigner)

	errCh := make(chan error, 1)
	go func() {
		if err := sftpServer.ListenAndServe(); err != nil && !errors.Is(err, net.ErrClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("failed to start sftp server on port %d: %w", s.port, err)
	case <-time.After(200 * time.Millisecond):
		s.server = sftpServer
		s.running = true
		log.Info().
			Int("port", s.port).
			Str("host_key_path", hostKeyPath).
			Msg("[SFTP] server started")
		return nil
	}
}

func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.server == nil {
		s.running = false
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), sftpServerShutdownTimout)
	defer cancel()
	if err := s.server.Shutdown(ctx); err != nil && !errors.Is(err, net.ErrClosed) {
		return err
	}

	s.server = nil
	s.running = false
	log.Info().Msg("[SFTP] server stopped")
	return nil
}

func (s *Service) Enabled() bool {
	return s.enabled
}

func (s *Service) Port() int {
	return s.port
}

func (s *Service) passwordHandler(ctx gliderssh.Context, password string) bool {
	authed, err := s.accountService.Authenticate(context.Background(), ctx.User(), password)
	if err != nil {
		log.Warn().Err(err).Str("user", ctx.User()).Msg("[SFTP] authentication failed")
		return false
	}
	if !authed {
		log.Warn().Str("user", ctx.User()).Msg("[SFTP] invalid credentials")
	}
	return authed
}

func (s *Service) handleSFTPSubsystem(session gliderssh.Session) {
	handlers := newSpaceHandlers(s.spaceService, s.accountService, session.User())
	requestServer := pkgsftp.NewRequestServer(session, pkgsftp.Handlers{
		FileGet:  handlers,
		FilePut:  handlers,
		FileCmd:  handlers,
		FileList: handlers,
	})

	err := requestServer.Serve()
	if errors.Is(err, io.EOF) {
		_ = requestServer.Close()
		return
	}
	if err != nil {
		_ = requestServer.Close()
		log.Warn().Err(err).Str("user", session.User()).Msg("[SFTP] request server error")
	}
}

func loadOrCreateHostSigner() (xssh.Signer, string, error) {
	keyPath, err := resolveHostKeyPath()
	if err != nil {
		return nil, "", err
	}

	keyBytes, err := os.ReadFile(keyPath)
	if err == nil {
		signer, parseErr := xssh.ParsePrivateKey(keyBytes)
		if parseErr != nil {
			return nil, "", fmt.Errorf("parse sftp host key: %w", parseErr)
		}
		return signer, keyPath, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, "", fmt.Errorf("read sftp host key: %w", err)
	}

	signer, pemBytes, err := generateHostSigner()
	if err != nil {
		return nil, "", err
	}

	if err := os.MkdirAll(filepath.Dir(keyPath), sftpKeyDirPermission); err != nil {
		return nil, "", fmt.Errorf("create sftp host key directory: %w", err)
	}
	if err := os.WriteFile(keyPath, pemBytes, sftpKeyFilePermission); err != nil {
		return nil, "", fmt.Errorf("write sftp host key: %w", err)
	}

	return signer, keyPath, nil
}

func resolveHostKeyPath() (string, error) {
	if custom := strings.TrimSpace(os.Getenv(sftpHostKeyFilePathEnv)); custom != "" {
		return custom, nil
	}

	if configDir := strings.TrimSpace(config.ConfigDir()); configDir != "" {
		return filepath.Join(configDir, "secrets", defaultSFTPHostKeyName), nil
	}

	userConfigDir, err := os.UserConfigDir()
	if err == nil && strings.TrimSpace(userConfigDir) != "" {
		return filepath.Join(userConfigDir, "Cohesion", "secrets", defaultSFTPHostKeyName), nil
	}

	executablePath, err := os.Executable()
	if err != nil {
		return "", errors.New("failed to resolve sftp host key path")
	}
	return filepath.Join(filepath.Dir(executablePath), "data", defaultSFTPHostKeyName), nil
}

func generateHostSigner() (xssh.Signer, []byte, error) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate sftp host key: %w", err)
	}

	pkcs8Key, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal sftp host key: %w", err)
	}

	signer, err := xssh.NewSignerFromKey(privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("create sftp host signer: %w", err)
	}

	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8Key,
	})

	return signer, pemBytes, nil
}
