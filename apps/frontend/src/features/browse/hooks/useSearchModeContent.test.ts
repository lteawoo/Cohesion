import { renderHook } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { useSearchModeContent } from './useSearchModeContent';

const h = vi.hoisted(() => {
  const openResult = vi.fn();
  const searchSource = {
    query: 'report',
    results: [
      {
        spaceId: 7,
        spaceName: 'Workspace',
        name: 'report.txt',
        path: 'docs/report.txt',
        parentPath: 'docs',
        isDir: false,
        size: 128,
        modTime: '2026-03-06T12:00:00Z',
      },
    ],
    errorMessage: null,
    isSearching: false,
    resultCount: 1,
    currentLimit: 80,
    hasMore: false,
    hasEnoughQuery: true,
    canLoadMore: false,
    loadMore: vi.fn(),
    openResult,
  };

  return {
    openResult,
    searchSource,
  };
});

vi.mock('@/features/search/hooks/useSearchExplorerSource', () => ({
  useSearchExplorerSource: () => h.searchSource,
}));

vi.mock('@/features/search/utils/highlightQueryMatch', () => ({
  highlightQueryMatch: (value: string) => value,
}));

describe('useSearchModeContent', () => {
  it('includes space and parent path in search result meta', () => {
    const { result } = renderHook(() => useSearchModeContent({
      isSearchMode: true,
      browseContent: [],
      browseErrorMessage: null,
      browseLoading: false,
    }));

    expect(result.current.sourceContent).toHaveLength(1);
    const record = result.current.sourceContent[0];
    expect(String(result.current.renderSearchMeta(record))).toContain('Workspace / docs');
  });

  it('omits date from grid meta while keeping space and parent path', () => {
    const { result } = renderHook(() => useSearchModeContent({
      isSearchMode: true,
      browseContent: [],
      browseErrorMessage: null,
      browseLoading: false,
    }));

    const record = result.current.sourceContent[0];
    const gridMeta = String(result.current.renderSearchGridMeta(record));
    expect(gridMeta).toContain('Workspace / docs');
    expect(gridMeta).not.toContain('2026');
  });

  it('opens the mapped search result by record path', () => {
    const { result } = renderHook(() => useSearchModeContent({
      isSearchMode: true,
      browseContent: [],
      browseErrorMessage: null,
      browseLoading: false,
    }));

    result.current.openSearchResultByRecordPath('7::docs/report.txt');

    expect(h.openResult).toHaveBeenCalledWith(h.searchSource.results[0]);
  });
});
