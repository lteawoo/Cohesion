import { useCallback, useRef } from 'react';
import type { Space } from '@/features/space/types';
import { apiFetch } from '@/api/client';
import {
  createTransferId,
  isAbortError,
  normalizeRelativePath,
  triggerBrowserDownloadFromUrl,
  type DownloadTicketResponse,
  type TransferMessageApi,
  type Translate,
} from './transferOperationsShared';

interface UseDirectDownloadTransfersParams {
  selectedSpace?: Space;
  t: Translate;
  readErrorMessage: (response: Response, fallback: string) => Promise<string>;
  setDownloadTransferStatus: (
    transferId: string,
    name: string,
    status: 'running' | 'completed' | 'handed_off' | 'failed' | 'expired' | 'canceled',
    options?: { loaded?: number; total?: number; message?: string; spaceId?: number }
  ) => void;
}

interface UseDirectDownloadTransfersResult {
  startDirectDownload: (relativePath: string, downloadName: string) => Promise<void>;
  cancelDirectDownload: (transferId: string) => boolean;
  dismissDirectDownload: (transferId: string) => void;
}

export function useDirectDownloadTransfers({
  selectedSpace,
  t,
  readErrorMessage,
  setDownloadTransferStatus,
}: UseDirectDownloadTransfersParams): UseDirectDownloadTransfersResult {
  const downloadAbortControllersRef = useRef<Map<string, AbortController>>(new Map());

  const startDirectDownload = useCallback(async (relativePath: string, downloadName: string) => {
    if (!selectedSpace) {
      throw new Error(t('fileOperations.selectedSpaceRequired'));
    }

    const transferId = createTransferId();
    const abortController = new AbortController();
    downloadAbortControllersRef.current.set(transferId, abortController);
    setDownloadTransferStatus(transferId, downloadName, 'running', {
      spaceId: selectedSpace.id,
    });

    try {
      const ticketResponse = await apiFetch(`/api/spaces/${selectedSpace.id}/files/download-ticket`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ path: normalizeRelativePath(relativePath) }),
        signal: abortController.signal,
      });
      downloadAbortControllersRef.current.delete(transferId);

      if (!ticketResponse.ok) {
        const errorMessage = await readErrorMessage(ticketResponse, t('fileOperations.downloadPrepareFailed'));
        setDownloadTransferStatus(transferId, downloadName, 'failed', {
          message: errorMessage,
          spaceId: selectedSpace.id,
        });
        throw new Error(errorMessage);
      }

      const payload = (await ticketResponse.json()) as DownloadTicketResponse;
      if (!payload.downloadUrl || typeof payload.downloadUrl !== 'string') {
        const errorMessage = t('fileOperations.downloadUrlCreateFailed');
        setDownloadTransferStatus(transferId, downloadName, 'failed', {
          message: errorMessage,
          spaceId: selectedSpace.id,
        });
        throw new Error(errorMessage);
      }

      const resolvedDownloadName = payload.fileName ?? downloadName;
      setDownloadTransferStatus(transferId, resolvedDownloadName, 'handed_off', {
        spaceId: selectedSpace.id,
      });
      triggerBrowserDownloadFromUrl(payload.downloadUrl, payload.fileName);
    } catch (error) {
      downloadAbortControllersRef.current.delete(transferId);
      if (isAbortError(error)) {
        setDownloadTransferStatus(transferId, downloadName, 'canceled', {
          message: t('fileOperations.transferCanceled'),
          spaceId: selectedSpace.id,
        });
        return;
      }
      const errorMessage = error instanceof Error ? error.message : t('fileOperations.downloadFailed');
      setDownloadTransferStatus(transferId, downloadName, 'failed', {
        message: errorMessage,
        spaceId: selectedSpace.id,
      });
      throw error;
    }
  }, [readErrorMessage, selectedSpace, setDownloadTransferStatus, t]);

  const cancelDirectDownload = useCallback((transferId: string): boolean => {
    const activeDownloadController = downloadAbortControllersRef.current.get(transferId);
    if (activeDownloadController) {
      activeDownloadController.abort();
      return true;
    }
    return false;
  }, []);

  const dismissDirectDownload = useCallback((transferId: string) => {
    downloadAbortControllersRef.current.delete(transferId);
  }, []);

  return {
    startDirectDownload,
    cancelDirectDownload,
    dismissDirectDownload,
  };
}
