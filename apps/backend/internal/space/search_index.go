package space

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type SearchIndexStorer interface {
	EnsureSpaceStates(ctx context.Context, spaceIDs []int64) error
	ListDirtySpaceIDs(ctx context.Context) ([]int64, error)
	ReplaceSpaceEntries(ctx context.Context, spaceID int64, entries []SearchIndexEntry) error
	SearchEntries(ctx context.Context, spaceIDs []int64, queryLower string) ([]SearchIndexResult, error)
	MarkSpaceDirty(ctx context.Context, spaceID int64) error
	MarkSpacesDirty(ctx context.Context, spaceIDs []int64) error
	RecordIndexFailure(ctx context.Context, spaceID int64, failure string) error
}

type SearchIndexManager struct {
	spaceService *Service
	store        SearchIndexStorer
	mu           sync.Mutex
}

func NewSearchIndexManager(spaceService *Service, store SearchIndexStorer) *SearchIndexManager {
	return &SearchIndexManager{
		spaceService: spaceService,
		store:        store,
	}
}

func (m *SearchIndexManager) Bootstrap(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ensureReadyLocked(ctx, nil, true)
}

func (m *SearchIndexManager) Search(ctx context.Context, spaceIDs []int64, queryLower string) ([]SearchIndexResult, error) {
	if len(spaceIDs) == 0 || strings.TrimSpace(queryLower) == "" {
		return []SearchIndexResult{}, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.store.EnsureSpaceStates(ctx, spaceIDs); err != nil {
		return nil, err
	}

	requested := make(map[int64]struct{}, len(spaceIDs))
	for _, spaceID := range spaceIDs {
		requested[spaceID] = struct{}{}
	}

	if err := m.ensureReadyLocked(ctx, requested, false); err != nil {
		return nil, err
	}

	return m.store.SearchEntries(ctx, spaceIDs, queryLower)
}

func (m *SearchIndexManager) MarkSpaceDirty(ctx context.Context, spaceID int64) error {
	return m.store.MarkSpaceDirty(ctx, spaceID)
}

func (m *SearchIndexManager) MarkAllDirty(ctx context.Context) error {
	spaces, err := m.spaceService.GetAllSpaces(ctx)
	if err != nil {
		return err
	}

	spaceIDs := make([]int64, 0, len(spaces))
	for _, item := range spaces {
		spaceIDs = append(spaceIDs, item.ID)
	}

	if err := m.store.EnsureSpaceStates(ctx, spaceIDs); err != nil {
		return err
	}
	return m.store.MarkSpacesDirty(ctx, spaceIDs)
}

func (m *SearchIndexManager) ensureReadyLocked(ctx context.Context, requested map[int64]struct{}, strict bool) error {
	spaces, err := m.spaceService.GetAllSpaces(ctx)
	if err != nil {
		return err
	}

	spaceIDs := make([]int64, 0, len(spaces))
	for _, item := range spaces {
		spaceIDs = append(spaceIDs, item.ID)
	}
	if err := m.store.EnsureSpaceStates(ctx, spaceIDs); err != nil {
		return err
	}

	dirtySpaceIDs, err := m.store.ListDirtySpaceIDs(ctx)
	if err != nil {
		return err
	}

	var firstErr error
	for _, spaceID := range dirtySpaceIDs {
		if len(requested) > 0 {
			if _, ok := requested[spaceID]; !ok {
				continue
			}
		}

		if err := m.reindexSpace(ctx, spaceID); err != nil {
			if recordErr := m.store.RecordIndexFailure(ctx, spaceID, err.Error()); recordErr != nil && firstErr == nil {
				firstErr = fmt.Errorf("reindex dirty space %d: %w (record failure: %v)", spaceID, err, recordErr)
			} else if strict && firstErr == nil {
				firstErr = fmt.Errorf("reindex dirty space %d: %w", spaceID, err)
			}
		}
	}

	if strict {
		return firstErr
	}
	return nil
}

func (m *SearchIndexManager) reindexSpace(ctx context.Context, spaceID int64) error {
	spaceData, err := m.spaceService.GetSpaceByID(ctx, spaceID)
	if err != nil {
		return err
	}

	entries, err := buildSearchIndexEntries(spaceData)
	if err != nil {
		return err
	}

	return m.store.ReplaceSpaceEntries(ctx, spaceID, entries)
}

func buildSearchIndexEntries(spaceData *Space) ([]SearchIndexEntry, error) {
	if spaceData == nil {
		return []SearchIndexEntry{}, nil
	}

	entries := []SearchIndexEntry{}
	err := filepath.WalkDir(spaceData.SpacePath, func(currentPath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsPermission(walkErr) {
				if entry != nil && entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			return walkErr
		}
		if entry == nil || currentPath == spaceData.SpacePath {
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		relativePath, err := filepath.Rel(spaceData.SpacePath, currentPath)
		if err != nil {
			return err
		}
		relativePath = filepath.ToSlash(relativePath)

		info, err := entry.Info()
		if err != nil {
			return nil
		}

		parentPath := filepath.ToSlash(filepath.Dir(relativePath))
		if parentPath == "." {
			parentPath = ""
		}

		entries = append(entries, SearchIndexEntry{
			SpaceID:    spaceData.ID,
			Name:       entry.Name(),
			Path:       relativePath,
			ParentPath: parentPath,
			IsDir:      entry.IsDir(),
			Size:       info.Size(),
			ModTime:    info.ModTime(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	return entries, nil
}
