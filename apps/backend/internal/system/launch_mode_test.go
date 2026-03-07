package system

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectLaunchModeUsesOverride(t *testing.T) {
	t.Setenv(launchModeEnv, string(LaunchModeInteractive))

	if got := DetectLaunchMode(); got != LaunchModeInteractive {
		t.Fatalf("expected interactive launch mode, got %q", got)
	}
}

func TestParseLaunchModeFallsBackToBackground(t *testing.T) {
	if got := ParseLaunchMode("invalid"); got != LaunchModeBackground {
		t.Fatalf("expected background fallback, got %q", got)
	}
}

func TestDetectLaunchModeForFilesTreatsRegularFilesAsBackground(t *testing.T) {
	tempFile, err := os.Create(filepath.Join(t.TempDir(), "stdout.log"))
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer tempFile.Close()

	if got := detectLaunchModeForFiles(tempFile, tempFile); got != LaunchModeBackground {
		t.Fatalf("expected background launch mode, got %q", got)
	}
}
