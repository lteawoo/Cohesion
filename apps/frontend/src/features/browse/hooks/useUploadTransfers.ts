import { useCallback, useRef } from 'react';
import { apiUpload } from '@/api/client';
import type { Space } from '@/features/space/types';
import { useTransferCenterStore } from '@/stores/transferCenterStore';
import {
  createTransferId,
  isAbortError,
  MAX_ACTIVE_UPLOAD_BATCHES,
  normalizeRelativePath,
  normalizeUploadSource,
  type TransferMessageApi,
  type Translate,
  type UploadBatchQueueEntry,
  type UploadConflictPolicy,
  type UploadExecutionResult,
  type UploadExecutionTask,
  type UploadResult,
  type UploadSource,
  type UploadSummary,
  type UploadResponsePayload,
} from './transferOperationsShared';

interface UseUploadTransfersParams {
  selectedSpace?: Space;
  t: Translate;
  message: TransferMessageApi;
  refreshContents: () => Promise<void>;
  promptConflictPolicy: (fileName: string) => Promise<UploadConflictPolicy | null>;
  readErrorMessage: (response: Response, fallback: string) => Promise<string>;
  setUploadTransferStatus: (
    transferId: string,
    name: string,
    status: 'queued' | 'uploading' | 'completed' | 'failed' | 'canceled',
    options?: { loaded?: number; total?: number; message?: string }
  ) => void;
}

interface UseUploadTransfersResult {
  handleFileUpload: (files: UploadSource, targetPath: string) => Promise<void>;
  cancelUploadTransfer: (transferId: string) => boolean;
  dismissUploadTransfer: (transferId: string) => void;
}

