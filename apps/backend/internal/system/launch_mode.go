package system

import (
	"os"
	"strings"

	"github.com/mattn/go-isatty"
)

const launchModeEnv = "COHESION_LAUNCH_MODE"

type LaunchMode string

const (
	LaunchModeBackground  LaunchMode = "background"
	LaunchModeInteractive LaunchMode = "interactive"
)

func DetectLaunchMode() LaunchMode {
	if mode, ok := parseLaunchMode(os.Getenv(launchModeEnv)); ok {
		return mode
	}
	return detectLaunchModeForFiles(os.Stdout, os.Stderr)
}

func ParseLaunchMode(raw string) LaunchMode {
	if mode, ok := parseLaunchMode(raw); ok {
		return mode
	}
	return LaunchModeBackground
}

func (m LaunchMode) String() string {
	if m == LaunchModeInteractive {
		return string(LaunchModeInteractive)
	}
	return string(LaunchModeBackground)
}

func (m LaunchMode) IsInteractive() bool {
	return m == LaunchModeInteractive
}

func parseLaunchMode(raw string) (LaunchMode, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(LaunchModeInteractive):
		return LaunchModeInteractive, true
	case string(LaunchModeBackground):
		return LaunchModeBackground, true
	default:
		return "", false
	}
}

func detectLaunchModeForFiles(stdout, stderr *os.File) LaunchMode {
	if isTerminalFile(stdout) || isTerminalFile(stderr) {
		return LaunchModeInteractive
	}
	return LaunchModeBackground
}

func isTerminalFile(file *os.File) bool {
	if file == nil {
		return false
	}
	fd := file.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}
