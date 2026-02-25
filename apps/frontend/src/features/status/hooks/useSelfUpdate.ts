import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { getSelfUpdateStatus, startSelfUpdate, type SelfUpdateStatus } from '@/api/config';

const POLL_INTERVAL = 3000;
const RUNNING_STATES = new Set(['checking', 'downloading', 'staging', 'switching']);

export function useSelfUpdate() {
  const [status, setStatus] = useState<SelfUpdateStatus | null>(null);
  const [isStarting, setIsStarting] = useState(false);
  const intervalRef = useRef<number | null>(null);

  const fetchStatus = useCallback(async () => {
    try {
      const nextStatus = await getSelfUpdateStatus();
      setStatus(nextStatus);
    } catch {
      // 서버 재시작 중에는 조회 실패가 자연스러워 상태 유지
    }
  }, []);

  const startUpdate = useCallback(async (force = false) => {
    setIsStarting(true);
    try {
      await startSelfUpdate(force);
      await fetchStatus();
    } finally {
      setIsStarting(false);
    }
  }, [fetchStatus]);

  useEffect(() => {
    fetchStatus();
    intervalRef.current = window.setInterval(fetchStatus, POLL_INTERVAL);

    return () => {
      if (intervalRef.current !== null) {
        window.clearInterval(intervalRef.current);
      }
    };
  }, [fetchStatus]);

  const isUpdating = useMemo(() => {
    return status !== null && RUNNING_STATES.has(status.state);
  }, [status]);

  return {
    status,
    isStarting,
    isUpdating,
    startUpdate,
    refetchStatus: fetchStatus,
  };
}
