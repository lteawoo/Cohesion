package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveProductionHomeDirForOS_LinuxUsesVarLibRoot(t *testing.T) {
	appDir, err := resolveProductionHomeDirForOS("linux")
	if err != nil {
		t.Fatalf("resolve production home dir for linux: %v", err)
	}

	if appDir != "/var/lib/cohesion" {
		t.Fatalf("expected %q, got %q", "/var/lib/cohesion", appDir)
	}
}

func TestResolveProductionHomeDirForOS_UsesUserHomeOutsideLinux(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	appDir, err := resolveProductionHomeDirForOS("darwin")
	if err != nil {
		t.Fatalf("resolve production home dir for darwin: %v", err)
	}

	expected := filepath.Join(homeDir, ".cohesion")
	if appDir != expected {
		t.Fatalf("expected %q, got %q", expected, appDir)
	}
}

func TestResolveProductionHomeDir_UsesStateRootOverride(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(ProductionStateRootEnv, stateRoot)

	appDir, err := ResolveProductionHomeDir()
	if err != nil {
		t.Fatalf("resolve production home dir: %v", err)
	}

	if appDir != stateRoot {
		t.Fatalf("expected %q, got %q", stateRoot, appDir)
	}
}

func TestEnsureProductionHomeDir_UsesStateRootOverride(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".cohesion")
	t.Setenv(ProductionStateRootEnv, stateRoot)

	appDir, err := EnsureProductionHomeDir()
	if err != nil {
		t.Fatalf("ensure production home dir: %v", err)
	}

	if appDir != stateRoot {
		t.Fatalf("expected %q, got %q", stateRoot, appDir)
	}
	if info, err := os.Stat(appDir); err != nil {
		t.Fatalf("stat production home dir: %v", err)
	} else if !info.IsDir() {
		t.Fatalf("expected %q to be a directory", appDir)
	}
}

func TestExpandHomePath(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	expanded, ok := ExpandHomePath("~/cohesion/config.yaml")
	if !ok {
		t.Fatal("expected path to be expanded")
	}

	expected := filepath.Join(homeDir, "cohesion", "config.yaml")
	if expanded != expected {
		t.Fatalf("expected %q, got %q", expected, expanded)
	}
}
