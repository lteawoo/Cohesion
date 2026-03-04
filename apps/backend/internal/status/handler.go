package status

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"taeu.kr/cohesion/internal/config"
	"taeu.kr/cohesion/internal/platform/web"
	"taeu.kr/cohesion/internal/smb"
	"taeu.kr/cohesion/internal/space"
)

type ProtocolStatus struct {
	Status       string `json:"status"`
	Message      string `json:"message"`
	Reason       string `json:"reason,omitempty"`
	Port         string `json:"port,omitempty"`
	Path         string `json:"path,omitempty"`
	EndpointMode string `json:"endpointMode,omitempty"`
	RolloutPhase string `json:"rolloutPhase,omitempty"`
	PolicySource string `json:"policySource,omitempty"`
	BindReady    bool   `json:"bindReady,omitempty"`
	RuntimeReady bool   `json:"runtimeReady,omitempty"`
	MinVersion   string `json:"minVersion,omitempty"`
	MaxVersion   string `json:"maxVersion,omitempty"`
}

type StatusResponse struct {
	Protocols map[string]ProtocolStatus `json:"protocols"`
	Hosts     []string                  `json:"hosts"`
}

type Handler struct {
	db              *sql.DB
	spaceService    *space.Service
	smbReadinessSvc smbReadinessProvider
	port            string
}

type smbReadinessProvider interface {
	Readiness() smb.Readiness
}

func NewHandler(db *sql.DB, spaceService *space.Service, smbReadinessSvc smbReadinessProvider, port string) *Handler {
	return &Handler{
		db:              db,
		spaceService:    spaceService,
		smbReadinessSvc: smbReadinessSvc,
		port:            port,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("/api/status", web.Handler(h.handleStatus))
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) *web.Error {
	if r.Method != http.MethodGet {
		return &web.Error{
			Code:    http.StatusMethodNotAllowed,
			Message: "Method not allowed",
		}
	}

	protocols := make(map[string]ProtocolStatus)

	// WEB (DB 연결 기반)
	protocols["http"] = h.checkHTTP()

	// WebDAV (Space 조회 가능 여부)
	protocols["webdav"] = h.checkWebDAV()

	protocols["ftp"] = h.checkFTP()
	protocols["sftp"] = h.checkSFTP()
	protocols["smb"] = h.checkSMB()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(StatusResponse{
		Protocols: protocols,
		Hosts:     h.getAccessibleHosts(),
	})

	return nil
}

func (h *Handler) checkHTTP() ProtocolStatus {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := h.db.PingContext(ctx); err != nil {
		return ProtocolStatus{
			Status:  "unhealthy",
			Message: "DB 연결 실패",
			Path:    "/",
		}
	}

	return ProtocolStatus{
		Status:  "healthy",
		Message: "정상",
		Path:    "/",
	}
}

func (h *Handler) checkWebDAV() ProtocolStatus {
	if !config.Conf.Server.WebdavEnabled {
		return ProtocolStatus{
			Status:  "unavailable",
			Message: "비활성화",
			Port:    h.port,
			Path:    "/dav/",
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := h.spaceService.GetAllSpaces(ctx)
	if err != nil {
		return ProtocolStatus{
			Status:  "unhealthy",
			Message: "Space 서비스 오류",
			Port:    h.port,
			Path:    "/dav/",
		}
	}

	return ProtocolStatus{
		Status:  "healthy",
		Message: "정상",
		Port:    h.port,
		Path:    "/dav/",
	}
}

func (h *Handler) checkFTP() ProtocolStatus {
	if !config.Conf.Server.FtpEnabled {
		return ProtocolStatus{
			Status:  "unavailable",
			Message: "비활성화",
			Port:    strconv.Itoa(config.Conf.Server.FtpPort),
		}
	}

	if config.Conf.Server.FtpPort <= 0 {
		return ProtocolStatus{
			Status:  "unhealthy",
			Message: "포트 설정 오류",
		}
	}

	port := strconv.Itoa(config.Conf.Server.FtpPort)
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", port), 1500*time.Millisecond)
	if err != nil {
		return ProtocolStatus{
			Status:  "unhealthy",
			Message: "연결 실패",
			Port:    port,
		}
	}
	_ = conn.Close()

	return ProtocolStatus{
		Status:  "healthy",
		Message: "정상",
		Port:    port,
	}
}

func (h *Handler) checkSFTP() ProtocolStatus {
	if !config.Conf.Server.SftpEnabled {
		return ProtocolStatus{
			Status:  "unavailable",
			Message: "비활성화",
			Port:    strconv.Itoa(config.Conf.Server.SftpPort),
		}
	}

	if config.Conf.Server.SftpPort <= 0 {
		return ProtocolStatus{
			Status:  "unhealthy",
			Message: "포트 설정 오류",
		}
	}

	port := strconv.Itoa(config.Conf.Server.SftpPort)
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", port), 1500*time.Millisecond)
	if err != nil {
		return ProtocolStatus{
			Status:  "unhealthy",
			Message: "연결 실패",
			Port:    port,
		}
	}
	_ = conn.Close()

	return ProtocolStatus{
		Status:  "healthy",
		Message: "정상",
		Port:    port,
	}
}

func (h *Handler) checkSMB() ProtocolStatus {
	port := config.Conf.Server.SmbPort
	rolloutPhase := config.Conf.Server.SmbRolloutPhase
	if rolloutPhase == "" {
		rolloutPhase = config.DefaultSMBRolloutPhase
	}

	status := ProtocolStatus{
		Port:         strconv.Itoa(port),
		EndpointMode: config.SMBEndpointModeDirect,
		RolloutPhase: rolloutPhase,
		PolicySource: "config",
		MinVersion:   config.DefaultSMBMinVersion,
		MaxVersion:   config.DefaultSMBMaxVersion,
	}

	if !config.Conf.Server.SmbEnabled {
		status.Status = "unavailable"
		status.Message = "비활성화"
		return status
	}

	if config.Conf.Server.SmbPort <= 0 {
		status.Status = "unhealthy"
		status.Message = "포트 설정 오류"
		status.Port = ""
		return status
	}

	if h.smbReadinessSvc == nil {
		status.Status = "unhealthy"
		status.Message = "준비 상태 확인 불가"
		return status
	}

	readiness := h.smbReadinessSvc.Readiness()
	if readiness.EndpointMode != "" {
		status.EndpointMode = readiness.EndpointMode
	}
	if readiness.RolloutPhase != "" {
		status.RolloutPhase = readiness.RolloutPhase
	}
	if readiness.PolicySource != "" {
		status.PolicySource = readiness.PolicySource
	}
	status.BindReady = readiness.BindReady
	status.RuntimeReady = readiness.RuntimeReady
	switch readiness.State {
	case smb.StateHealthy:
		status.Status = "healthy"
		status.Message = readiness.Message
	case smb.StateUnavailable:
		status.Status = "unavailable"
		status.Message = readiness.Message
	default:
		status.Status = "unhealthy"
		status.Message = readiness.Message
	}
	status.Reason = readiness.Reason

	if status.Message == "" {
		status.Message = "SMB 상태 미상"
	}
	return status
}

func (h *Handler) getAccessibleHosts() []string {
	hosts := []string{fmt.Sprintf("localhost:%s", h.port)}

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return hosts
	}

	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP.IsLoopback() || ipNet.IP.To4() == nil {
			continue
		}
		hosts = append(hosts, fmt.Sprintf("%s:%s", ipNet.IP.String(), h.port))
	}

	return hosts
}
