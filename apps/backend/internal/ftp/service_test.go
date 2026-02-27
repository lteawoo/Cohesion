package ftp

import (
	"net"
	"testing"
)

func TestServiceStartDisabled(t *testing.T) {
	t.Parallel()

	svc := NewService(nil, nil, false, 2121)
	if err := svc.Start(); err != nil {
		t.Fatalf("start disabled service: %v", err)
	}
	if err := svc.Stop(); err != nil {
		t.Fatalf("stop disabled service: %v", err)
	}
}

func TestServiceStartFailsWhenPortInUse(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}

	first := NewService(nil, nil, true, port)
	if err := first.Start(); err != nil {
		t.Fatalf("start first service: %v", err)
	}
	defer func() {
		if err := first.Stop(); err != nil {
			t.Fatalf("stop first service: %v", err)
		}
	}()

	second := NewService(nil, nil, true, port)
	if err := second.Start(); err == nil {
		t.Fatalf("expected start to fail when port %d is already in use", port)
	}
}
