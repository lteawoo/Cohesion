package smb

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/lteawoo/smb-core"
	"github.com/rs/zerolog/log"
	"taeu.kr/cohesion/internal/account"
	"taeu.kr/cohesion/internal/config"
	"taeu.kr/cohesion/internal/space"
)

type Service struct {
	spaceService   *space.Service
	accountService *account.Service
	enabled        bool
	port           int
	rolloutPhase   string

	listener       net.Listener
	core           smbcore.Runtime
	running        bool
	bindReady      bool
	runtimeReady   bool
	lastError      error
	lastErrorStage FailureStage
	runtimeIssue   string
	mu             sync.RWMutex
}

func NewService(
	spaceService *space.Service,
	accountService *account.Service,
	enabled bool,
	port int,
	rolloutPhase string,
) *Service {
	if port <= 0 {
		port = config.DefaultSMBPort
	}
	rolloutPhase = normalizeRolloutPhase(rolloutPhase)

	var authAdapter smbcore.Authenticator
	var authzAdapter smbcore.Authorizer
	var fsAdapter smbcore.FileSystem
	if spaceService != nil && accountService != nil {
		authAdapter = &coreAuthenticator{accountService: accountService}
		authzAdapter = &coreAuthorizer{
			spaceService:   spaceService,
			accountService: accountService,
		}
		fsAdapter = &coreFileSystem{
			spaceService:   spaceService,
			accountService: accountService,
		}
	}

	var core smbcore.Runtime
	if authAdapter != nil && authzAdapter != nil && fsAdapter != nil {
		initialized, coreErr := smbcore.NewEngine(smbcore.Config{
			MinDialect:   toCoreDialect(config.DefaultSMBMinVersion),
			MaxDialect:   toCoreDialect(config.DefaultSMBMaxVersion),
			RolloutPhase: smbcore.RolloutPhase(rolloutPhase),
		}, authAdapter, authzAdapter, fsAdapter, nil)
		if coreErr != nil {
			log.Warn().
				Err(coreErr).
				Str("min_version", config.DefaultSMBMinVersion).
				Str("max_version", config.DefaultSMBMaxVersion).
				Str("rollout_phase", rolloutPhase).
				Msg("[SMB] failed to initialize smbcore boundary")
		} else {
			core = initialized
		}
	}

	return &Service{
		spaceService:   spaceService,
		accountService: accountService,
		enabled:        enabled,
		port:           port,
		rolloutPhase:   rolloutPhase,
		core:           core,
	}
}

func (s *Service) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.enabled {
		s.listener = nil
		s.running = false
		s.bindReady = false
		s.runtimeReady = false
		s.lastError = nil
		s.lastErrorStage = StageNone
		return nil
	}
	if s.running {
		return nil
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", s.port))
	if err != nil {
		s.listener = nil
		s.running = false
		s.bindReady = false
		s.runtimeReady = false
		s.lastError = err
		s.lastErrorStage = StageBind
		return fmt.Errorf("failed to start smb service on port %d: %w", s.port, err)
	}

	s.listener = listener
	s.running = true
	s.bindReady = true
	s.runtimeIssue = ""
	s.runtimeReady = s.core != nil && s.spaceService != nil && s.accountService != nil
	if s.runtimeReady {
		if checker, ok := s.core.(interface{ CheckUsability(context.Context) error }); ok {
			if err := checker.CheckUsability(context.Background()); err != nil {
				s.runtimeReady = false
				s.runtimeIssue = err.Error()
				log.Warn().
					Err(err).
					Str("stage", string(StageSession)).
					Str("reason", ReasonRuntimeNotReady).
					Msg("[SMB] runtime usability check failed")
			}
		}
	}
	s.lastError = nil
	s.lastErrorStage = StageNone
	go s.acceptLoop(listener)
	log.Info().
		Int("port", s.port).
		Str("endpoint_mode", config.SMBEndpointModeDirect).
		Str("rollout_phase", s.rolloutPhase).
		Str("min_version", config.DefaultSMBMinVersion).
		Str("max_version", config.DefaultSMBMaxVersion).
		Bool("bind_ready", s.bindReady).
		Bool("runtime_ready", s.runtimeReady).
		Msg("[SMB] service started")
	return nil
}

