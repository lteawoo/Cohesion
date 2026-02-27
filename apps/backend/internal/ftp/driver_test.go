package ftp

import (
	"errors"
	"os"
	"testing"
)

func TestNormalizeVirtualPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "space root", input: "space-a", want: "/space-a"},
		{name: "nested path", input: "/space-a/folder/file.txt", want: "/space-a/folder/file.txt"},
		{name: "windows separators", input: `\\space-a\\folder\\file.txt`, want: "/space-a/folder/file.txt"},
		{name: "clean traversal", input: "space-a/../space-b", want: "/space-b"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := normalizeVirtualPath(tt.input)
			if got != tt.want {
				t.Fatalf("normalizeVirtualPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSplitVirtualPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		wantSpace   string
		wantRelPath string
		wantErr     error
	}{
		{name: "space root", input: "/space-a", wantSpace: "space-a", wantRelPath: ""},
		{name: "space child", input: "/space-a/folder/file.txt", wantSpace: "space-a", wantRelPath: "folder/file.txt"},
		{name: "reject slash root", input: "/", wantErr: os.ErrPermission},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			spaceName, relPath, err := splitVirtualPath(tt.input)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("splitVirtualPath(%q) error = %v, want %v", tt.input, err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("splitVirtualPath(%q) unexpected error: %v", tt.input, err)
			}
			if spaceName != tt.wantSpace || relPath != tt.wantRelPath {
				t.Fatalf("splitVirtualPath(%q) = (%q, %q), want (%q, %q)", tt.input, spaceName, relPath, tt.wantSpace, tt.wantRelPath)
			}
		})
	}
}

func TestIsPathWithinSpace(t *testing.T) {
	t.Parallel()

	spacePath := "/tmp/cohesion-space"
	tests := []struct {
		name  string
		path  string
		allow bool
	}{
		{name: "space root", path: "/tmp/cohesion-space", allow: true},
		{name: "space child", path: "/tmp/cohesion-space/folder/file.txt", allow: true},
		{name: "sibling path", path: "/tmp/cohesion-space-other/file.txt", allow: false},
		{name: "escape path", path: "/tmp/cohesion-space/../outside.txt", allow: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := isPathWithinSpace(tt.path, spacePath)
			if got != tt.allow {
				t.Fatalf("isPathWithinSpace(%q, %q) = %v, want %v", tt.path, spacePath, got, tt.allow)
			}
		})
	}
}
