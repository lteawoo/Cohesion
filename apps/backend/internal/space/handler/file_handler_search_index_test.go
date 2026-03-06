package handler

import (
	"context"
	"testing"

	"taeu.kr/cohesion/internal/space"
)

type fakeSearchIndexService struct {
	markedSpaces []int64
	markAllCalls int
}

func (f *fakeSearchIndexService) Bootstrap(context.Context) error {
	return nil
}

func (f *fakeSearchIndexService) Search(context.Context, []int64, string) ([]space.SearchIndexResult, error) {
	return []space.SearchIndexResult{}, nil
}

func (f *fakeSearchIndexService) MarkSpaceDirty(_ context.Context, spaceID int64) error {
	f.markedSpaces = append(f.markedSpaces, spaceID)
	return nil
}

func (f *fakeSearchIndexService) MarkAllDirty(context.Context) error {
	f.markAllCalls++
	return nil
}

func TestMarkSearchIndexDirty_MarksTrashAction(t *testing.T) {
	indexer := &fakeSearchIndexService{}
	handler := &Handler{
		searchIndexer: indexer,
	}

	handler.markSearchIndexDirty(context.Background(), 7, "trash")

	if len(indexer.markedSpaces) != 1 {
		t.Fatalf("expected one dirty mark call, got %d", len(indexer.markedSpaces))
	}
	if indexer.markedSpaces[0] != 7 {
		t.Fatalf("expected space 7 to be marked dirty, got %d", indexer.markedSpaces[0])
	}
	if indexer.markAllCalls != 0 {
		t.Fatalf("expected mark-all not to be called, got %d", indexer.markAllCalls)
	}
}
