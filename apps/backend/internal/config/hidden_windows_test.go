//go:build windows

package config

import (
	"path/filepath"
	"syscall"
	"testing"
)

func TestEnsureProductionHomeDir_SetsHiddenAttributeOnWindows(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("USERPROFILE", homeDir)
	t.Setenv("HOME", homeDir)

	appDir, err := EnsureProductionHomeDir()
	if err != nil {
		t.Fatalf("ensure production home dir: %v", err)
	}

	pathPtr, err := syscall.UTF16PtrFromString(filepath.Clean(appDir))
	if err != nil {
		t.Fatalf("utf16 ptr: %v", err)
	}

	attrs, err := syscall.GetFileAttributes(pathPtr)
	if err != nil {
		t.Fatalf("get file attributes: %v", err)
	}
	if attrs&syscall.FILE_ATTRIBUTE_HIDDEN == 0 {
		t.Fatalf("expected hidden attribute for %q, attrs=%d", appDir, attrs)
	}
}
