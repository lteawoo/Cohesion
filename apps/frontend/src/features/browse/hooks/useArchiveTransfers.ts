import { useCallback, useRef } from 'react';
import { apiFetch } from '@/api/client';
import { useTransferCenterStore } from '@/stores/transferCenterStore';
import type { ArchiveTransferItem, BrowserTransferStatus } from '@/stores/transferCenterStore';
import type { Space } from '@/features/space/types';
import {
  createTransferId,
  MAX_ACTIVE_ARCHIVE_TASKS,
  triggerBrowserDownloadFromUrl,
  type ArchiveDownloadJobResponse,
  type ArchiveQueueEntry,
  type DownloadTicketResponse,
  type TransferMessageApi,
  type Translate,
} from './transferOperationsShared';

interface UseArchiveTransfersParams {
  selectedSpace?: Space;
  t: Translate;
  message: TransferMessageApi;
  readErrorMessage: (response: Response, fallback: string) => Promise<string>;
  setArchiveTransferStatus: (
    transferId: string,
    name: string,
    payload: ArchiveDownloadJobResponse,
    overrideStatus?: BrowserTransferStatus,
    overrideMessage?: string,
    spaceId?: number,
    requestedPaths?: string[]
  ) => void;
}

interface ResumeArchiveTransferParams {
  transferId: string;
  archiveSpaceId: number;
  initialJob: ArchiveDownloadJobResponse;
  fallbackName: string;
  notify: boolean;
  requestedPaths?: string[];
}

interface UseArchiveTransfersResult {
  enqueueArchiveDownload: (relativePaths: string[], fallbackArchiveName: string) => Promise<void>;
  cancelArchiveTransfer: (transferId: string) => boolean;
  dismissArchiveTransfer: (transferId: string) => void;
  retryArchiveTransfer: (transferId: string) => void;
  resumeArchiveTransfer: (transfer: ArchiveTransferItem, notify: boolean) => void;
}

