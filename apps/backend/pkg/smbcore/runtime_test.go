package smbcore

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigValidateDialectBounds(t *testing.T) {
	if _, err := NewEngine(Config{
		MinDialect:   Dialect210,
		MaxDialect:   Dialect311,
		RolloutPhase: RolloutPhaseReadOnly,
	}, nil, nil, nil, nil); err != nil {
		t.Fatalf("expected valid dialect bounds: %v", err)
	}

	if _, err := NewEngine(Config{
		MinDialect:   Dialect311,
		MaxDialect:   Dialect210,
		RolloutPhase: RolloutPhaseReadOnly,
	}, nil, nil, nil, nil); err == nil {
		t.Fatal("expected invalid dialect bounds error")
	}
}

func TestConfigValidateRolloutPhase(t *testing.T) {
	if _, err := NewEngine(Config{
		MinDialect:   Dialect210,
		MaxDialect:   Dialect311,
		RolloutPhase: RolloutPhaseWriteSafe,
	}, nil, nil, nil, nil); err != nil {
		t.Fatalf("expected valid rollout phase: %v", err)
	}

	if _, err := NewEngine(Config{
		MinDialect:   Dialect210,
		MaxDialect:   Dialect311,
		RolloutPhase: "invalid-phase",
	}, nil, nil, nil, nil); err == nil {
		t.Fatal("expected invalid rollout phase error")
	}
}

func TestNoExternalSMBServerRuntimeDependency(t *testing.T) {
	goModPath := findGoMod(t)
	data, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}

	content := string(data)
	disallowed := []string{
		"github.com/macos-fuse-t/go-smb2",
		"github.com/hirochachacha/go-smb2",
		"github.com/CloudSoda/go-smb2",
		"github.com/stacktitan/smb",
	}
	for _, dependency := range disallowed {
		if strings.Contains(content, dependency) {
			t.Fatalf("unexpected external smb server runtime dependency found: %s", dependency)
		}
	}
}

func TestEngineCheckUsability(t *testing.T) {
	engine, err := NewEngine(Config{
		MinDialect:   Dialect210,
		MaxDialect:   Dialect311,
		RolloutPhase: RolloutPhaseReadOnly,
	}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	if err := engine.CheckUsability(context.Background()); err == nil {
		t.Fatal("expected usability check failure when dependencies are missing")
	}
}

func findGoMod(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	for {
		candidate := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found from current working directory")
		}
		dir = parent
	}
}
