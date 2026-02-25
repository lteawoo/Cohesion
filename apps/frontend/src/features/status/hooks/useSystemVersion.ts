import { useState, useEffect, useCallback } from 'react';
import { apiFetch } from '@/api/client';
import type { SystemVersionResponse } from '../types';

export function useSystemVersion() {
  const [versionInfo, setVersionInfo] = useState<SystemVersionResponse | null>(null);
  const [isLoading, setIsLoading] = useState(false);

  const fetchVersionInfo = useCallback(async () => {
    setIsLoading(true);
    try {
      const response = await apiFetch('/api/system/version');
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
      const data: SystemVersionResponse = await response.json();
      setVersionInfo(data);
    } catch {
      setVersionInfo(null);
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchVersionInfo();
  }, [fetchVersionInfo]);

  return { versionInfo, isLoading, refetch: fetchVersionInfo };
}
