package space

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Evaluating the existing prefix also covers temporarily disconnected space roots.
func canonicalStoragePath(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(path)
	if !errors.Is(err, os.ErrNotExist) {
		return resolved, err
	}
	parent := filepath.Dir(path)
	if parent == path {
		return "", err
	}
	resolved, err = canonicalStoragePath(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolved, filepath.Base(path)), nil
}

func (s *Service) SetDownloadStorage(path string) { s.downloadStorage = path }

func (s *Service) OverlapsDownloadStorage(path string) (bool, error) {
	if s.downloadStorage == "" {
		return false, nil
	}
	a, err := canonicalStoragePath(path)
	if err != nil {
		return false, err
	}
	b := s.downloadStorage
	for _, pair := range [][2]string{{a, b}, {b, a}} {
		rel, err := filepath.Rel(pair[0], pair[1])
		if err == nil && (rel == "." || filepath.IsLocal(rel)) {
			return true, nil
		}
		// Case-insensitive volumes may preserve different spellings of the same ancestor.
		if strings.EqualFold(filepath.VolumeName(pair[0]), filepath.VolumeName(pair[1])) {
			info, err := os.Stat(pair[0])
			if err != nil {
				continue
			}
			for dir := pair[1]; ; dir = filepath.Dir(dir) {
				candidate, err := os.Stat(dir)
				if err == nil && os.SameFile(info, candidate) {
					return true, nil
				}
				if filepath.Dir(dir) == dir {
					break
				}
			}
		}
	}
	return false, nil
}
