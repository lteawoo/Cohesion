
import { useState, useCallback } from 'react';
import type { FileNode } from '../types';
import { apiFetch } from '@/api/client';
import { toApiError } from '@/api/error';

export function useBrowseApi() {
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  const fetchData = useCallback(async (url: string, fallbackErrorMessage: string): Promise<FileNode[]> => {
    setIsLoading(true);
    setError(null);
    try {
      const response = await apiFetch(url);
      if (!response.ok) {
        throw await toApiError(response, fallbackErrorMessage);
      }
      const data = await response.json();
      return data;
    } catch (e) {
      const apiError = e instanceof Error ? e : new Error(fallbackErrorMessage);
      setError(apiError);
      throw apiError;
    } finally {
      setIsLoading(false);
    }
  }, []);

  const fetchBaseDirectories = useCallback(async () => {
    return await fetchData('/api/browse/base-directories', '기본 디렉토리를 불러오지 못했습니다.');
  }, [fetchData]);

  const fetchDirectoryContents = useCallback(async (path: string, systemMode = false) => {
    const url = `/api/browse?path=${encodeURIComponent(path)}${systemMode ? '&system=true' : ''}`;
    return await fetchData(url, '디렉토리 목록을 불러오지 못했습니다.');
  }, [fetchData]);

  const fetchSpaceDirectoryContents = useCallback(async (spaceId: number, relativePath: string) => {
    const url = `/api/spaces/${spaceId}/browse?path=${encodeURIComponent(relativePath)}`;
    return await fetchData(url, 'Space 폴더 목록을 불러오지 못했습니다.');
  }, [fetchData]);

  return { isLoading, error, fetchBaseDirectories, fetchDirectoryContents, fetchSpaceDirectoryContents };
}
