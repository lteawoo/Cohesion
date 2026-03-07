package sftp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"taeu.kr/cohesion/internal/config"
)

func TestPrewarmHostKey_SourceGeneratedThenEnvWithEnvPathOverride(t *testing.T) {
	hostKeyPath := filepath.Join(t.TempDir(), "sftp_host_key")
	t.Setenv("COHESION_SFTP_HOST_KEY_FILE", hostKeyPath)

	first, err := PrewarmHostKey()
	if err != nil {
		t.Fatalf("first prewarm host key: %v", err)
	}
	if first.Source != "generated" {
		t.Fatalf("expected generated source on first prewarm, got %q", first.Source)
	}
	if first.Path != hostKeyPath {
		t.Fatalf("expected host key path %q, got %q", hostKeyPath, first.Path)
	}

	second, err := PrewarmHostKey()
	if err != nil {
		t.Fatalf("second prewarm host key: %v", err)
	}
	if second.Source != "env" {
		t.Fatalf("expected env source on second prewarm with env path override, got %q", second.Source)
	}
	if second.Path != hostKeyPath {
		t.Fatalf("expected host key path %q, got %q", hostKeyPath, second.Path)
	}

	content, err := os.ReadFile(hostKeyPath)
	if err != nil {
		t.Fatalf("read generated host key: %v", err)
	}
	if strings.TrimSpace(string(content)) == "" {
		t.Fatalf("expected non-empty host key file, got %q", string(content))
	}
}

func TestResolveHostKeyPath_UsesStateRootSecretsDirForProductionConfig(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(config.ProductionStateRootEnv, stateRoot)
	t.Setenv("COHESION_SFTP_HOST_KEY_FILE", "")
	config.SetConfig("production")

	hostKeyPath, err := resolveHostKeyPath()
	if err != nil {
		t.Fatalf("resolve host key path: %v", err)
	}

	expected := filepath.Join(stateRoot, "secrets", defaultSFTPHostKeyName)
	if hostKeyPath != expected {
		t.Fatalf("expected %q, got %q", expected, hostKeyPath)
	}
}
