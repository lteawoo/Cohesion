package system

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBuildArchiveNameCandidates(t *testing.T) {
	candidates := buildArchiveNameCandidates("v0.4.0", "darwin", "arm64")
	if len(candidates) != 4 {
		t.Fatalf("expected 4 candidates, got %d", len(candidates))
	}
	if candidates[0] != "cohesion_v0.4.0_apple_darwin_arm64.tar.gz" {
		t.Fatalf("unexpected first candidate: %s", candidates[0])
	}
	if candidates[1] != "cohesion_v0.4.0_darwin_arm64.tar.gz" {
		t.Fatalf("unexpected second candidate: %s", candidates[1])
	}
	if candidates[2] != "cohesion_0.4.0_apple_darwin_arm64.tar.gz" {
		t.Fatalf("unexpected third candidate: %s", candidates[2])
	}
	if candidates[3] != "cohesion_0.4.0_darwin_arm64.tar.gz" {
		t.Fatalf("unexpected fourth candidate: %s", candidates[3])
	}
}

func TestPickArchiveAsset(t *testing.T) {
	assets := []githubReleaseAsset{
		{Name: "checksums.txt", BrowserDownloadURL: "https://example/checksums.txt"},
		{Name: "cohesion_0.4.0_apple_darwin_arm64.tar.gz", BrowserDownloadURL: "https://example/archive.tar.gz"},
	}

	name, url, err := pickArchiveAsset(assets, "v0.4.0", "darwin", "arm64")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if name != "cohesion_0.4.0_apple_darwin_arm64.tar.gz" {
		t.Fatalf("unexpected archive name: %s", name)
	}
	if url != "https://example/archive.tar.gz" {
		t.Fatalf("unexpected archive url: %s", url)
	}
}

func TestBuildArchiveNameCandidatesKeepsLinuxNaming(t *testing.T) {
	candidates := buildArchiveNameCandidates("v0.4.0", "linux", "amd64")
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}
	if candidates[0] != "cohesion_v0.4.0_linux_amd64.tar.gz" {
		t.Fatalf("unexpected first linux candidate: %s", candidates[0])
	}
	if candidates[1] != "cohesion_0.4.0_linux_amd64.tar.gz" {
		t.Fatalf("unexpected second linux candidate: %s", candidates[1])
	}
}

func TestFindChecksumForAsset(t *testing.T) {
	tmpDir := t.TempDir()
	checksumFile := filepath.Join(tmpDir, "checksums.txt")
	content := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  cohesion_v0.4.0_darwin_amd64.tar.gz\n"
	if err := os.WriteFile(checksumFile, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write checksum file: %v", err)
	}

	checksum, err := findChecksumForAsset(checksumFile, "cohesion_v0.4.0_darwin_amd64.tar.gz")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if checksum != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("unexpected checksum: %s", checksum)
	}
}

func TestBuildLocalHealthURL(t *testing.T) {
	url, err := buildLocalHealthURL("3000")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if url != "http://127.0.0.1:3000/api/health" {
		t.Fatalf("unexpected health url: %s", url)
	}
}

func TestBuildLocalHealthURLRejectsInvalidPort(t *testing.T) {
	if _, err := buildLocalHealthURL("invalid"); err == nil {
		t.Fatal("expected invalid port error")
	}
}

func TestBuildLocalVersionURL(t *testing.T) {
	url, err := buildLocalVersionURL("3000")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if url != "http://127.0.0.1:3000/api/system/version" {
		t.Fatalf("unexpected version url: %s", url)
	}
}

func TestBuildLocalVersionURLRejectsInvalidPort(t *testing.T) {
	if _, err := buildLocalVersionURL("invalid"); err == nil {
		t.Fatal("expected invalid port error")
	}
}

func TestConfigureUpdaterCommandIOInheritsTerminalWhenInteractive(t *testing.T) {
	cmd := exec.Command("echo")

	configureUpdaterCommandIO(cmd, LaunchModeInteractive)

	if cmd.Stdout != os.Stdout {
		t.Fatal("expected updater stdout to inherit current stdout")
	}
	if cmd.Stderr != os.Stderr {
		t.Fatal("expected updater stderr to inherit current stderr")
	}
}

func TestConfigureUpdaterCommandIOKeepsBackgroundDefault(t *testing.T) {
	cmd := exec.Command("echo")

	configureUpdaterCommandIO(cmd, LaunchModeBackground)

	if cmd.Stdout != nil {
		t.Fatal("expected background updater stdout to remain unset")
	}
	if cmd.Stderr != nil {
		t.Fatal("expected background updater stderr to remain unset")
	}
}
