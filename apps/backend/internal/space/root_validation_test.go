package space

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestValidateSpaceRootReturnsValidForReadableDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	result, err := ValidateSpaceRoot(root)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected root to be valid, got %#v", result)
	}
	if result.Code != SpaceRootValidationCodeValid {
		t.Fatalf("expected %q, got %q", SpaceRootValidationCodeValid, result.Code)
	}
}

func TestValidateSpaceRootReturnsNotFoundForMissingDirectory(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "missing")
	result, err := ValidateSpaceRoot(root)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Valid {
		t.Fatalf("expected missing root to be invalid, got %#v", result)
	}
	if result.Code != SpaceRootValidationCodeNotFound {
		t.Fatalf("expected %q, got %q", SpaceRootValidationCodeNotFound, result.Code)
	}
}

func TestValidateSpaceRootReturnsNotDirectoryForFile(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(root, []byte("hello"), 0o644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	result, err := ValidateSpaceRoot(root)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Valid {
		t.Fatalf("expected file path to be invalid, got %#v", result)
	}
	if result.Code != SpaceRootValidationCodeNotDirectory {
		t.Fatalf("expected %q, got %q", SpaceRootValidationCodeNotDirectory, result.Code)
	}
}

func TestValidateSpaceRootReturnsPermissionDeniedForUnreadableDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission-denied directory semantics differ on Windows")
	}

	root := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "child.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatalf("failed to create child file: %v", err)
	}
	if err := os.Chmod(root, 0o000); err != nil {
		t.Fatalf("failed to remove directory permissions: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(root, 0o755)
	})

	result, err := ValidateSpaceRoot(root)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Valid {
		t.Fatalf("expected unreadable root to be invalid, got %#v", result)
	}
	if result.Code != SpaceRootValidationCodePermissionDenied {
		t.Fatalf("expected %q, got %q", SpaceRootValidationCodePermissionDenied, result.Code)
	}
}
