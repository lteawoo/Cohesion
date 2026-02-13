
import { useState, useCallback } from 'react';
import type { FileNode } from '../types';

export function useBrowseApi() {
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  const fetchData = useCallback(async (url: string): Promise<FileNode[]> => {
    setIsLoading(true);
    setError(null);
    try {
      const response = await fetch(url);
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
      const data = await response.json();
      return data;
    } catch (e) {
      setError(e as Error);
      return []; // 에러 발생 시 빈 배열 반환
    } finally {
      setIsLoading(false);
    }
  }, []);

  const fetchBaseDirectories = useCallback(async () => {
    return await fetchData('/api/browse/base-directories');
  }, [fetchData]);

  const fetchDirectoryContents = useCallback(async (path: string, systemMode = false) => {
    const url = `/api/browse?path=${encodeURIComponent(path)}${systemMode ? '&system=true' : ''}`;
    return await fetchData(url);
  }, [fetchData]);

  const fetchSpaceDirectoryContents = useCallback(async (spaceId: number, relativePath: string) => {
    const url = `/api/spaces/${spaceId}/browse?path=${encodeURIComponent(relativePath)}`;
    return await fetchData(url);
  }, [fetchData]);

  return { isLoading, error, fetchBaseDirectories, fetchDirectoryContents, fetchSpaceDirectoryContents };
}
