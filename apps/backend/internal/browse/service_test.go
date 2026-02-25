package browse

import (
	"path/filepath"
	"testing"
)

func TestNormalizeBrowsePathWindowsDriveRoot(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"C:":    `C:\`,
		"c:":    `C:\`,
		"C:\\":  `C:\`,
		"C:/":   `C:\`,
		"C:.":   `C:\`,
		"C:\\.": `C:\`,
	}

	for input, expected := range tests {
		input := input
		expected := expected
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			if actual := normalizeBrowsePathForOS(input, "windows"); actual != expected {
				t.Fatalf("normalizeBrowsePath(%q) = %q, want %q", input, actual, expected)
			}
		})
	}
}

func TestNormalizeBrowsePathWindowsDriveRelative(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		`C:Users`:        `C:\Users`,
		`c:Users\Public`: `C:\Users\Public`,
		`D:temp\..\logs`: `D:\logs`,
		`e:.\downloads`:  `E:\downloads`,
		`F:..\workspace`: `F:\workspace`,
	}

	for input, expected := range tests {
		input := input
		expected := expected
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			if actual := normalizeBrowsePathForOS(input, "windows"); actual != expected {
				t.Fatalf("normalizeBrowsePath(%q) = %q, want %q", input, actual, expected)
			}
		})
	}
}

func TestNormalizeBrowsePathWindowsUNC(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		`\\server\share`:            `\\server\share`,
		`\\server\share\folder`:     `\\server\share\folder`,
		`\\server\share\foo\..\bar`: `\\server\share\bar`,
		`//server/share/a/./b/../c`: `\\server\share\a\c`,
	}

	for input, expected := range tests {
		input := input
		expected := expected
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			if actual := normalizeBrowsePathForOS(input, "windows"); actual != expected {
				t.Fatalf("normalizeBrowsePath(%q) = %q, want %q", input, actual, expected)
			}
		})
	}
}

func TestNormalizeBrowsePathForNonWindows(t *testing.T) {
	t.Parallel()

	input := "/tmp/../var//log"
	expected := "/var/log"
	if actual := normalizeBrowsePathForOS(input, "linux"); actual != expected {
		t.Fatalf("normalizeBrowsePathForOS(%q, linux) = %q, want %q", input, actual, expected)
	}
}

func TestNormalizeBrowsePathForNonWindowsPreservesWhitespace(t *testing.T) {
	t.Parallel()

	input := " /tmp/project "
	expected := filepath.Clean(input)
	if actual := normalizeBrowsePathForOS(input, "linux"); actual != expected {
		t.Fatalf("normalizeBrowsePathForOS(%q, linux) = %q, want %q", input, actual, expected)
	}
}

func TestMountpointDedupKeyByOS(t *testing.T) {
	t.Parallel()

	if actual := mountpointDedupKey(`C:\`, "windows"); actual != `c:\` {
		t.Fatalf("mountpointDedupKey windows = %q, want %q", actual, `c:\`)
	}
	if actual := mountpointDedupKey("/Volumes/Data", "darwin"); actual != "/Volumes/Data" {
		t.Fatalf("mountpointDedupKey darwin = %q, want %q", actual, "/Volumes/Data")
	}
}
