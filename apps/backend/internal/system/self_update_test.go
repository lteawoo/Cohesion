package system

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildArchiveNameCandidates(t *testing.T) {
	candidates := buildArchiveNameCandidates("v0.4.0", "darwin", "arm64")
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}
	if candidates[0] != "cohesion_v0.4.0_darwin_arm64.tar.gz" {
		t.Fatalf("unexpected first candidate: %s", candidates[0])
	}
	if candidates[1] != "cohesion_0.4.0_darwin_arm64.tar.gz" {
		t.Fatalf("unexpected second candidate: %s", candidates[1])
	}
}

func TestPickArchiveAsset(t *testing.T) {
	assets := []githubReleaseAsset{
		{Name: "checksums.txt", BrowserDownloadURL: "https://example/checksums.txt"},
		{Name: "cohesion_0.4.0_darwin_arm64.tar.gz", BrowserDownloadURL: "https://example/archive.tar.gz"},
	}

	name, url, err := pickArchiveAsset(assets, "v0.4.0", "darwin", "arm64")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if name != "cohesion_0.4.0_darwin_arm64.tar.gz" {
		t.Fatalf("unexpected archive name: %s", name)
	}
	if url != "https://example/archive.tar.gz" {
		t.Fatalf("unexpected archive url: %s", url)
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
