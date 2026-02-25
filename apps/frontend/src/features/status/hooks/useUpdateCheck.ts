import { useState, useEffect, useCallback, useRef } from 'react';
import { apiFetch } from '@/api/client';
import type { UpdateCheckResponse } from '../types';

const POLL_INTERVAL = 10 * 60 * 1000;

export function useUpdateCheck() {
  const [updateInfo, setUpdateInfo] = useState<UpdateCheckResponse | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const intervalRef = useRef<number | null>(null);

  const fetchUpdateInfo = useCallback(async () => {
    setIsLoading(true);
    try {
      const response = await apiFetch('/api/system/update-check');
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
      const data: UpdateCheckResponse = await response.json();
      setUpdateInfo(data);
    } catch {
      setUpdateInfo(null);
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchUpdateInfo();
    intervalRef.current = window.setInterval(fetchUpdateInfo, POLL_INTERVAL);

    return () => {
      if (intervalRef.current !== null) {
        window.clearInterval(intervalRef.current);
      }
    };
  }, [fetchUpdateInfo]);

  return { updateInfo, isLoading, refetch: fetchUpdateInfo };
}
