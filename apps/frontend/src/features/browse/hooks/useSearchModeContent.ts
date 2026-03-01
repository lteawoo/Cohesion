import { useCallback, useMemo } from 'react';
import type { ReactNode } from 'react';
import { useSearchExplorerSource } from '@/features/search/hooks/useSearchExplorerSource';
import type { SearchFileResult } from '@/features/search/types';
import { highlightQueryMatch } from '@/features/search/utils/highlightQueryMatch';
import { formatDate, formatSize } from '../constants';
import type { FileNode } from '../types';

interface UseSearchModeContentParams {
  isSearchMode: boolean;
  browseContent: FileNode[];
  browseErrorMessage: string | null;
  browseLoading: boolean;
}

interface UseSearchModeContentResult {
  searchSource: ReturnType<typeof useSearchExplorerSource>;
  sourceContent: FileNode[];
  openSearchResultByRecordPath: (recordPath: string) => void;
  renderSearchName: (record: FileNode) => ReactNode;
  renderSearchMeta: (record: FileNode) => string;
  activeErrorMessage: string | null;
  activeLoading: boolean;
}

function resolveSearchRowPath(item: SearchFileResult): string {
  return `${item.spaceId}::${item.path}`;
}

export function useSearchModeContent({
  isSearchMode,
  browseContent,
  browseErrorMessage,
  browseLoading,
}: UseSearchModeContentParams): UseSearchModeContentResult {
  const searchSource = useSearchExplorerSource(isSearchMode);

  const searchItemsByRowPath = useMemo(() => {
    const next = new Map<string, SearchFileResult>();
    searchSource.results.forEach((item) => {
      next.set(resolveSearchRowPath(item), item);
    });
    return next;
  }, [searchSource.results]);

  const searchContent = useMemo<FileNode[]>(() => {
    return searchSource.results.map((item) => ({
      name: item.name,
      path: resolveSearchRowPath(item),
      isDir: item.isDir,
      modTime: item.modTime,
      size: item.size,
    }));
  }, [searchSource.results]);

  const sourceContent = isSearchMode ? searchContent : browseContent;

  const resolveSearchResult = useCallback((recordPath: string): SearchFileResult | null => {
    return searchItemsByRowPath.get(recordPath) ?? null;
  }, [searchItemsByRowPath]);

  const openSearchResultByRecordPath = useCallback((recordPath: string) => {
    const item = resolveSearchResult(recordPath);
    if (!item) {
      return;
    }
    searchSource.openResult(item);
  }, [resolveSearchResult, searchSource]);

  const renderSearchName = useCallback((record: FileNode) => {
    const item = resolveSearchResult(record.path);
    const name = item?.name ?? record.name;
    return highlightQueryMatch(name, searchSource.query);
  }, [resolveSearchResult, searchSource.query]);

  const renderSearchMeta = useCallback((record: FileNode) => {
    const item = resolveSearchResult(record.path);
    if (!item) {
      return `${record.isDir ? '-' : formatSize(record.size)} | ${formatDate(record.modTime)}`;
    }
    const sizeText = item.isDir ? '-' : formatSize(item.size);
    return `${sizeText} | ${formatDate(item.modTime)} | ${item.spaceName}`;
  }, [resolveSearchResult]);

  const isSearching = isSearchMode && searchSource.isSearching;
  const activeErrorMessage = isSearchMode ? searchSource.errorMessage : browseErrorMessage;
  const activeLoading = isSearchMode ? isSearching : browseLoading;

  return {
    searchSource,
    sourceContent,
    openSearchResultByRecordPath,
    renderSearchName,
    renderSearchMeta,
    activeErrorMessage,
    activeLoading,
  };
}
