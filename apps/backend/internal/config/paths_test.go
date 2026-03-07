package config

import (
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