func (s *Service) acceptLoop(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			log.Warn().
				Err(err).
				Str("stage", string(StageAccept)).
				Str("reason", ReasonAcceptFailed).
				Msg("[SMB] accept failed")
			s.mu.Lock()
			s.lastError = err
			s.lastErrorStage = StageAccept
			s.mu.Unlock()
			continue
		}

		if s.core == nil {
			_ = conn.Close()
			continue
		}
		go s.handleConn(conn)
	}
}

func (s *Service) handleConn(conn net.Conn) {
	if err := s.core.HandleConn(context.Background(), conn); err != nil && !errors.Is(err, smbcore.ErrRuntimeNotImplemented) {
		log.Warn().
			Err(err).
			Str("stage", string(StageSession)).
			Str("reason", ReasonRuntimeError).
			Msg("[SMB] runtime error while handling session")
		s.mu.Lock()
		s.lastError = err
		s.lastErrorStage = StageSession
		s.mu.Unlock()
	}
}

func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener == nil {
		s.running = false
		s.bindReady = false
		s.runtimeReady = false
		return nil
	}

	if err := s.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		s.lastError = err
		s.lastErrorStage = StageStop
		return err
	}

	s.listener = nil
	s.running = false
	s.bindReady = false
	s.runtimeReady = false
	s.lastError = nil
	s.lastErrorStage = StageNone
	s.runtimeIssue = ""
	log.Info().Msg("[SMB] service stopped")
	return nil
}

func (s *Service) Enabled() bool {
	return s.enabled
}

func (s *Service) Port() int {
	return s.port
}

func (s *Service) EndpointMode() string {
	return config.SMBEndpointModeDirect
}

func (s *Service) MinVersion() string {
	return config.DefaultSMBMinVersion
}

func (s *Service) MaxVersion() string {
	return config.DefaultSMBMaxVersion
}

func (s *Service) Readiness() Readiness {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metadata := Readiness{
		Port:         s.port,
		EndpointMode: config.SMBEndpointModeDirect,
		RolloutPhase: s.rolloutPhase,
		PolicySource: "config",
		MinVersion:   config.DefaultSMBMinVersion,
		MaxVersion:   config.DefaultSMBMaxVersion,
		BindReady:    s.bindReady,
		RuntimeReady: s.runtimeReady,
	}

	if !s.enabled {
		metadata.State = StateUnavailable
		metadata.Reason = ReasonDisabled
		metadata.Message = "SMB 비활성화"
		return metadata
	}

	if s.lastError != nil {
		metadata.State = StateUnhealthy
		metadata.Reason = ReasonRuntimeError
		metadata.Stage = s.lastErrorStage
		metadata.Message = "SMB 런타임 오류"
		return metadata
	}

	if !s.running || !s.bindReady {
		metadata.State = StateUnhealthy
		metadata.Reason = ReasonBindNotReady
		metadata.Stage = StageBind
		metadata.Message = "SMB 바인드 준비 안됨"
		return metadata
	}

	if !s.runtimeReady {
		metadata.State = StateUnhealthy
		metadata.Reason = ReasonRuntimeNotReady
		metadata.Stage = StageSession
		if s.runtimeIssue != "" {
			metadata.Message = fmt.Sprintf("SMB %s 프로토콜 준비 안됨: %s", s.rolloutPhase, s.runtimeIssue)
		} else {
			metadata.Message = fmt.Sprintf("SMB %s 프로토콜 준비 안됨", s.rolloutPhase)
		}
		return metadata
	}

	metadata.State = StateHealthy
	metadata.Reason = ReasonReady
	metadata.Message = "정상"
	return metadata
}

func toCoreDialect(version string) smbcore.Dialect {
	switch version {
	case config.SMBVersion21:
		return smbcore.Dialect210
	case config.SMBVersion300:
		return smbcore.Dialect300
	case config.SMBVersion302:
		return smbcore.Dialect302
	case config.SMBVersion311:
		return smbcore.Dialect311
	default:
		return smbcore.Dialect311
	}
}

func normalizeRolloutPhase(value string) string {
	switch value {
	case config.SMBRolloutPhaseReadOnly, config.SMBRolloutPhaseWriteSafe, config.SMBRolloutPhaseWriteFull:
		return value
	default:
		return config.DefaultSMBRolloutPhase
	}
}
