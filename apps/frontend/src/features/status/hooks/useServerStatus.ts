import { useState, useEffect, useCallback, useRef } from 'react';
import type { StatusResponse } from '../types';

const POLL_INTERVAL = 30000;

export function useServerStatus() {
  const [status, setStatus] = useState<StatusResponse | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [isServerUp, setIsServerUp] = useState(false);
  const intervalRef = useRef<number | null>(null);

  const fetchStatus = useCallback(async () => {
    setIsLoading(true);
    try {
      const response = await fetch('/api/status');
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
      const data: StatusResponse = await response.json();
      setStatus(data);
      setIsServerUp(true);
    } catch {
      setStatus(null);
      setIsServerUp(false);
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchStatus();

    intervalRef.current = window.setInterval(fetchStatus, POLL_INTERVAL);

    return () => {
      if (intervalRef.current !== null) {
        window.clearInterval(intervalRef.current);
      }
    };
  }, [fetchStatus]);

  return { status, isLoading, isServerUp, refetch: fetchStatus };
}
