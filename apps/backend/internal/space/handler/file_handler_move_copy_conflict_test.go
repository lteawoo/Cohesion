package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"taeu.kr/cohesion/internal/account"
	"taeu.kr/cohesion/internal/auth"
	"taeu.kr/cohesion/internal/platform/web"
	"taeu.kr/cohesion/internal/space"
)

type allowAllSpaceAccessService struct{}

func (s *allowAllSpaceAccessService) CanAccessSpaceByID(_ context.Context, _ string, _ int64, _ account.Permission) (bool, error) {
	return true, nil
}

type transferResponsePayload struct {
	Succeeded []string `json:"succeeded"`
	Skipped   []string `json:"skipped"`
	Failed    []struct {
		Path   string `json:"path"`
		Reason string `json:"reason"`
		Code   string `json:"code"`
	} `json:"failed"`
}

func newTransferRequest(
	t *testing.T,
	op string,
	conflictPolicy string,
	spaceID int64,
	source string,
	destination string,
) *http.Request {
	t.Helper()

	payload := map[string]interface{}{
		"sources": []string{source},
		"destination": map[string]interface{}{
			"spaceId": spaceID,
			"path":    destination,
		},
	}
	if conflictPolicy != "" {
		payload["conflictPolicy"] = conflictPolicy
	}

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/spaces/1/files/"+op, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(auth.WithClaims(req.Context(), &auth.Claims{Username: "tester"}))
	return req
}

func decodeTransferResponse(t *testing.T, rec *httptest.ResponseRecorder) transferResponsePayload {
	t.Helper()

	var payload transferResponsePayload
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode transfer response: %v", err)
	}
	return payload
}

func TestHandleFileTransfer_ConflictPolicies(t *testing.T) {
	setup := func(t *testing.T) (*Handler, string) {
		t.Helper()

		spaceRoot := t.TempDir()
		store := &fakeUploadSpaceStore{
			spacesByID: map[int64]*space.Space{
				1: {
					ID:        1,
					SpaceName: "Transfer",
					SpacePath: spaceRoot,
				},
			},
		}

		handler := NewHandler(space.NewService(store), nil, &allowAllSpaceAccessService{})
		return handler, spaceRoot
	}

	writeFixture := func(t *testing.T, root string) {
		t.Helper()

		srcDir := filepath.Join(root, "src")
		dstDir := filepath.Join(root, "dst")
		if err := os.MkdirAll(srcDir, 0o755); err != nil {
			t.Fatalf("failed to create src dir: %v", err)
		}
		if err := os.MkdirAll(dstDir, 0o755); err != nil {
			t.Fatalf("failed to create dst dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("from-src"), 0o644); err != nil {
			t.Fatalf("failed to write source file: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dstDir, "a.txt"), []byte("from-dst"), 0o644); err != nil {
			t.Fatalf("failed to write destination file: %v", err)
		}
	}

	run := func(t *testing.T, op string, conflictPolicy string) transferResponsePayload {
		t.Helper()

		handler, root := setup(t)
		writeFixture(t, root)

		req := newTransferRequest(t, op, conflictPolicy, 1, "src/a.txt", "dst")
		rec := httptest.NewRecorder()
		var webErr *web.Error
		if op == "move" {
			webErr = handler.handleFileMove(rec, req, 1)
		} else {
			webErr = handler.handleFileCopy(rec, req, 1)
		}
		if webErr != nil {
			t.Fatalf("unexpected web error: %+v", webErr)
		}

		resp := decodeTransferResponse(t, rec)

		srcPath := filepath.Join(root, "src", "a.txt")
		dstPath := filepath.Join(root, "dst", "a.txt")
		renamedPath := filepath.Join(root, "dst", "a (1).txt")

		switch conflictPolicy {
		case "":
			if len(resp.Failed) != 1 || resp.Failed[0].Code != fileConflictCodeDestinationExists {
				t.Fatalf("expected destination_exists failure, got %+v", resp.Failed)
			}
			if len(resp.Succeeded) != 0 {
				t.Fatalf("expected no success, got %v", resp.Succeeded)
			}
			if len(resp.Skipped) != 0 {
				t.Fatalf("expected no skipped, got %v", resp.Skipped)
			}
			if _, err := os.Stat(srcPath); err != nil {
				t.Fatalf("source must remain on conflict: %v", err)
			}
			dstContent, err := os.ReadFile(dstPath)
			if err != nil {
				t.Fatalf("failed to read destination file: %v", err)
			}
			if string(dstContent) != "from-dst" {
				t.Fatalf("destination should remain original, got %q", string(dstContent))
			}
		case "overwrite":
			if len(resp.Failed) != 0 {
				t.Fatalf("expected no failure, got %+v", resp.Failed)
			}
			if len(resp.Succeeded) != 1 {
				t.Fatalf("expected one success, got %v", resp.Succeeded)
			}
			dstContent, err := os.ReadFile(dstPath)
			if err != nil {
				t.Fatalf("failed to read overwritten destination: %v", err)
			}
			if string(dstContent) != "from-src" {
				t.Fatalf("destination should be overwritten with source content, got %q", string(dstContent))
			}
			if op == "move" {
				if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
					t.Fatalf("source should be removed after move overwrite, err=%v", err)
				}
			} else {
				if _, err := os.Stat(srcPath); err != nil {
					t.Fatalf("source should remain after copy overwrite, err=%v", err)
				}
			}
		case "rename":
			if len(resp.Failed) != 0 {
				t.Fatalf("expected no failure, got %+v", resp.Failed)
			}
			if len(resp.Succeeded) != 1 {
				t.Fatalf("expected one success, got %v", resp.Succeeded)
			}
			renamedContent, err := os.ReadFile(renamedPath)
			if err != nil {
				t.Fatalf("failed to read renamed destination file: %v", err)
			}
			if string(renamedContent) != "from-src" {
				t.Fatalf("renamed destination should contain source content, got %q", string(renamedContent))
			}
			if op == "move" {
				if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
					t.Fatalf("source should be removed after move rename, err=%v", err)
				}
			} else {
				if _, err := os.Stat(srcPath); err != nil {
					t.Fatalf("source should remain after copy rename, err=%v", err)
				}
			}
		case "skip":
			if len(resp.Failed) != 0 {
				t.Fatalf("expected no failure, got %+v", resp.Failed)
			}
			if len(resp.Succeeded) != 0 {
				t.Fatalf("expected no success for skip, got %v", resp.Succeeded)
			}
			if len(resp.Skipped) != 1 || resp.Skipped[0] != "src/a.txt" {
				t.Fatalf("expected one skipped source, got %v", resp.Skipped)
			}
			if _, err := os.Stat(srcPath); err != nil {
				t.Fatalf("source should remain after skip, err=%v", err)
			}
			dstContent, err := os.ReadFile(dstPath)
			if err != nil {
				t.Fatalf("failed to read destination file after skip: %v", err)
			}
			if string(dstContent) != "from-dst" {
				t.Fatalf("destination should remain original after skip, got %q", string(dstContent))
			}
		}

		return resp
	}

	for _, op := range []string{"move", "copy"} {
		t.Run(op+"_conflict_without_policy", func(t *testing.T) {
			run(t, op, "")
		})
		t.Run(op+"_overwrite", func(t *testing.T) {
			run(t, op, "overwrite")
		})
		t.Run(op+"_rename", func(t *testing.T) {
			run(t, op, "rename")
		})
		t.Run(op+"_skip", func(t *testing.T) {
			run(t, op, "skip")
		})
	}
}

