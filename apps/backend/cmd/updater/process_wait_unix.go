//go:build !windows

package main

import (
	"errors"
	"fmt"
	"syscall"
	"time"
)

func waitForProcessExit(pid int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		if err != nil && !errors.Is(err, syscall.EPERM) {
			return fmt.Errorf("failed to probe process state: %w", err)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("process %d did not exit in time", pid)
		}
		time.Sleep(300 * time.Millisecond)
	}
}
