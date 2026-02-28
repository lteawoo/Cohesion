import { useState, useEffect, useCallback, useRef } from 'react';
import type { ProtocolStatus, StatusResponse } from '../types';
import { apiFetch } from '@/api/client';

const POLL_INTERVAL = 30000;
const KNOWN_PROTOCOL_STATUSES: readonly ProtocolStatus['status'][] = ['healthy', 'unhealthy', 'unavailable'];

function isKnownProtocolStatus(value: unknown): value is ProtocolStatus['status'] {
  return KNOWN_PROTOCOL_STATUSES.includes(value as ProtocolStatus['status']);
}

function normalizeProtocolStatus(value: unknown): ProtocolStatus {
  const raw = (value ?? {}) as Partial<Record<keyof ProtocolStatus, unknown>>;
  const status = isKnownProtocolStatus(raw.status) ? raw.status : 'unavailable';

  return {
    status,
    message: typeof raw.message === 'string' ? raw.message : '',
    port: typeof raw.port === 'string' ? raw.port : undefined,
    path: typeof raw.path === 'string' ? raw.path : undefined,
  };
}

function normalizeStatusResponse(value: unknown): StatusResponse {
  const raw = (value ?? {}) as {
    protocols?: Record<string, unknown>;
    hosts?: unknown;
  };
  const protocols: Record<string, ProtocolStatus> = {};

  if (raw.protocols && typeof raw.protocols === 'object') {
    Object.entries(raw.protocols).forEach(([key, status]) => {
      protocols[key] = normalizeProtocolStatus(status);
    });
  }

  return {
    protocols,
    hosts: Array.isArray(raw.hosts)
      ? raw.hosts.filter((host): host is string => typeof host === 'string')
      : [],
  };
}

export function useServerStatus() {
  const [status, setStatus] = useState<StatusResponse | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [isServerUp, setIsServerUp] = useState(false);
  const intervalRef = useRef<number | null>(null);

  const fetchStatus = useCallback(async () => {
    setIsLoading(true);
    try {
      const response = await apiFetch('/api/status');
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
      const raw = await response.json();
      const data = normalizeStatusResponse(raw);
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
