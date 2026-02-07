package status

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"taeu.kr/cohesion/internal/platform/web"
	"taeu.kr/cohesion/internal/space"
)

type ProtocolStatus struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Port    string `json:"port,omitempty"`
	Path    string `json:"path,omitempty"`
}

type StatusResponse struct {
	Protocols map[string]ProtocolStatus `json:"protocols"`
	Hosts     []string                  `json:"hosts"`
}

type Handler struct {
	db           *sql.DB
	spaceService *space.Service
	port         string
}

func NewHandler(db *sql.DB, spaceService *space.Service, port string) *Handler {
	return &Handler{
		db:           db,
		spaceService: spaceService,
		port:         port,
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

	// HTTP (DB 연결 기반)
	protocols["http"] = h.checkHTTP()

	// WebDAV (Space 조회 가능 여부)
	protocols["webdav"] = h.checkWebDAV()

	// FTP (미구현)
	protocols["ftp"] = ProtocolStatus{
		Status:  "unavailable",
		Message: "미구현",
		Port:    "21",
	}

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
			Port:    h.port,
			Path:    "/api/",
		}
	}

	return ProtocolStatus{
		Status:  "healthy",
		Message: "정상",
		Port:    h.port,
		Path:    "/api/",
	}
}

func (h *Handler) checkWebDAV() ProtocolStatus {
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