func TestHandleFileCopy_SameDestinationWithOverwriteDoesNotDeleteSource(t *testing.T) {
	spaceRoot := t.TempDir()
	store := &fakeUploadSpaceStore{
		spacesByID: map[int64]*space.Space{
			1: {
				ID:        1,
				SpaceName: "Transfer",
				SpacePath: spaceRoot,
			},
		},
	}

	handler := NewHandler(space.NewService(store), nil, &allowAllSpaceAccessService{})

	srcDir := filepath.Join(spaceRoot, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}

	srcPath := filepath.Join(srcDir, "a.txt")
	if err := os.WriteFile(srcPath, []byte("from-src"), 0o644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	req := newTransferRequest(t, "copy", "overwrite", 1, "src/a.txt", "src")
	rec := httptest.NewRecorder()
	webErr := handler.handleFileCopy(rec, req, 1)
	if webErr != nil {
		t.Fatalf("unexpected web error: %+v", webErr)
	}

	resp := decodeTransferResponse(t, rec)
	if len(resp.Succeeded) != 0 {
		t.Fatalf("expected no success, got %v", resp.Succeeded)
	}
	if len(resp.Skipped) != 0 {
		t.Fatalf("expected no skipped, got %v", resp.Skipped)
	}
	if len(resp.Failed) != 1 {
		t.Fatalf("expected one failure, got %+v", resp.Failed)
	}
	if resp.Failed[0].Code != fileConflictCodeSameDestination {
		t.Fatalf("expected same_destination code, got %+v", resp.Failed[0])
	}

	content, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("source file should remain after same-destination copy conflict: %v", err)
	}
	if string(content) != "from-src" {
		t.Fatalf("source content should remain unchanged, got %q", string(content))
	}
}

func TestMoveWithDestinationSwap_RestoresDestinationWhenMoveFails(t *testing.T) {
	root := t.TempDir()
	destPath := filepath.Join(root, "dest.txt")
	if err := os.WriteFile(destPath, []byte("original-destination"), 0o644); err != nil {
		t.Fatalf("failed to write destination fixture: %v", err)
	}

	missingSource := filepath.Join(root, "missing-source.txt")
	err := moveWithDestinationSwap(missingSource, destPath)
	if err == nil {
		t.Fatalf("expected moveWithDestinationSwap to fail for missing source")
	}

	content, readErr := os.ReadFile(destPath)
	if readErr != nil {
		t.Fatalf("destination should be restored after failed move overwrite: %v", readErr)
	}
	if string(content) != "original-destination" {
		t.Fatalf("destination content should remain unchanged, got %q", string(content))
	}

	backupMatches, globErr := filepath.Glob(filepath.Join(root, ".dest.txt.cohesion-bak-*"))
	if globErr != nil {
		t.Fatalf("failed to inspect backup artifacts: %v", globErr)
	}
	if len(backupMatches) != 0 {
		t.Fatalf("unexpected backup artifacts remain: %v", backupMatches)
	}
}
