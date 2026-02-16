package ftp

import (
	"errors"
	"fmt"
	"sync"
	"time"

	goftp "github.com/goftp/server"
	"github.com/rs/zerolog/log"
	"taeu.kr/cohesion/internal/account"
	"taeu.kr/cohesion/internal/space"
)

const (
	defaultFTPPort = 2121
)

type Service struct {
	spaceService   *space.Service
	accountService *account.Service
	server         *goftp.Server
	enabled        bool
	port           int
	running        bool
	mu             sync.RWMutex
}

func NewService(spaceService *space.Service, accountService *account.Service, enabled bool, port int) *Service {
	if port <= 0 {
		port = defaultFTPPort
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

	opts := &goftp.ServerOpts{
		Factory:        &driverFactory{spaceService: s.spaceService, accountService: s.accountService},
		Port:           s.port,
		Hostname:       "0.0.0.0",
		Name:           "Cohesion FTP",
		WelcomeMessage: "Cohesion FTP",
		Auth:           &accountAuth{accountService: s.accountService},
		Logger:         &ftpLogger{},
	}

	ftpServer := goftp.NewServer(opts)
	errCh := make(chan error, 1)

	go func() {
		if err := ftpServer.ListenAndServe(); err != nil && !errors.Is(err, goftp.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("failed to start ftp server on port %d: %w", s.port, err)
	case <-time.After(200 * time.Millisecond):
		s.server = ftpServer
		s.running = true
		log.Info().Msgf("[FTP] server started on port %d", s.port)
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

	if err := s.server.Shutdown(); err != nil {
		return err
	}

	s.server = nil
	s.running = false
	log.Info().Msg("[FTP] server stopped")
	return nil
}

func (s *Service) Enabled() bool {
	return s.enabled
}

func (s *Service) Port() int {
	return s.port
}
