package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveProductionConfigDir(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	configDir, err := ResolveProductionConfigDir()
	if err != nil {
		t.Fatalf("resolve production config dir: %v", err)
	}

	expected := filepath.Join(homeDir, ".cohesion", "config")
	if configDir != expected {
		t.Fatalf("expected %q, got %q", expected, configDir)
	}
}

func TestEnsureProductionHomeDir(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	appDir, err := EnsureProductionHomeDir()
	if err != nil {
		t.Fatalf("ensure production home dir: %v", err)
	}

	expected := filepath.Join(homeDir, ".cohesion")
	if appDir != expected {
		t.Fatalf("expected %q, got %q", expected, appDir)
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
