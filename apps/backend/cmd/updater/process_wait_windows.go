//go:build windows

package main

import "time"

func waitForProcessExit(_ int, timeout time.Duration) error {
	// Windows에서는 실행 중 바이너리 rename이 실패하므로,
	// replaceWithRetry의 rename 재시도 루프가 실제 종료 타이밍을 흡수한다.
	time.Sleep(500 * time.Millisecond)
	_ = timeout
	return nil
}
