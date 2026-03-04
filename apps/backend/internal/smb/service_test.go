package smb

import (
	"net"
	"strconv"
	"testing"
	"time"
)

func TestService_StartStop_Disabled(t *testing.T) {
	svc := NewService(nil, nil, false, 1445, "readonly")
	if err := svc.Start(); err != nil {
		t.Fatalf("start disabled service: %v", err)
	}
	readiness := svc.Readiness()
	if readiness.State != StateUnavailable {
		t.Fatalf("expected unavailable readiness, got %q", readiness.State)
	}
	if readiness.Reason != ReasonDisabled {
		t.Fatalf("expected disabled reason, got %q", readiness.Reason)
	}
	if err := svc.Stop(); err != nil {
		t.Fatalf("stop disabled service: %v", err)
	}
}

func TestService_StartStop_Enabled(t *testing.T) {
	port := reservePort(t)
	svc := NewService(nil, nil, true, port, "readonly")

	if err := svc.Start(); err != nil {
		t.Fatalf("start service: %v", err)
	}
	defer svc.Stop()

	addr := net.JoinHostPort("127.0.0.1", toPortString(port))
	if err := waitDial(addr, 1*time.Second); err != nil {
		t.Fatalf("expected service to accept tcp connections: %v", err)
	}
	readiness := svc.Readiness()
	if readiness.State != StateUnhealthy {
		t.Fatalf("expected unhealthy readiness before runtime integration, got %q", readiness.State)
	}
	if readiness.Reason != ReasonRuntimeNotReady {
		t.Fatalf("expected runtime-not-ready reason, got %q", readiness.Reason)
	}
	if readiness.Message != "SMB readonly 프로토콜 준비 안됨" {
		t.Fatalf("unexpected readiness message: %q", readiness.Message)
	}
	if readiness.RolloutPhase != "readonly" {
		t.Fatalf("unexpected rollout phase: %q", readiness.RolloutPhase)
	}
	if !readiness.BindReady {
		t.Fatal("expected bind-ready=true after start")
	}

	if err := svc.Stop(); err != nil {
		t.Fatalf("stop service: %v", err)
	}
	if err := waitDial(addr, 300*time.Millisecond); err == nil {
		t.Fatal("expected connection failure after stop")
	}
}

func TestService_StartFailure_UpdatesReadiness(t *testing.T) {
	busy, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatalf("listen busy: %v", err)
	}
	defer busy.Close()

	port := busy.Addr().(*net.TCPAddr).Port
	svc := NewService(nil, nil, true, port, "readonly")
	if err := svc.Start(); err == nil {
		t.Fatal("expected start failure on busy port")
	}

	readiness := svc.Readiness()
	if readiness.State != StateUnhealthy {
		t.Fatalf("expected unhealthy readiness, got %q", readiness.State)
	}
	if readiness.Stage != StageBind {
		t.Fatalf("expected bind stage, got %q", readiness.Stage)
	}
	if readiness.Reason != ReasonRuntimeError {
		t.Fatalf("expected runtime_error reason, got %q", readiness.Reason)
	}
}

func TestService_AcceptLoop_DoesNotBlockOnSlowConnection(t *testing.T) {
	accountSvc, spaceSvc, db := setupGuardServices(t)
	defer db.Close()

	port := reservePort(t)
	svc := NewService(spaceSvc, accountSvc, true, port, "readonly")
	if err := svc.Start(); err != nil {
		t.Fatalf("start service: %v", err)
	}
	defer svc.Stop()

	addr := net.JoinHostPort("127.0.0.1", toPortString(port))

	conn1, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial first connection: %v", err)
	}
	defer conn1.Close()

	conn2, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial second connection: %v", err)
	}
	defer conn2.Close()

	// SMB1 probe should be rejected and connection closed quickly.
	frame := []byte{
		0x00, 0x00, 0x00, 0x08,
		0xFF, 'S', 'M', 'B', 0x72, 0x00, 0x00, 0x00,
	}
	if _, err := conn2.Write(frame); err != nil {
		t.Fatalf("write smb1 probe frame: %v", err)
	}
	_ = conn2.SetReadDeadline(time.Now().Add(700 * time.Millisecond))
	buf := make([]byte, 1)
	if _, err := conn2.Read(buf); err == nil {
		t.Fatal("expected second connection to be closed after smb1 rejection")
	} else if ne, ok := err.(net.Error); ok && ne.Timeout() {
		t.Fatal("second connection timed out, accept-loop may be blocked by first connection")
	}
}

func reservePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func waitDial(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		conn, err := net.DialTimeout("tcp", addr, 80*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return err
		}
		time.Sleep(40 * time.Millisecond)
	}
}

func toPortString(port int) string {
	return strconv.Itoa(port)
}
