package handler

import (
	"context"
	"errors"
	"io"

	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	store "taeu.kr/cohesion/internal/space/store"
)

type snapshotReservation struct{ spaceID, size int64 }
type downloadSnapshots struct {
	dir     string
	mu      sync.Mutex
	pending map[string]snapshotReservation
	slots   chan struct{}
}

func newDownloadSnapshots(dir string) (*downloadSnapshots, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("snapshot storage must be a directory")
	}
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, err
	}
	return &downloadSnapshots{dir: dir, pending: map[string]snapshotReservation{}, slots: make(chan struct{}, 2)}, nil
}

func (s *downloadSnapshots) filename(id string) string { return filepath.Join(s.dir, id+".snapshot") }

var errSnapshotChanged = errors.New("source changed during snapshot")

var errSnapshotQuota = errors.New("download snapshot quota exceeded")

func (s *downloadSnapshots) reserve(ctx context.Context, db *store.TransferStore, link store.DownloadLink, quota *int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if quota != nil {
		used, err := db.DownloadSnapshotUsage(ctx, link.SpaceID, time.Now().Unix())
		if err != nil {
			return err
		}
		remaining := *quota - used
		for _, r := range s.pending {
			if r.spaceID == link.SpaceID {
				remaining -= r.size
			}
		}
		if link.Size > remaining {
			return errSnapshotQuota
		}
	}
	s.pending[link.Snapshot] = snapshotReservation{spaceID: link.SpaceID, size: link.Size}
	return nil
}

func (s *downloadSnapshots) cleanup(ctx context.Context, db *store.TransferStore) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	live, err := db.LiveDownloadSnapshots(ctx, time.Now().Unix())
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name, ok := strings.CutSuffix(entry.Name(), ".snapshot")
		if !ok {
			continue
		}
		if _, pending := s.pending[name]; !pending && !live[name] && validTransferID(name) && !entry.IsDir() {
			if err := os.Remove(s.filename(name)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	return nil
}

// Compare each opened handle with its directory entry to exclude symlink aliases and swapped special files.
func openShareSource(root, path string) (*os.File, os.FileInfo, error) {
	if _, err := filepath.Localize(path); err != nil || path == "." || strings.ContainsAny(path, "\x00\r\n") {
		return nil, nil, os.ErrInvalid
	}
	dir, err := os.OpenRoot(root)
	if err != nil {
		return nil, nil, err
	}
	defer func() { dir.Close() }()
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if !filepath.IsLocal(part) || strings.EqualFold(part, spaceTrashDirectoryName) || strings.HasPrefix(strings.ToLower(part), uploadStagePrefix) {
			return nil, nil, os.ErrInvalid
		}
		before, err := dir.Lstat(part)
		if err != nil {
			return nil, nil, err
		}
		if i == len(parts)-1 {
			if !before.Mode().IsRegular() {
				return nil, nil, os.ErrInvalid
			}
			file, err := dir.OpenFile(part, os.O_RDONLY|syscall.O_NONBLOCK, 0)
			if err != nil {
				return nil, nil, err
			}
			after, err := file.Stat()
			if err != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) {
				file.Close()
				return nil, nil, os.ErrInvalid
			}
			return file, after, nil
		}
		if !before.IsDir() {
			return nil, nil, os.ErrInvalid
		}
		next, err := dir.OpenRoot(part)
		if err != nil {
			return nil, nil, err
		}
		after, err := next.Stat(".")
		if err != nil || !os.SameFile(before, after) {
			next.Close()
			return nil, nil, os.ErrInvalid
		}
		dir.Close()
		dir = next
	}
	return nil, nil, os.ErrInvalid
}

type snapshotReader struct {
	ctx  context.Context
	file *os.File
}

func (r snapshotReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.file.Read(p)
}

func copyDownloadSnapshot(ctx context.Context, src, dst *os.File, before os.FileInfo) error {
	if _, err := io.CopyN(dst, snapshotReader{ctx, src}, before.Size()); err != nil {
		if errors.Is(err, io.EOF) {
			return errSnapshotChanged
		}
		return err
	}
	after, err := src.Stat()
	if err != nil {
		return err
	}
	if after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
		return errSnapshotChanged
	}
	return dst.Sync()
}
