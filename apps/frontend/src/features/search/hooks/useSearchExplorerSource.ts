import { useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { searchFiles } from "@/features/search/api/searchApi";
import type { SearchFileResult } from "@/features/search/types";
import { useBrowseStore } from "@/stores/browseStore";
import { useSpaceStore } from "@/stores/spaceStore";

const MIN_SEARCH_QUERY_LENGTH = 2;
const SEARCH_PAGE_LIMIT = 200;

function resolveBrowsePath(item: SearchFileResult): string {
  return item.isDir ? item.path : item.parentPath;
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

    let isCancelled = false;
    if (query.length < MIN_SEARCH_QUERY_LENGTH) {
      setResults([]);
      setIsLoading(false);
      setErrorMessage(null);
      return () => {
        isCancelled = true;
      };
    }

    setIsLoading(true);
    setErrorMessage(null);

    void (async () => {
      try {
        const data = await searchFiles(query, SEARCH_PAGE_LIMIT);
        if (!isCancelled) {
          setResults(data);
        }
      } catch (error) {
        if (!isCancelled) {
          setResults([]);
          setErrorMessage(error instanceof Error ? error.message : "검색 결과를 불러오지 못했습니다.");
        }
      } finally {
        if (!isCancelled) {
          setIsLoading(false);
        }
      }
    })();

    return () => {
      isCancelled = true;
    };
  }, [enabled, query]);

  const openResult = useCallback((item: SearchFileResult) => {
    const targetSpace = spaces.find((space) => space.id === item.spaceId);
    if (!targetSpace) {
      return;
    }
    setPath(resolveBrowsePath(item), targetSpace);
    navigate("/");
  }, [navigate, setPath, spaces]);

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
