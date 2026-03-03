
import { useState, useCallback } from 'react';
import type { FileNode } from '../types';
import { apiFetch } from '@/api/client';
import { toApiError } from '@/api/error';
import { useTranslation } from 'react-i18next';

export function useBrowseApi() {
  const { t } = useTranslation();
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
    // System browse root candidates for Space creation flow.
    return await fetchData('/api/browse/base-directories', t('browseApi.loadBaseDirectoriesFailed'));
  }, [fetchData, t]);

  const fetchDirectoryContents = useCallback(async (path: string) => {
    // System browse endpoint. Not for space-scoped browsing.
    const url = `/api/browse?path=${encodeURIComponent(path)}`;
    return await fetchData(url, t('browseApi.loadDirectoriesFailed'));
  }, [fetchData, t]);

  const fetchSpaceDirectoryContents = useCallback(async (spaceId: number, relativePath: string) => {
    // Space-scoped browsing endpoint for main explorer flow.
    const url = `/api/spaces/${spaceId}/browse?path=${encodeURIComponent(relativePath)}`;
    return await fetchData(url, t('browseApi.loadSpaceDirectoriesFailed'));
  }, [fetchData, t]);

  return { isLoading, error, fetchBaseDirectories, fetchDirectoryContents, fetchSpaceDirectoryContents };
}