export function useUploadTransfers({
  selectedSpace,
  t,
  message,
  refreshContents,
  promptConflictPolicy,
  readErrorMessage,
  setUploadTransferStatus,
}: UseUploadTransfersParams): UseUploadTransfersResult {
  const uploadAbortControllersRef = useRef<Map<string, AbortController>>(new Map());
  const canceledQueuedUploadTransferIdsRef = useRef<Set<string>>(new Set());
  const uploadBatchQueueRef = useRef<UploadBatchQueueEntry[]>([]);
  const activeUploadBatchCountRef = useRef(0);

  const performUpload = useCallback(
    async (
      file: File,
      targetPath: string,
      transferId: string,
      signal: AbortSignal,
      conflictPolicy?: UploadConflictPolicy
    ): Promise<UploadResult> => {
      if (!selectedSpace) {
        throw new Error(t('fileOperations.selectedSpaceRequired'));
      }

      const formData = new FormData();
      formData.append('path', normalizeRelativePath(targetPath));
      formData.append('size', String(file.size));
      if (conflictPolicy) {
        formData.append('conflictPolicy', conflictPolicy);
      }
      formData.append('file', file);

      const response = await apiUpload(`/api/spaces/${selectedSpace.id}/files/upload`, {
        method: 'POST',
        body: formData,
      }, {
        signal,
        onUploadProgress: (loaded, total) => {
          setUploadTransferStatus(transferId, file.name, 'uploading', { loaded, total });
        },
      });

      if (!response.ok) {
        const errorMessage = await readErrorMessage(response, t('fileOperations.uploadFailed'));
        const uploadError = new Error(errorMessage) as Error & { status?: number };
        uploadError.status = response.status;
        throw uploadError;
      }

      const result = (await response.json()) as UploadResponsePayload;
      const status = result.status === 'skipped' ? 'skipped' : 'uploaded';
      const filename = typeof result.filename === 'string' && result.filename.trim()
        ? result.filename
        : file.name;
      return { status, filename };
    },
    [readErrorMessage, selectedSpace, setUploadTransferStatus, t]
  );

  const runUploadBatch = useCallback(async (batch: UploadBatchQueueEntry): Promise<UploadExecutionResult[]> => {
    let batchConflictPolicy: UploadConflictPolicy | null = null;
    const results: UploadExecutionResult[] = [...batch.settledResults];

    for (const task of batch.tasks) {
      if (canceledQueuedUploadTransferIdsRef.current.delete(task.transferId)) {
        setUploadTransferStatus(task.transferId, task.file.name, 'canceled', {
          loaded: 0,
          total: task.file.size,
          message: t('fileOperations.transferCanceled'),
        });
        results.push({
          transferId: task.transferId,
          file: task.file,
          outcome: 'canceled',
          message: t('fileOperations.transferCanceled'),
        });
        continue;
      }

      const abortController = new AbortController();
      uploadAbortControllersRef.current.set(task.transferId, abortController);
      setUploadTransferStatus(task.transferId, task.file.name, 'uploading', { loaded: 0, total: task.file.size });

      try {
        const result = await performUpload(
          task.file,
          batch.targetPath,
          task.transferId,
          abortController.signal,
          batchConflictPolicy ?? undefined
        );
        uploadAbortControllersRef.current.delete(task.transferId);
        setUploadTransferStatus(task.transferId, task.file.name, 'completed', {
          loaded: task.file.size,
          total: task.file.size,
          message: result.status === 'skipped' ? t('fileOperations.transferItemSkipped') : undefined,
        });
        results.push({
          transferId: task.transferId,
          file: task.file,
          outcome: result.status === 'skipped' ? 'skipped' : 'uploaded',
          filename: result.filename,
        });
        continue;
      } catch (error: unknown) {
        const status = error && typeof error === 'object' && 'status' in error
          ? Number(error.status)
          : 0;
        const errorMessage = error && typeof error === 'object' && 'message' in error
          ? String(error.message)
          : t('fileOperations.uploadFailed');

        if (isAbortError(error)) {
          uploadAbortControllersRef.current.delete(task.transferId);
          setUploadTransferStatus(task.transferId, task.file.name, 'canceled', {
            loaded: 0,
            total: task.file.size,
            message: t('fileOperations.transferCanceled'),
          });
          results.push({
            transferId: task.transferId,
            file: task.file,
            outcome: 'canceled',
            message: t('fileOperations.transferCanceled'),
          });
          continue;
        }

        if (status === 409 && !batchConflictPolicy) {
          const selectedPolicy = await promptConflictPolicy(task.file.name);
          if (!selectedPolicy) {
            uploadAbortControllersRef.current.delete(task.transferId);
            setUploadTransferStatus(task.transferId, task.file.name, 'canceled', {
              loaded: 0,
              total: task.file.size,
              message: t('fileOperations.transferCanceled'),
            });
            results.push({
              transferId: task.transferId,
              file: task.file,
              outcome: 'canceled',
              message: t('fileOperations.transferCanceled'),
            });
            continue;
          }
          batchConflictPolicy = selectedPolicy;

          try {
            const retried = await performUpload(
              task.file,
              batch.targetPath,
              task.transferId,
              abortController.signal,
              batchConflictPolicy
            );
            uploadAbortControllersRef.current.delete(task.transferId);
            setUploadTransferStatus(task.transferId, task.file.name, 'completed', {
              loaded: task.file.size,
              total: task.file.size,
              message: retried.status === 'skipped' ? t('fileOperations.transferItemSkipped') : undefined,
            });
            results.push({
              transferId: task.transferId,
              file: task.file,
              outcome: retried.status === 'skipped' ? 'skipped' : 'uploaded',
              filename: retried.filename,
            });
          } catch (retryError: unknown) {
            uploadAbortControllersRef.current.delete(task.transferId);
            const retryMessage = retryError && typeof retryError === 'object' && 'message' in retryError
              ? String(retryError.message)
              : t('fileOperations.uploadFailed');
            setUploadTransferStatus(task.transferId, task.file.name, 'failed', {
              loaded: 0,
              total: task.file.size,
              message: retryMessage,
            });
            results.push({
              transferId: task.transferId,
              file: task.file,
              outcome: 'failed',
              message: retryMessage,
            });
          }
          continue;
        }

        uploadAbortControllersRef.current.delete(task.transferId);
        setUploadTransferStatus(task.transferId, task.file.name, 'failed', {
          loaded: 0,
          total: task.file.size,
          message: errorMessage,
        });
        results.push({
          transferId: task.transferId,
          file: task.file,
          outcome: 'failed',
          message: errorMessage,
        });
      }
    }

    return results;
  }, [performUpload, promptConflictPolicy, setUploadTransferStatus, t]);

  const pumpUploadQueue = useCallback(function pumpUploadQueue() {
    while (activeUploadBatchCountRef.current < MAX_ACTIVE_UPLOAD_BATCHES && uploadBatchQueueRef.current.length > 0) {
      const nextBatch = uploadBatchQueueRef.current.shift();
      if (!nextBatch) {
        return;
      }

      activeUploadBatchCountRef.current += 1;
      void runUploadBatch(nextBatch)
        .then((results) => {
          nextBatch.resolve(results);
        })
        .catch((error) => {
          nextBatch.reject(error);
        })
        .finally(() => {
          activeUploadBatchCountRef.current = Math.max(0, activeUploadBatchCountRef.current - 1);
          pumpUploadQueue();
        });
    }
  }, [runUploadBatch]);

  const enqueueUploadBatch = useCallback((tasks: UploadExecutionTask[], targetPath: string): Promise<UploadExecutionResult[]> => {
    return new Promise((resolve, reject) => {
      uploadBatchQueueRef.current.push({
        tasks,
        targetPath,
        settledResults: [],
        resolve,
        reject,
      });
      pumpUploadQueue();
    });
  }, [pumpUploadQueue]);

  const handleFileUpload = useCallback(async (files: UploadSource, targetPath: string) => {
    const uploadFiles = normalizeUploadSource(files);
    if (uploadFiles.length === 0) {
      return;
    }

    const summary: UploadSummary = { uploaded: 0, skipped: 0, failed: 0 };
    const failedReasons: string[] = [];
    const uploadedNames: string[] = [];
    const skippedNames: string[] = [];
    const queuedTasks = uploadFiles.map((file) => {
      const transferId = createTransferId();
      setUploadTransferStatus(transferId, file.name, 'queued', { loaded: 0, total: file.size });
      return {
        transferId,
        file,
      };
    });

    let executionResults: UploadExecutionResult[] = [];
    try {
      executionResults = await enqueueUploadBatch(queuedTasks, targetPath);
    } catch (error) {
      message.error(error instanceof Error ? error.message : t('fileOperations.uploadFailed'));
      return;
    }
    const abortedByUser = executionResults.some((result) => result.outcome === 'canceled');

    for (const result of executionResults) {
      switch (result.outcome) {
        case 'uploaded':
          summary.uploaded += 1;
          uploadedNames.push(result.filename ?? result.file.name);
          break;
        case 'skipped':
          summary.skipped += 1;
          skippedNames.push(result.filename ?? result.file.name);
          break;
        case 'failed':
          summary.failed += 1;
          failedReasons.push(`${result.file.name}: ${result.message ?? t('fileOperations.uploadFailed')}`);
          break;
        case 'canceled':
          break;
        default:
          break;
      }
    }

    if (summary.uploaded > 0 || summary.skipped > 0) {
      await refreshContents();
    }

    if (uploadFiles.length === 1) {
      if (abortedByUser) {
        message.info(t('fileOperations.uploadStopped'));
        return;
      }
      if (summary.uploaded === 1) {
        message.success(t('fileOperations.uploadSingleSuccess', { name: uploadedNames[0] ?? uploadFiles[0].name }));
        return;
      }
      if (summary.skipped === 1) {
        message.warning(t('fileOperations.uploadSingleSkipped', { name: skippedNames[0] ?? uploadFiles[0].name }));
        return;
      }
      message.error(failedReasons[0] ?? t('fileOperations.uploadFailed'));
      return;
    }

    const summaryMessage = t('fileOperations.uploadSummary', {
      uploaded: summary.uploaded,
      skipped: summary.skipped,
      failed: summary.failed,
    });
    if (abortedByUser) {
      message.warning(`${summaryMessage} (${t('fileOperations.userAbortedSuffix')})`);
      return;
    }
    if (summary.failed > 0) {
      const firstFailure = failedReasons[0] ? ` - ${failedReasons[0]}` : '';
      message.warning(`${summaryMessage}${firstFailure}`);
      return;
    }
    message.success(summaryMessage);
  }, [enqueueUploadBatch, message, refreshContents, setUploadTransferStatus, t]);

  const cancelUploadTransfer = useCallback((transferId: string): boolean => {
    const activeUploadController = uploadAbortControllersRef.current.get(transferId);
    if (activeUploadController) {
      activeUploadController.abort();
      return true;
    }

    const queuedUploadBatch = uploadBatchQueueRef.current.find((batch) => (
      batch.tasks.some((task) => task.transferId === transferId)
    ));
    if (queuedUploadBatch) {
      const queuedTaskIndex = queuedUploadBatch.tasks.findIndex((task) => task.transferId === transferId);
      const [queuedTask] = queuedUploadBatch.tasks.splice(queuedTaskIndex, 1);
      if (queuedTask) {
        queuedUploadBatch.settledResults.push({
          transferId: queuedTask.transferId,
          file: queuedTask.file,
          outcome: 'canceled',
          message: t('fileOperations.transferCanceled'),
        });
        setUploadTransferStatus(queuedTask.transferId, queuedTask.file.name, 'canceled', {
          loaded: 0,
          total: queuedTask.file.size,
          message: t('fileOperations.transferCanceled'),
        });
      }
      if (queuedUploadBatch.tasks.length === 0) {
        uploadBatchQueueRef.current = uploadBatchQueueRef.current.filter((batch) => batch !== queuedUploadBatch);
        queuedUploadBatch.resolve([...queuedUploadBatch.settledResults]);
      }
      return true;
    }

    const queuedUploadTransfer = useTransferCenterStore.getState().transfers.find((transfer) => (
      transfer.id === transferId && transfer.kind === 'upload' && transfer.status === 'queued'
    ));
    if (queuedUploadTransfer?.kind === 'upload') {
      canceledQueuedUploadTransferIdsRef.current.add(transferId);
      setUploadTransferStatus(transferId, queuedUploadTransfer.name, 'canceled', {
        loaded: 0,
        total: queuedUploadTransfer.total,
        message: t('fileOperations.transferCanceled'),
      });
      return true;
    }

    return false;
  }, [setUploadTransferStatus, t]);

  const dismissUploadTransfer = useCallback((transferId: string) => {
    uploadAbortControllersRef.current.delete(transferId);
    canceledQueuedUploadTransferIdsRef.current.delete(transferId);
  }, []);

  return {
    handleFileUpload,
    cancelUploadTransfer,
    dismissUploadTransfer,
  };
}
