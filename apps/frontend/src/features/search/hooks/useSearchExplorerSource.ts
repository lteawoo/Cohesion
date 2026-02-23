import { useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { searchFiles } from "@/features/search/api/searchApi";
import type { SearchFileResult } from "@/features/search/types";
import { useBrowseStore } from "@/stores/browseStore";
import { useSpaceStore } from "@/stores/spaceStore";
import { useTranslation } from "react-i18next";

const MIN_SEARCH_QUERY_LENGTH = 2;
const SEARCH_PAGE_LIMIT = 80;

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
  hasEnoughQuery: boolean;
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

  useEffect(() => {
    if (!enabled) {
      return;
    }

    const controller = new AbortController();
    let isDisposed = false;
    if (query.length < MIN_SEARCH_QUERY_LENGTH) {
      setResults([]);
      setIsLoading(false);
      setErrorMessage(null);
      return () => {
        isDisposed = true;
        controller.abort();
      };
    }

    setIsLoading(true);
    setErrorMessage(null);

    void (async () => {
      try {
        const data = await searchFiles(query, SEARCH_PAGE_LIMIT, { signal: controller.signal });
        if (!isDisposed) {
          setResults(data);
        }
      } catch (error) {
        if (isAbortError(error)) {
          return;
        }
        if (!isDisposed) {
          setResults([]);
          setErrorMessage(error instanceof Error ? error.message : t('search.loadResultsFailed'));
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
  }, [enabled, query, t]);

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

  return {
    query,
    results,
    errorMessage,
    isSearching,
    resultCount,
    hasEnoughQuery,
    openResult,
  };
}
