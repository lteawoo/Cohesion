import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { searchFiles } from "@/features/search/api/searchApi";
import type { SearchFileResult } from "@/features/search/types";
import { useBrowseStore } from "@/stores/browseStore";
import { useSpaceStore } from "@/stores/spaceStore";
import { useTranslation } from "react-i18next";

const MIN_SEARCH_QUERY_LENGTH = 2;
const SEARCH_PAGE_INITIAL_LIMIT = 80;
const SEARCH_PAGE_LIMIT_STEP = 80;
const SEARCH_PAGE_MAX_LIMIT = 400;

function resolveBrowsePath(item: SearchFileResult): string {
  return item.isDir ? item.path : item.parentPath;
}

function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === "AbortError";
}

export interface SearchExplorerSource {
  query: string;
  results: SearchFileResult[];
  errorMessage: string | null;
  isSearching: boolean;
  resultCount: number;
  currentLimit: number;
  hasMore: boolean;
  hasEnoughQuery: boolean;
  canLoadMore: boolean;
  loadMore: () => void;
  openResult: (item: SearchFileResult) => void;
}

export function useSearchExplorerSource(enabled: boolean): SearchExplorerSource {
  const { t } = useTranslation();
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const spaces = useSpaceStore((state) => state.spaces);
  const setPath = useBrowseStore((state) => state.setPath);

  const query = useMemo(() => searchParams.get("q")?.trim() ?? "", [searchParams]);
  const [results, setResults] = useState<SearchFileResult[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [requestedLimit, setRequestedLimit] = useState(SEARCH_PAGE_INITIAL_LIMIT);
  const [hasMore, setHasMore] = useState(false);
  const lastQueryRef = useRef(query);

  useEffect(() => {
    if (!enabled) {
      return;
    }

    if (lastQueryRef.current !== query) {
      lastQueryRef.current = query;
      setHasMore(false);
      setResults([]);
      setErrorMessage(null);
      if (requestedLimit !== SEARCH_PAGE_INITIAL_LIMIT) {
        setRequestedLimit(SEARCH_PAGE_INITIAL_LIMIT);
        setIsLoading(false);
        return;
      }
    }

    const controller = new AbortController();
    let isDisposed = false;
    if (query.length < MIN_SEARCH_QUERY_LENGTH) {
      setResults([]);
      setIsLoading(false);
      setErrorMessage(null);
      setHasMore(false);
      return () => {
        isDisposed = true;
        controller.abort();
      };
    }

    setIsLoading(true);
    setErrorMessage(null);

    void (async () => {
      try {
        const data = await searchFiles(query, requestedLimit, { signal: controller.signal });
        if (!isDisposed) {
          setResults(data.items);
          setHasMore(data.hasMore);
        }
      } catch (error) {
        if (isAbortError(error)) {
          return;
        }
        if (!isDisposed) {
          setResults([]);
          setErrorMessage(error instanceof Error ? error.message : t('search.loadResultsFailed'));
          setHasMore(false);
        }
      } finally {
        if (!isDisposed) {
          setIsLoading(false);
        }
      }
    })();

    return () => {
      isDisposed = true;
      controller.abort();
    };
  }, [enabled, query, requestedLimit, t]);

  const openResult = useCallback((item: SearchFileResult) => {
    const targetSpace = spaces.find((space) => space.id === item.spaceId);
    if (!targetSpace) {
      return;
    }
    setPath(resolveBrowsePath(item), targetSpace);
    navigate("/", {
      state: {
        fromSearchQuery: query,
      },
    });
  }, [navigate, query, setPath, spaces]);

  const hasEnoughQuery = query.length >= MIN_SEARCH_QUERY_LENGTH;
  const isSearching = enabled && hasEnoughQuery && isLoading;
  const resultCount = hasEnoughQuery ? results.length : 0;
  const canLoadMore = hasMore && requestedLimit < SEARCH_PAGE_MAX_LIMIT;

  const loadMore = useCallback(() => {
    setRequestedLimit((current) => Math.min(current + SEARCH_PAGE_LIMIT_STEP, SEARCH_PAGE_MAX_LIMIT));
  }, []);

  return {
    query,
    results,
    errorMessage,
    isSearching,
    resultCount,
    currentLimit: requestedLimit,
    hasMore,
    hasEnoughQuery,
    canLoadMore,
    loadMore,
    openResult,
  };
}