export function useArchiveTransfers({
  selectedSpace,
  t,
  message,
  readErrorMessage,
  setArchiveTransferStatus,
}: UseArchiveTransfersParams): UseArchiveTransfersResult {
  const archiveQueueRef = useRef<ArchiveQueueEntry[]>([]);
  const activeArchiveTaskCountRef = useRef(0);

  const isArchiveTransferCanceled = useCallback((transferId: string): boolean => {
    const transfer = useTransferCenterStore.getState().transfers.find((item) => item.id === transferId);
    return transfer?.kind === 'archive' && transfer.status === 'canceled';
  }, []);

  const waitForNextArchivePoll = useCallback(async () => {
    await new Promise((resolve) => window.setTimeout(resolve, 800));
  }, []);

  const driveArchiveDownloadFlow = useCallback(async ({
    transferId,
    archiveSpaceId,
    initialJob,
    fallbackName,
    notify,
    requestedPaths,
  }: ResumeArchiveTransferParams): Promise<void> => {
    let archiveJob = initialJob;
    let archiveName = archiveJob.fileName ?? fallbackName;
    setArchiveTransferStatus(transferId, archiveName, archiveJob, undefined, undefined, archiveSpaceId, requestedPaths);

    while (archiveJob.status === 'queued' || archiveJob.status === 'running') {
      if (isArchiveTransferCanceled(transferId)) {
        return;
      }
      await waitForNextArchivePoll();
      const statusResponse = await apiFetch(
        `/api/spaces/${archiveSpaceId}/files/archive-downloads?jobId=${encodeURIComponent(archiveJob.jobId ?? '')}`
      );
      if (!statusResponse.ok) {
        if (isArchiveTransferCanceled(transferId)) {
          return;
        }
        const errorMessage = await readErrorMessage(statusResponse, t('fileOperations.downloadPrepareFailed'));
        const terminalStatus = statusResponse.status === 404 || statusResponse.status === 410 ? 'expired' : 'failed';
        setArchiveTransferStatus(
          transferId,
          archiveName,
          { ...archiveJob, failureReason: errorMessage },
          terminalStatus,
          errorMessage,
          archiveSpaceId,
          requestedPaths
        );
        if (notify) {
          if (terminalStatus === 'expired') {
            message.warning(errorMessage);
          } else {
            message.error(errorMessage);
          }
        }
        return;
      }
      archiveJob = (await statusResponse.json()) as ArchiveDownloadJobResponse;
      archiveName = archiveJob.fileName ?? archiveName;
      if (isArchiveTransferCanceled(transferId)) {
        return;
      }
      setArchiveTransferStatus(transferId, archiveName, archiveJob, undefined, undefined, archiveSpaceId, requestedPaths);
    }

    if (archiveJob.status === 'canceled') {
      const canceledMessage = t('fileOperations.transferCanceled');
      setArchiveTransferStatus(
        transferId,
        archiveName,
        archiveJob,
        'canceled',
        canceledMessage,
        archiveSpaceId,
        requestedPaths
      );
      return;
    }

    if (archiveJob.status === 'failed') {
      const failureReason = archiveJob.failureReason ?? t('fileOperations.downloadPrepareFailed');
      setArchiveTransferStatus(
        transferId,
        archiveName,
        archiveJob,
        'failed',
        failureReason,
        archiveSpaceId,
        requestedPaths
      );
      if (notify) {
        message.error(failureReason);
      }
      return;
    }

    if (archiveJob.status === 'expired') {
      const expiredMessage = archiveJob.failureReason ?? t('fileOperations.archiveExpired');
      setArchiveTransferStatus(
        transferId,
        archiveName,
        archiveJob,
        'expired',
        expiredMessage,
        archiveSpaceId,
        requestedPaths
      );
      if (notify) {
        message.warning(expiredMessage);
      }
      return;
    }

    if (isArchiveTransferCanceled(transferId)) {
      return;
    }
    const ticketResponse = await apiFetch(`/api/spaces/${archiveSpaceId}/files/archive-download-ticket`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ jobId: archiveJob.jobId }),
    });
    if (!ticketResponse.ok) {
      const errorMessage = await readErrorMessage(ticketResponse, t('fileOperations.downloadPrepareFailed'));
      const terminalStatus = ticketResponse.status === 410 ? 'expired' : 'failed';
      setArchiveTransferStatus(
        transferId,
        archiveName,
        { ...archiveJob, failureReason: errorMessage },
        terminalStatus,
        errorMessage,
        archiveSpaceId,
        requestedPaths
      );
      if (notify) {
        if (terminalStatus === 'expired') {
          message.warning(errorMessage);
        } else {
          message.error(errorMessage);
        }
      }
      return;
    }

    const payload = (await ticketResponse.json()) as DownloadTicketResponse;
    if (!payload.downloadUrl || typeof payload.downloadUrl !== 'string') {
      const errorMessage = t('fileOperations.downloadUrlCreateFailed');
      setArchiveTransferStatus(
        transferId,
        archiveName,
        { ...archiveJob, failureReason: errorMessage },
        'failed',
        errorMessage,
        archiveSpaceId,
        requestedPaths
      );
      if (notify) {
        message.error(errorMessage);
      }
      return;
    }

    const readyName = payload.fileName ?? archiveName;
    setArchiveTransferStatus(
      transferId,
      readyName,
      { ...archiveJob, fileName: readyName, status: 'ready' },
      'handed_off',
      undefined,
      archiveSpaceId,
      requestedPaths
    );
    triggerBrowserDownloadFromUrl(payload.downloadUrl, payload.fileName);
    if (notify) {
      message.success(t('fileOperations.archiveReady', { name: readyName }));
    }
  }, [isArchiveTransferCanceled, message, readErrorMessage, setArchiveTransferStatus, t, waitForNextArchivePoll]);

  const runArchiveQueueTask = useCallback(async (task: ArchiveQueueEntry): Promise<void> => {
    const createResponse = await apiFetch(`/api/spaces/${task.archiveSpaceId}/files/archive-downloads`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ paths: task.relativePaths }),
    });
    if (!createResponse.ok) {
      const errorMessage = await readErrorMessage(createResponse, t('fileOperations.downloadPrepareFailed'));
      setArchiveTransferStatus(
        task.transferId,
        task.fallbackArchiveName,
        { status: 'failed', failureReason: errorMessage },
        'failed',
        errorMessage,
        task.archiveSpaceId,
        task.relativePaths
      );
      throw new Error(errorMessage);
    }

    const archiveJob = (await createResponse.json()) as ArchiveDownloadJobResponse;
    const jobId = typeof archiveJob.jobId === 'string' ? archiveJob.jobId : '';
    if (!jobId) {
      const errorMessage = t('fileOperations.downloadPrepareFailed');
      setArchiveTransferStatus(
        task.transferId,
        task.fallbackArchiveName,
        { status: 'failed', failureReason: errorMessage },
        'failed',
        errorMessage,
        task.archiveSpaceId,
        task.relativePaths
      );
      throw new Error(errorMessage);
    }

    await driveArchiveDownloadFlow({
      transferId: task.transferId,
      archiveSpaceId: task.archiveSpaceId,
      initialJob: archiveJob,
      fallbackName: archiveJob.fileName ?? task.fallbackArchiveName,
      notify: true,
      requestedPaths: task.relativePaths,
    });
  }, [driveArchiveDownloadFlow, readErrorMessage, setArchiveTransferStatus, t]);

  const pumpArchiveQueue = useCallback(function pumpArchiveQueue() {
    while (activeArchiveTaskCountRef.current < MAX_ACTIVE_ARCHIVE_TASKS && archiveQueueRef.current.length > 0) {
      const nextTask = archiveQueueRef.current.shift();
      if (!nextTask) {
        return;
      }

      activeArchiveTaskCountRef.current += 1;
      void runArchiveQueueTask(nextTask)
        .then(() => {
          nextTask.resolve();
        })
        .catch((error) => {
          nextTask.reject(error);
        })
        .finally(() => {
          activeArchiveTaskCountRef.current = Math.max(0, activeArchiveTaskCountRef.current - 1);
          pumpArchiveQueue();
        });
    }
  }, [runArchiveQueueTask]);

  const enqueueArchiveTask = useCallback((task: Omit<ArchiveQueueEntry, 'resolve' | 'reject'>): Promise<void> => {
    return new Promise((resolve, reject) => {
      archiveQueueRef.current.push({
        ...task,
        resolve,
        reject,
      });
      pumpArchiveQueue();
    });
  }, [pumpArchiveQueue]);

  const enqueueArchiveDownload = useCallback(async (relativePaths: string[], fallbackArchiveName: string) => {
    if (!selectedSpace) {
      throw new Error(t('fileOperations.selectedSpaceRequired'));
    }

    const transferId = createTransferId();
    setArchiveTransferStatus(transferId, fallbackArchiveName, {
      status: 'queued',
      fileName: fallbackArchiveName,
      totalItems: 0,
      processedItems: 0,
      totalSourceBytes: 0,
      processedSourceBytes: 0,
    }, undefined, undefined, selectedSpace.id, relativePaths);

    await enqueueArchiveTask({
      transferId,
      archiveSpaceId: selectedSpace.id,
      relativePaths,
      fallbackArchiveName,
    });
  }, [enqueueArchiveTask, selectedSpace, setArchiveTransferStatus, t]);

  const cancelArchiveTransfer = useCallback((transferId: string): boolean => {
    const activeArchiveTransfer = useTransferCenterStore.getState().transfers.find((transfer) => (
      transfer.id === transferId
      && transfer.kind === 'archive'
      && (transfer.status === 'queued' || transfer.status === 'running')
      && Boolean(transfer.jobId)
      && Boolean(transfer.spaceId)
    ));
    if (activeArchiveTransfer?.kind === 'archive' && activeArchiveTransfer.jobId && activeArchiveTransfer.spaceId) {
      void (async () => {
        const archiveJobId = activeArchiveTransfer.jobId;
        if (!archiveJobId) {
          return;
        }
        const cancelResponse = await apiFetch(
          `/api/spaces/${activeArchiveTransfer.spaceId}/files/archive-downloads?jobId=${encodeURIComponent(archiveJobId)}`,
          { method: 'DELETE' }
        );
        if (!cancelResponse.ok) {
          const errorMessage = await readErrorMessage(cancelResponse, t('fileOperations.downloadPrepareFailed'));
          message.error(errorMessage);
          return;
        }

        const canceledPayload = (await cancelResponse.json()) as ArchiveDownloadJobResponse;
        setArchiveTransferStatus(
          activeArchiveTransfer.id,
          canceledPayload.fileName ?? activeArchiveTransfer.name,
          canceledPayload,
          'canceled',
          t('fileOperations.transferCanceled'),
          activeArchiveTransfer.spaceId,
          activeArchiveTransfer.requestedPaths
        );
      })().catch((error: unknown) => {
        const errorMessage = error instanceof Error ? error.message : t('fileOperations.downloadPrepareFailed');
        message.error(errorMessage);
      });
      return true;
    }

    const queuedArchiveIndex = archiveQueueRef.current.findIndex((task) => task.transferId === transferId);
    if (queuedArchiveIndex !== -1) {
      const [queuedArchiveTask] = archiveQueueRef.current.splice(queuedArchiveIndex, 1);
      setArchiveTransferStatus(
        queuedArchiveTask.transferId,
        queuedArchiveTask.fallbackArchiveName,
        { status: 'queued', fileName: queuedArchiveTask.fallbackArchiveName },
        'canceled',
        t('fileOperations.transferCanceled'),
        queuedArchiveTask.archiveSpaceId,
        queuedArchiveTask.relativePaths
      );
      queuedArchiveTask.resolve();
      return true;
    }

    return false;
  }, [message, readErrorMessage, setArchiveTransferStatus, t]);

  const dismissArchiveTransfer = useCallback((transferId: string) => {
    const archiveTaskIndex = archiveQueueRef.current.findIndex((task) => task.transferId === transferId);
    if (archiveTaskIndex !== -1) {
      archiveQueueRef.current.splice(archiveTaskIndex, 1);
    }
  }, []);

  const retryArchiveTransfer = useCallback((transferId: string) => {
    const transfer = useTransferCenterStore.getState().transfers.find((item) => item.id === transferId);
    if (!transfer || transfer.kind !== 'archive') {
      return;
    }
    if (!transfer.spaceId || !transfer.requestedPaths?.length) {
      message.error(t('fileOperations.downloadPrepareFailed'));
      return;
    }

    setArchiveTransferStatus(
      transfer.id,
      transfer.name,
      {
        status: 'queued',
        fileName: transfer.name,
        totalItems: 0,
        processedItems: 0,
        totalSourceBytes: 0,
        processedSourceBytes: 0,
      },
      undefined,
      undefined,
      transfer.spaceId,
      transfer.requestedPaths
    );

    void enqueueArchiveTask({
      transferId: transfer.id,
      archiveSpaceId: transfer.spaceId,
      relativePaths: transfer.requestedPaths,
      fallbackArchiveName: transfer.name,
    }).catch((error: unknown) => {
      message.error(error instanceof Error ? error.message : t('fileOperations.downloadPrepareFailed'));
    });
  }, [enqueueArchiveTask, message, setArchiveTransferStatus, t]);

  const resumeArchiveTransfer = useCallback((transfer: ArchiveTransferItem, notify: boolean) => {
    const initialArchiveStatus = transfer.status === 'ready'
      ? 'ready'
      : transfer.status === 'running'
        ? 'running'
        : 'queued';

    void driveArchiveDownloadFlow({
      transferId: transfer.id,
      archiveSpaceId: transfer.spaceId!,
      initialJob: {
        jobId: transfer.jobId,
        status: initialArchiveStatus,
        fileName: transfer.name,
        totalItems: transfer.totalItems,
        processedItems: transfer.processedItems,
        totalSourceBytes: transfer.totalSourceBytes,
        processedSourceBytes: transfer.processedSourceBytes,
      },
      fallbackName: transfer.name,
      notify,
      requestedPaths: transfer.requestedPaths,
    });
  }, [driveArchiveDownloadFlow]);

  return {
    enqueueArchiveDownload,
    cancelArchiveTransfer,
    dismissArchiveTransfer,
    retryArchiveTransfer,
    resumeArchiveTransfer,
  };
}
