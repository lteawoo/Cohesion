package handler

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path"
	"strings"
	"unicode"

	"taeu.kr/cohesion/internal/platform/web"
	store "taeu.kr/cohesion/internal/space/store"
)

type downloadSource struct {
	file *os.File
	info os.FileInfo
	item store.DownloadLinkItem
}

type downloadSelection struct {
	files []downloadSource
	size  int64
}

func openDownloadSelection(root string, paths []string) (*downloadSelection, error) {
	selection := &downloadSelection{}
	seen := make(map[string]bool)
	names := make(map[string]bool)
	for _, sourcePath := range paths {
		if seen[sourcePath] {
			selection.close()
			return nil, os.ErrInvalid
		}
		seen[sourcePath] = true
		file, info, err := openShareSource(root, sourcePath)
		if err != nil {
			selection.close()
			return nil, err
		}
		name := path.Base(sourcePath)
		if len(paths) > 1 {
			name = bundleEntryName(name, names)
		}
		item := store.DownloadLinkItem{ID: len(selection.files) + 1, Name: name, Size: info.Size()}
		selection.files = append(selection.files, downloadSource{file: file, info: info, item: item})
		if info.Size() > math.MaxInt64-selection.size {
			selection.close()
			return nil, os.ErrInvalid
		}
		selection.size += info.Size()
	}
	return selection, nil
}

func (s *downloadSelection) close() {
	for _, source := range s.files {
		source.file.Close()
	}
}

func (s *downloadSelection) copy(ctx context.Context, dest *os.File) error {
	for _, source := range s.files {
		if err := copyDownloadSnapshot(ctx, source.file, dest, source.info); err != nil {
			return err
		}
	}
	return dest.Sync()
}

func bundleEntryName(original string, used map[string]bool) string {
	name := strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) || unicode.IsControl(r) {
			return '_'
		}
		return r
	}, original)
	name = strings.TrimRight(name, ". ")
	if name == "" {
		name = "file"
	}
	suffix := path.Ext(name)
	stem := strings.TrimSuffix(name, suffix)
	if stem == "" {
		stem = name
		suffix = ""
	}
	candidate := name
	for i := 2; used[strings.ToLower(candidate)]; i++ {
		candidate = fmt.Sprintf("%s (%d)%s", stem, i, suffix)
	}
	used[strings.ToLower(candidate)] = true
	return candidate
}

func streamDownloadBundle(w http.ResponseWriter, r *http.Request, file *os.File, items []store.DownloadLinkItem) *web.Error {
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="cohesion-files.zip"`)
	archive := zip.NewWriter(w)
	for _, item := range items {
		entry, err := archive.CreateHeader(&zip.FileHeader{Name: item.Name, Method: zip.Store})
		if err != nil {
			panic(http.ErrAbortHandler)
		}
		if _, err = io.CopyN(entry, snapshotReader{r.Context(), file}, item.Size); err != nil {
			// An interrupted HTTP stream must not look like a complete ZIP.
			panic(http.ErrAbortHandler)
		}
	}
	if err := archive.Close(); err != nil {
		panic(http.ErrAbortHandler)
	}
	return nil
}
