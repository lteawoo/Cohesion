import { useCallback, useEffect, useRef } from 'react';
import { App, Checkbox, Radio, Space as AntSpace, Typography } from 'antd';
import { useBrowseStore } from '@/stores/browseStore';
import type { TreeInvalidationTarget } from '@/stores/browseStore';
import { useTransferCenterStore } from '@/stores/transferCenterStore';
import type {
  ArchiveTransferItem,
  BrowserTransferItem,
  BrowserTransferStatus,
  DownloadTransferItem,
  UploadTransferItem,
} from '@/stores/transferCenterStore';
import type { Space } from '@/features/space/types';
import { apiFetch, apiUpload } from '@/api/client';
import { useTranslation } from 'react-i18next';

type UploadConflictPolicy = 'overwrite' | 'rename' | 'skip';
type TransferMode = 'move' | 'copy';
type TransferConflictPolicy = 'overwrite' | 'rename' | 'skip';
type UploadSource = File | File[] | FileList;
type UploadStatus = 'uploaded' | 'skipped';
type TrashConflictPolicy = 'overwrite' | 'rename' | 'skip';

export interface TrashItem {
  id: number;
  originalPath: string;
  itemName: string;
  isDir: boolean;
  itemSize: number;
  deletedBy: string;
  deletedAt: string;
}

interface UseFileOperationsReturn {
  handleRename: (oldPath: string, newName: string) => Promise<void>;
  handleCreateFolder: (parentPath: string, folderName: string) => Promise<void>;
  handleDelete: (record: { path: string; name: string; isDir: boolean }) => Promise<void>;
  handleBulkDelete: (paths: string[]) => Promise<void>;
  fetchTrashItems: () => Promise<TrashItem[]>;
  handleTrashRestore: (ids: number[]) => Promise<void>;
  handleTrashDelete: (ids: number[]) => Promise<void>;
  handleTrashEmpty: () => Promise<void>;
  handleMove: (sources: string[], destination: string, destinationSpace?: Space) => Promise<void>;
  handleCopy: (sources: string[], destination: string, destinationSpace?: Space) => Promise<void>;
  handleBulkDownload: (paths: string[]) => Promise<void>;
  handleFileUpload: (files: UploadSource, targetPath: string) => Promise<void>;
  transfers: BrowserTransferItem[];
  cancelUpload: (transferId: string) => void;
  retryTransfer: (transferId: string) => void;
  dismissTransfer: (transferId: string) => void;
}

interface DownloadTicketResponse {
  downloadUrl?: string;
  fileName?: string;
}

interface UploadResponsePayload {
  message?: string;
  filename?: string;
  status?: UploadStatus;
}

interface UploadResult {
  status: UploadStatus;
  filename: string;
}

interface ArchiveDownloadJobResponse {
  jobId?: string;
  status?: 'queued' | 'running' | 'ready' | 'failed' | 'expired' | 'canceled';
  fileName?: string;
  sourceCount?: number;
  totalItems?: number;
  processedItems?: number;
  totalSourceBytes?: number;
  processedSourceBytes?: number;
  failureReason?: string;
  artifactSize?: number;
}

interface DownloadTransferOptions {
  loaded?: number;
  total?: number;
  message?: string;
  spaceId?: number;
}

interface UploadExecutionTask {
  transferId: string;
  file: File;
}

interface UploadExecutionResult {
  transferId: string;
  file: File;
  outcome: 'uploaded' | 'skipped' | 'failed' | 'canceled';
  filename?: string;
  message?: string;
}

interface UploadBatchQueueEntry {
  tasks: UploadExecutionTask[];
  targetPath: string;
  settledResults: UploadExecutionResult[];
  resolve: (results: UploadExecutionResult[]) => void;
  reject: (error: unknown) => void;
}

interface ArchiveQueueEntry {
  transferId: string;
  archiveSpaceId: number;
  relativePaths: string[];
  fallbackArchiveName: string;
  resolve: () => void;
  reject: (error: unknown) => void;
}

interface UploadSummary {
  uploaded: number;
  skipped: number;
  failed: number;
}

interface TransferFailurePayload {
  path?: string;
  reason?: string;
  code?: string;
}

interface TransferResponsePayload {
  succeeded?: string[];
  skipped?: string[];
  failed?: TransferFailurePayload[];
}

interface TransferSummary {
  succeeded: number;
  skipped: number;
  failed: number;
}

interface TransferOperationResult {
  summary: TransferSummary;
  succeededSources: string[];
  failedReasons: string[];
  abortedByUser: boolean;
}

interface TransferConflictSelection {
  policy: TransferConflictPolicy;
  applyToRemaining: boolean;
}

interface TrashListResponsePayload {
  items?: TrashItem[];
}

interface TrashRestoreSuccessPayload {
  id?: number;
  originalPath?: string;
}

interface TrashRestoreFailurePayload {
  id?: number;
  originalPath?: string;
  reason?: string;
  code?: string;
}

interface TrashRestoreResponsePayload {
  succeeded?: TrashRestoreSuccessPayload[];
  skipped?: TrashRestoreSuccessPayload[];
  failed?: TrashRestoreFailurePayload[];
}

interface TrashDeleteSuccessPayload {
  id?: number;
}

interface TrashDeleteFailurePayload {
  id?: number;
  reason?: string;
}

interface TrashDeleteResponsePayload {
  succeeded?: TrashDeleteSuccessPayload[];
  failed?: TrashDeleteFailurePayload[];
}

interface TrashEmptyFailurePayload {
  id?: number;
  reason?: string;
}

interface TrashEmptyResponsePayload {
  removed?: number;
  failed?: TrashEmptyFailurePayload[];
}

const MAX_ACTIVE_UPLOAD_BATCHES = 2;
const MAX_ACTIVE_ARCHIVE_TASKS = 2;

function normalizeRelativePath(path: string): string {
  return path.replace(/^\/+/, '').replace(/\/+$/, '');
}

function getParentPath(path: string): string {
  if (!path) return '';
  const normalizedPath = normalizeRelativePath(path);
  if (!normalizedPath) return '';
  const lastSlashIndex = normalizedPath.lastIndexOf('/');
  if (lastSlashIndex <= 0) return '';
  return normalizedPath.slice(0, lastSlashIndex);
}

function createInvalidationTarget(path: string, space?: Space): TreeInvalidationTarget {
  return {
    path,
    spaceId: space?.id,
  };
}

function triggerBrowserDownloadFromUrl(url: string, fileName?: string): void {
  const anchor = document.createElement('a');
  anchor.href = url;
  if (fileName) {
    anchor.download = fileName;
  }
  document.body.appendChild(anchor);
  anchor.click();
  document.body.removeChild(anchor);
}

function wasPageReloaded(): boolean {
  if (typeof window === 'undefined' || typeof performance === 'undefined') {
    return false;
  }
  const navigationEntries = performance.getEntriesByType('navigation') as PerformanceNavigationTiming[];
  if (navigationEntries.length > 0) {
    return navigationEntries[0]?.type === 'reload';
  }
  const legacyNavigation = performance.navigation;
  return typeof legacyNavigation !== 'undefined' && legacyNavigation.type === legacyNavigation.TYPE_RELOAD;
}

function normalizeUploadSource(files: UploadSource): File[] {
  if (files instanceof File) {
    return [files];
  }
  if (Array.isArray(files)) {
    return files;
  }
  return Array.from(files);
}

function isDestinationConflictFailure(item: { code?: string }): boolean {
  return item.code === 'destination_exists';
}

function createTransferId(): string {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
    return crypto.randomUUID();
  }
  return `transfer-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === 'AbortError';
}

export function useFileOperations(selectedPath: string, selectedSpace?: Space): UseFileOperationsReturn {
  const { t } = useTranslation();
  const { message, modal } = App.useApp();
  const content = useBrowseStore((state) => state.content);
  const fetchSpaceContents = useBrowseStore((state) => state.fetchSpaceContents);
  const invalidateTree = useBrowseStore((state) => state.invalidateTree);
  const transfers = useTransferCenterStore((state) => state.transfers);
  const upsertTransfer = useTransferCenterStore((state) => state.upsertTransfer);
  const dismissTransferFromStore = useTransferCenterStore((state) => state.dismissTransfer);
  const uploadAbortControllersRef = useRef<Map<string, AbortController>>(new Map());
  const downloadAbortControllersRef = useRef<Map<string, AbortController>>(new Map());
  const canceledQueuedUploadTransferIdsRef = useRef<Set<string>>(new Set());
  const uploadBatchQueueRef = useRef<UploadBatchQueueEntry[]>([]);
  const activeUploadBatchCountRef = useRef(0);
  const archiveQueueRef = useRef<ArchiveQueueEntry[]>([]);
  const activeArchiveTaskCountRef = useRef(0);
  const initialPersistedTransfersRef = useRef<BrowserTransferItem[] | null>(null);
  const reconciledPersistedTransfersRef = useRef(false);

  if (initialPersistedTransfersRef.current === null) {
    initialPersistedTransfersRef.current = useTransferCenterStore.getState().transfers.map((transfer) => ({ ...transfer }));
  }

  const dismissTransfer = useCallback((transferId: string) => {
    uploadAbortControllersRef.current.get(transferId)?.abort();
    uploadAbortControllersRef.current.delete(transferId);
    downloadAbortControllersRef.current.get(transferId)?.abort();
    downloadAbortControllersRef.current.delete(transferId);
    canceledQueuedUploadTransferIdsRef.current.delete(transferId);
    const archiveTaskIndex = archiveQueueRef.current.findIndex((task) => task.transferId === transferId);
    if (archiveTaskIndex !== -1) {
      archiveQueueRef.current.splice(archiveTaskIndex, 1);
    }
    dismissTransferFromStore(transferId);
  }, [dismissTransferFromStore]);

  const setUploadTransferStatus = useCallback((
    transferId: string,
    name: string,
    status: 'queued' | 'uploading' | 'completed' | 'failed' | 'canceled',
    options: { loaded?: number; total?: number; message?: string } = {}
  ) => {
    const loaded = options.loaded ?? 0;
    const total = options.total ?? 0;
    const progressPercent = total > 0 ? Math.min(100, Math.round((loaded / total) * 100)) : 0;
    upsertTransfer({
      id: transferId,
      kind: 'upload',
      name,
      status,
      spaceId: selectedSpace?.id,
      loaded,
      total,
      progressPercent,
      message: options.message,
    });
  }, [selectedSpace?.id, upsertTransfer]);

  const setArchiveTransferStatus = useCallback((
    transferId: string,
    name: string,
    payload: ArchiveDownloadJobResponse,
    overrideStatus?: BrowserTransferStatus,
    overrideMessage?: string,
    spaceId?: number,
    requestedPaths?: string[]
  ) => {
    const status = overrideStatus ?? payload.status ?? 'queued';
    const currentTransfer = useTransferCenterStore.getState().transfers.find((transfer) => transfer.id === transferId);
    const persistedRequestedPaths = currentTransfer?.kind === 'archive' ? currentTransfer.requestedPaths : undefined;
    upsertTransfer({
      id: transferId,
      kind: 'archive',
      name,
      status,
      spaceId: spaceId ?? selectedSpace?.id,
      jobId: payload.jobId,
      processedItems: payload.processedItems ?? 0,
      totalItems: payload.totalItems ?? 0,
      processedSourceBytes: payload.processedSourceBytes ?? 0,
      totalSourceBytes: payload.totalSourceBytes ?? 0,
      requestedPaths: requestedPaths ?? persistedRequestedPaths,
      message: overrideMessage ?? payload.failureReason,
    });
  }, [selectedSpace?.id, upsertTransfer]);

  const isArchiveTransferCanceled = useCallback((transferId: string): boolean => {
    const transfer = useTransferCenterStore.getState().transfers.find((item) => item.id === transferId);
    return transfer?.kind === 'archive' && transfer.status === 'canceled';
  }, []);

  const readErrorMessage = useCallback(async (response: Response, fallback: string): Promise<string> => {
    try {
      const error = await response.json();
      if (error?.message && typeof error.message === 'string') {
        return error.message;
      }
      if (error?.error && typeof error.error === 'string') {
        return error.error;
      }
    } catch {
      // ignore parse errors and fallback to default message
    }
    return fallback;
  }, []);

  const setDownloadTransferStatus = useCallback((
    transferId: string,
    name: string,
    status: Extract<BrowserTransferStatus, 'running' | 'completed' | 'handed_off' | 'failed' | 'expired' | 'canceled'>,
    options: DownloadTransferOptions = {}
  ) => {
    upsertTransfer({
      id: transferId,
      kind: 'download',
      name,
      status,
      spaceId: options.spaceId ?? selectedSpace?.id,
      loaded: options.loaded,
      total: options.total,
      message: options.message,
    });
  }, [selectedSpace?.id, upsertTransfer]);

  const cancelUpload = useCallback((transferId: string) => {
    const activeUploadController = uploadAbortControllersRef.current.get(transferId);
    if (activeUploadController) {
      activeUploadController.abort();
      return;
    }

    const activeDownloadController = downloadAbortControllersRef.current.get(transferId);
    if (activeDownloadController) {
      activeDownloadController.abort();
      return;
    }

    const activeArchiveTransfer = useTransferCenterStore.getState().transfers.find((transfer) => (
      transfer.id === transferId
      && transfer.kind === 'archive'
      && (transfer.status === 'queued' || transfer.status === 'running')
      && Boolean(transfer.jobId)
      && Boolean(transfer.spaceId)
    ));
    if (activeArchiveTransfer?.kind === 'archive' && activeArchiveTransfer.jobId && activeArchiveTransfer.spaceId) {
      void (async () => {
        const cancelResponse = await apiFetch(
          `/api/spaces/${activeArchiveTransfer.spaceId}/files/archive-downloads?jobId=${encodeURIComponent(activeArchiveTransfer.jobId)}`,
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
      return;
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
      return;
    }

    const queuedUploadTransfer = useTransferCenterStore.getState().transfers.find((transfer) => (
      transfer.id === transferId && transfer.kind === 'upload' && transfer.status === 'queued'
    ));
    if (queuedUploadTransfer) {
      canceledQueuedUploadTransferIdsRef.current.add(transferId);
      setUploadTransferStatus(transferId, queuedUploadTransfer.name, 'canceled', {
        loaded: 0,
        total: queuedUploadTransfer.total,
        message: t('fileOperations.transferCanceled'),
      });
      return;
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
    }
  }, [apiFetch, message, readErrorMessage, setArchiveTransferStatus, setUploadTransferStatus, t]);

  const waitForNextArchivePoll = useCallback(async () => {
    await new Promise((resolve) => window.setTimeout(resolve, 800));
  }, []);

  // 현재 경로로 목록 새로고침 (Space 필수)
  const refreshContents = useCallback(async () => {
    if (!selectedSpace) return;
    await fetchSpaceContents(selectedSpace.id, normalizeRelativePath(selectedPath));
  }, [selectedPath, selectedSpace, fetchSpaceContents]);

  const driveArchiveDownloadFlow = useCallback(async ({
    transferId,
    archiveSpaceId,
    initialJob,
    fallbackName,
    notify,
    requestedPaths,
  }: {
    transferId: string;
    archiveSpaceId: number;
    initialJob: ArchiveDownloadJobResponse;
    fallbackName: string;
    notify: boolean;
    requestedPaths?: string[];
  }): Promise<void> => {
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
  }, [apiFetch, isArchiveTransferCanceled, message, readErrorMessage, setArchiveTransferStatus, t, waitForNextArchivePoll]);

  // 파일 업로드 실행 함수
  const performUpload = useCallback(
    async (
      file: File,
      targetPath: string,
      transferId: string,
      signal: AbortSignal,
      conflictPolicy?: UploadConflictPolicy
    ): Promise<UploadResult> => {
      if (!selectedSpace) throw new Error(t('fileOperations.selectedSpaceRequired'));

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
    [selectedSpace, readErrorMessage, setUploadTransferStatus, t]
  );

  const promptConflictPolicy = useCallback((fileName: string): Promise<UploadConflictPolicy | null> => {
    return new Promise((resolve) => {
      let selectedPolicy: UploadConflictPolicy = 'overwrite';
      let settled = false;
      const settle = (value: UploadConflictPolicy | null) => {
        if (settled) {
          return;
        }
        settled = true;
        resolve(value);
      };

      modal.confirm({
        title: t('fileOperations.uploadConflictTitle'),
        content: (
          <AntSpace direction="vertical" size={12} style={{ width: '100%' }}>
            <Typography.Text>
              {t('fileOperations.uploadConflictFileExists', { fileName })}
            </Typography.Text>
            <Radio.Group
              defaultValue="overwrite"
              onChange={(event) => {
                selectedPolicy = event.target.value as UploadConflictPolicy;
              }}
            >
              <AntSpace direction="vertical" size={8}>
                <Radio value="overwrite">{t('fileOperations.overwrite')}</Radio>
                <Radio value="rename">{t('fileOperations.rename')}</Radio>
                <Radio value="skip">{t('fileOperations.skip')}</Radio>
              </AntSpace>
            </Radio.Group>
            <Typography.Text type="secondary">
              {t('fileOperations.uploadConflictApplyBatch')}
            </Typography.Text>
          </AntSpace>
        ),
        okText: t('fileOperations.apply'),
        cancelText: t('fileOperations.uploadStop'),
        onOk: () => {
          settle(selectedPolicy);
        },
        onCancel: () => {
          settle(null);
        },
      });
    });
  }, [modal, t]);

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

  const pumpUploadQueue = useCallback(() => {
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
  }, [apiFetch, driveArchiveDownloadFlow, readErrorMessage, setArchiveTransferStatus, t]);

  const pumpArchiveQueue = useCallback(() => {
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

  const retryTransfer = useCallback((transferId: string) => {
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

  useEffect(() => {
    if (reconciledPersistedTransfersRef.current) {
      return;
    }
    reconciledPersistedTransfersRef.current = true;
    if (!wasPageReloaded()) {
      return;
    }
    const persistedTransfers = initialPersistedTransfersRef.current ?? [];
    persistedTransfers.forEach((transfer) => {
      if (
        transfer.kind === 'upload'
        && (transfer.status === 'queued' || transfer.status === 'uploading')
      ) {
        const uploadTransfer = transfer as UploadTransferItem;
        upsertTransfer({
          ...uploadTransfer,
          status: 'failed',
          message: t('fileOperations.transferInterruptedOnReload'),
        });
        return;
      }

      if (transfer.kind === 'download' && transfer.status === 'running') {
        const downloadTransfer = transfer as DownloadTransferItem;
        upsertTransfer({
          ...downloadTransfer,
          status: 'failed',
          message: t('fileOperations.transferInterruptedOnReload'),
        });
        return;
      }

      if (
        transfer.kind === 'archive'
        && (transfer.status === 'queued' || transfer.status === 'running' || transfer.status === 'ready')
      ) {
        const archiveTransfer = transfer as ArchiveTransferItem;
        if (!archiveTransfer.jobId || !archiveTransfer.spaceId) {
          upsertTransfer({
            ...archiveTransfer,
            status: 'failed',
            message: t('fileOperations.transferInterruptedOnReload'),
          });
          return;
        }

        void driveArchiveDownloadFlow({
          transferId: archiveTransfer.id,
          archiveSpaceId: archiveTransfer.spaceId,
          initialJob: {
            jobId: archiveTransfer.jobId,
            status: archiveTransfer.status === 'ready' ? 'ready' : archiveTransfer.status,
            fileName: archiveTransfer.name,
            totalItems: archiveTransfer.totalItems,
            processedItems: archiveTransfer.processedItems,
            totalSourceBytes: archiveTransfer.totalSourceBytes,
            processedSourceBytes: archiveTransfer.processedSourceBytes,
          },
          fallbackName: archiveTransfer.name,
          notify: false,
          requestedPaths: archiveTransfer.requestedPaths,
        });
      }
    });
  }, [driveArchiveDownloadFlow, t, upsertTransfer]);

  const performTransferRequest = useCallback(
    async (
      mode: TransferMode,
      sources: string[],
      destination: string,
      destinationSpace?: Space,
      conflictPolicy?: TransferConflictPolicy
    ): Promise<TransferResponsePayload> => {
      if (!selectedSpace) {
        throw new Error(t('fileOperations.selectedSpaceRequired'));
      }

      const dstSpace = destinationSpace ?? selectedSpace;
      const relativeSources = sources.map((source) => normalizeRelativePath(source));
      const relativeDestination = normalizeRelativePath(destination);
      const payload: {
        sources: string[];
        destination: { spaceId: number; path: string };
        conflictPolicy?: TransferConflictPolicy;
      } = {
        sources: relativeSources,
        destination: {
          spaceId: dstSpace.id,
          path: relativeDestination,
        },
      };

      if (conflictPolicy) {
        payload.conflictPolicy = conflictPolicy;
      }

      const response = await apiFetch(`/api/spaces/${selectedSpace.id}/files/${mode}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });

      if (!response.ok) {
        const errorMessage = await readErrorMessage(
          response,
          mode === 'move' ? t('fileOperations.moveFailed') : t('fileOperations.copyFailed')
        );
        throw new Error(errorMessage);
      }

      return (await response.json()) as TransferResponsePayload;
    },
    [selectedSpace, readErrorMessage, t]
  );

  const promptTransferConflictSelection = useCallback((
    mode: TransferMode,
    sourcePath: string,
    remainingConflictCount: number
  ): Promise<TransferConflictSelection | null> => {
    return new Promise((resolve) => {
      let selectedPolicy: TransferConflictPolicy = 'overwrite';
      let applyToRemaining = remainingConflictCount > 0;
      let settled = false;
      const settle = (value: TransferConflictSelection | null) => {
        if (settled) {
          return;
        }
        settled = true;
        resolve(value);
      };
      const fileName = sourcePath.split('/').filter(Boolean).pop() ?? sourcePath;
      const transferVerb = mode === 'move' ? t('fileOperations.moveVerb') : t('fileOperations.copyVerb');

      modal.confirm({
        title: t('fileOperations.conflictTitle'),
        content: (
          <AntSpace direction="vertical" size={12} style={{ width: '100%' }}>
            <Typography.Text>
              {t('fileOperations.conflictItemExists', { fileName })}
            </Typography.Text>
            <Typography.Text type="secondary">
              {t('fileOperations.conflictChoose')}
            </Typography.Text>
            <Radio.Group
              defaultValue="overwrite"
              onChange={(event) => {
                selectedPolicy = event.target.value as TransferConflictPolicy;
              }}
            >
              <AntSpace direction="vertical" size={8}>
                <Radio value="overwrite">{t('fileOperations.overwrite')}</Radio>
                <Radio value="rename">{t('fileOperations.rename')}</Radio>
                <Radio value="skip">{t('fileOperations.skip')}</Radio>
              </AntSpace>
            </Radio.Group>
            <Typography.Text type="secondary">
              {t('fileOperations.conflictCurrentItem', {
                mode: transferVerb,
                current: remainingConflictCount + 1,
              })}
            </Typography.Text>
            {remainingConflictCount > 0 ? (
              <Checkbox
                defaultChecked
                onChange={(event) => {
                  applyToRemaining = event.target.checked;
                }}
              >
                {t('fileOperations.conflictApplyRemaining', { count: remainingConflictCount })}
              </Checkbox>
            ) : null}
          </AntSpace>
        ),
        okText: t('fileOperations.apply'),
        cancelText: t('fileOperations.stopAction', { mode: transferVerb }),
        onOk: () => {
          settle({ policy: selectedPolicy, applyToRemaining });
        },
        onCancel: () => {
          settle(null);
        },
      });
    });
  }, [modal, t]);

  const promptTrashConflictPolicy = useCallback((conflictCount: number): Promise<TrashConflictPolicy | null> => {
    return new Promise((resolve) => {
      let selectedPolicy: TrashConflictPolicy = 'overwrite';
      let settled = false;
      const settle = (value: TrashConflictPolicy | null) => {
        if (settled) {
          return;
        }
        settled = true;
        resolve(value);
      };

      modal.confirm({
        title: t('trashExplorer.conflictPolicyTitle'),
        content: (
          <AntSpace direction="vertical" size={12} style={{ width: '100%' }}>
            <Typography.Text>
              {t('trashExplorer.conflictDetected', { count: conflictCount })}
            </Typography.Text>
            <Typography.Text type="secondary">
              {t('trashExplorer.conflictPolicyDescription')}
            </Typography.Text>
            <Radio.Group
              defaultValue="overwrite"
              onChange={(event) => {
                selectedPolicy = event.target.value as TrashConflictPolicy;
              }}
            >
              <AntSpace direction="vertical" size={8}>
                <Radio value="overwrite">{t('trashExplorer.overwrite')}</Radio>
                <Radio value="rename">{t('trashExplorer.rename')}</Radio>
                <Radio value="skip">{t('trashExplorer.skip')}</Radio>
              </AntSpace>
            </Radio.Group>
          </AntSpace>
        ),
        okText: t('trashExplorer.apply'),
        cancelText: t('trashExplorer.stopRestore'),
        onOk: () => {
          settle(selectedPolicy);
        },
        onCancel: () => {
          settle(null);
        },
      });
    });
  }, [modal, t]);

  const performTrashRestoreRequest = useCallback(async (
    ids: number[],
    conflictPolicy?: TrashConflictPolicy
  ): Promise<TrashRestoreResponsePayload> => {
    if (!selectedSpace) {
      throw new Error(t('fileOperations.selectedSpaceRequired'));
    }

    const payload: { ids: number[]; conflictPolicy?: TrashConflictPolicy } = { ids };
    if (conflictPolicy) {
      payload.conflictPolicy = conflictPolicy;
    }

    const response = await apiFetch(`/api/spaces/${selectedSpace.id}/files/trash-restore`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });

    if (!response.ok) {
      const errorMessage = await readErrorMessage(response, t('fileOperations.trashRestoreFailed'));
      throw new Error(errorMessage);
    }

    return (await response.json()) as TrashRestoreResponsePayload;
  }, [readErrorMessage, selectedSpace, t]);

  const summarizeTrashRestoreFailure = useCallback((item: TrashRestoreFailurePayload): string => {
    const targetPath = item.originalPath ?? `#${item.id ?? 'unknown'}`;
    return `${targetPath}: ${item.reason ?? t('fileOperations.trashRestoreItemFailed')}`;
  }, [t]);

  const executeTransfer = useCallback(async (
    mode: TransferMode,
    sources: string[],
    destination: string,
    destinationSpace?: Space
  ): Promise<TransferOperationResult> => {
    const relativeSources = sources.map((source) => normalizeRelativePath(source));
    const summary: TransferSummary = { succeeded: 0, skipped: 0, failed: 0 };
    const succeededSources: string[] = [];
    const failedReasons: string[] = [];

    if (relativeSources.length === 0) {
      return { summary, succeededSources, failedReasons, abortedByUser: false };
    }

    const initialResult = await performTransferRequest(mode, relativeSources, destination, destinationSpace);
    const initialSucceeded = initialResult.succeeded ?? [];
    const initialSkipped = initialResult.skipped ?? [];
    const initialFailed = initialResult.failed ?? [];

    summary.succeeded += initialSucceeded.length;
    summary.skipped += initialSkipped.length;
    summary.failed += initialFailed.filter((item) => !isDestinationConflictFailure(item)).length;
    succeededSources.push(...initialSucceeded);

    initialFailed
      .filter((item) => !isDestinationConflictFailure(item))
      .forEach((item) => {
        const failurePath = item.path ?? t('fileOperations.unknownPath');
        failedReasons.push(
          `${failurePath}: ${item.reason ?? (mode === 'move' ? t('fileOperations.moveFailed') : t('fileOperations.copyFailed'))}`
        );
      });

    const conflictQueue = initialFailed
      .filter(isDestinationConflictFailure)
      .map((item) => item.path)
      .filter((path): path is string => typeof path === 'string' && path.length > 0);

    let abortedByUser = false;
    while (conflictQueue.length > 0) {
      const currentSource = conflictQueue.shift();
      if (!currentSource) {
        continue;
      }

      const conflictSelection = await promptTransferConflictSelection(mode, currentSource, conflictQueue.length);
      if (!conflictSelection) {
        const unresolvedSources = [currentSource, ...conflictQueue];
        summary.failed += unresolvedSources.length;
        unresolvedSources.forEach((sourcePath) => {
          failedReasons.push(`${sourcePath}: ${t('fileOperations.unresolvedByUser')}`);
        });
        conflictQueue.length = 0;
        abortedByUser = true;
        break;
      }

      if (conflictSelection.applyToRemaining) {
        const batchSources = [currentSource, ...conflictQueue];
        conflictQueue.length = 0;

        const retriedBatch = await performTransferRequest(
          mode,
          batchSources,
          destination,
          destinationSpace,
          conflictSelection.policy
        );

        const retriedSucceeded = retriedBatch.succeeded ?? [];
        const retriedSkipped = retriedBatch.skipped ?? [];
        const retriedFailed = retriedBatch.failed ?? [];

        summary.succeeded += retriedSucceeded.length;
        summary.skipped += retriedSkipped.length;
        summary.failed += retriedFailed.length;
        succeededSources.push(...retriedSucceeded);

        retriedFailed.forEach((item) => {
          const failurePath = item.path ?? t('fileOperations.unknownPath');
          failedReasons.push(
            `${failurePath}: ${item.reason ?? (mode === 'move' ? t('fileOperations.moveFailed') : t('fileOperations.copyFailed'))}`
          );
        });
        continue;
      }

      const retriedSingle = await performTransferRequest(
        mode,
        [currentSource],
        destination,
        destinationSpace,
        conflictSelection.policy
      );
      const retriedSucceeded = retriedSingle.succeeded ?? [];
      const retriedSkipped = retriedSingle.skipped ?? [];
      const retriedFailed = retriedSingle.failed ?? [];

      summary.succeeded += retriedSucceeded.length;
      summary.skipped += retriedSkipped.length;
      summary.failed += retriedFailed.length;
      succeededSources.push(...retriedSucceeded);

      retriedFailed.forEach((item) => {
        const failurePath = item.path ?? t('fileOperations.unknownPath');
        failedReasons.push(
          `${failurePath}: ${item.reason ?? (mode === 'move' ? t('fileOperations.moveFailed') : t('fileOperations.copyFailed'))}`
        );
      });
    }

    return { summary, succeededSources, failedReasons, abortedByUser };
  }, [performTransferRequest, promptTransferConflictSelection, t]);

  // 파일 업로드 처리 (다중 업로드 + 충돌 정책 일괄 적용)
  const handleFileUpload = useCallback(
    async (files: UploadSource, targetPath: string) => {
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
    },
    [enqueueUploadBatch, refreshContents, message, setUploadTransferStatus, t]
  );

  // 이름 변경 처리
  const handleRename = useCallback(
    async (oldPath: string, newName: string) => {
      if (!newName.trim()) {
        message.error(t('folderContent.renameRequired'));
        return;
      }
      if (!selectedSpace) {
        message.error(t('fileOperations.selectedSpaceRequired'));
        return;
      }

      try {
        const response = await apiFetch(`/api/spaces/${selectedSpace.id}/files/rename`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            path: normalizeRelativePath(oldPath),
            newName: newName.trim(),
          }),
        });

        if (!response.ok) {
          const error = await response.json();
          throw new Error(error.message || t('fileOperations.renameFailed'));
        }

        message.success(t('fileOperations.renameSuccess'));
        await refreshContents();
        invalidateTree([createInvalidationTarget(getParentPath(oldPath), selectedSpace)]);
      } catch (error) {
        message.error(error instanceof Error ? error.message : t('fileOperations.renameFailed'));
      }
    },
    [selectedSpace, refreshContents, message, invalidateTree, t]
  );

  // 새 폴더 만들기 처리
  const handleCreateFolder = useCallback(
    async (parentPath: string, folderName: string) => {
      if (!folderName.trim()) {
        message.error(t('folderContent.folderNameRequired'));
        return;
      }
      if (!selectedSpace) {
        message.error(t('fileOperations.selectedSpaceRequired'));
        return;
      }

      try {
        const response = await apiFetch(`/api/spaces/${selectedSpace.id}/files/create-folder`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            parentPath: normalizeRelativePath(parentPath),
            folderName: folderName.trim(),
          }),
        });

        if (!response.ok) {
          const error = await response.json();
          throw new Error(error.message || t('fileOperations.createFolderFailed'));
        }

        message.success(t('fileOperations.createFolderSuccess'));
        await refreshContents();
        invalidateTree([createInvalidationTarget(parentPath, selectedSpace)]);
      } catch (error) {
        message.error(error instanceof Error ? error.message : t('fileOperations.createFolderFailed'));
      }
    },
    [selectedSpace, refreshContents, message, invalidateTree, t]
  );

  // 다중 다운로드 처리
  const handleBulkDownload = useCallback(
    async (paths: string[]) => {
      if (paths.length === 0) return;
      if (!selectedSpace) {
        message.error(t('fileOperations.selectedSpaceRequired'));
        return;
      }

      try {
        const relativePaths = paths.map((path) => normalizeRelativePath(path));
        const singleItem = relativePaths.length === 1
          ? content.find((item) => normalizeRelativePath(item.path) === relativePaths[0])
          : undefined;
        const useArchiveDownload = relativePaths.length > 1 || Boolean(singleItem?.isDir);

        if (!useArchiveDownload) {
          const transferId = createTransferId();
          const downloadName = singleItem?.name ?? relativePaths[0].split('/').pop() ?? t('fileOperations.archiveFallbackName');
          const abortController = new AbortController();
          downloadAbortControllersRef.current.set(transferId, abortController);
          setDownloadTransferStatus(transferId, downloadName, 'running', {
            spaceId: selectedSpace.id,
          });
          try {
            const ticketResponse = await apiFetch(`/api/spaces/${selectedSpace.id}/files/download-ticket`, {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ path: relativePaths[0] }),
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
            return;
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
        }

        const transferId = createTransferId();
        const fallbackArchiveName = singleItem?.isDir
          ? `${singleItem.name}.zip`
          : t('fileOperations.archiveFallbackName');
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
      } catch (error) {
        message.error(error instanceof Error ? error.message : t('fileOperations.downloadFailed'));
      }
    },
    [
      content,
      enqueueArchiveTask,
      message,
      selectedSpace,
      setArchiveTransferStatus,
      setDownloadTransferStatus,
      t,
    ]
  );

  // 다중 휴지통 이동 처리
  const handleBulkDelete = useCallback(
    async (paths: string[]) => {
      if (paths.length === 0) return;

      modal.confirm({
        title: t('fileOperations.moveToTrashConfirmTitle'),
        content: t('fileOperations.moveToTrashBulkConfirmContent', { count: paths.length }),
        okText: t('folderContent.move'),
        okType: 'danger',
        cancelText: t('folderContent.cancel'),
        onOk: async () => {
          if (!selectedSpace) {
            message.error(t('fileOperations.selectedSpaceRequired'));
            return;
          }
          try {
            const relativePaths = paths.map(p => normalizeRelativePath(p));
            const response = await apiFetch(`/api/spaces/${selectedSpace.id}/files/delete-multiple`, {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ paths: relativePaths }),
            });

            if (!response.ok) {
              const error = await response.json();
              throw new Error(error.message || t('fileOperations.moveToTrashFailed'));
            }

            const result = await response.json();
            const succeededCount = result.succeeded?.length || 0;
            const failedCount = result.failed?.length || 0;

            if (failedCount > 0) {
              message.warning(t('fileOperations.moveToTrashBulkPartial', { succeeded: succeededCount, failed: failedCount }));
            } else {
              message.success(t('fileOperations.moveToTrashBulkSuccess', { count: succeededCount }));
            }

            await refreshContents();
            invalidateTree(paths.map((path) => createInvalidationTarget(getParentPath(path), selectedSpace)));
          } catch (error) {
            message.error(error instanceof Error ? error.message : t('fileOperations.moveToTrashFailed'));
          }
        },
      });
    },
    [selectedSpace, refreshContents, message, modal, invalidateTree, t]
  );

  const fetchTrashItems = useCallback(async (): Promise<TrashItem[]> => {
    if (!selectedSpace) {
      throw new Error(t('fileOperations.selectedSpaceRequired'));
    }

    const response = await apiFetch(`/api/spaces/${selectedSpace.id}/files/trash`, {
      method: 'GET',
    });
    if (!response.ok) {
      const errorMessage = await readErrorMessage(response, t('fileOperations.trashListLoadFailed'));
      throw new Error(errorMessage);
    }

    const payload = (await response.json()) as TrashListResponsePayload;
    return Array.isArray(payload.items) ? payload.items : [];
  }, [readErrorMessage, selectedSpace, t]);

  const handleTrashRestore = useCallback(async (ids: number[]) => {
    if (ids.length === 0) {
      return;
    }

    try {
      const firstResult = await performTrashRestoreRequest(ids);
      const initialSucceeded = firstResult.succeeded ?? [];
      const initialSkipped = firstResult.skipped ?? [];
      const initialFailed = firstResult.failed ?? [];

      let succeededCount = initialSucceeded.length;
      let skippedCount = initialSkipped.length;
      const failedReasons: string[] = initialFailed
        .filter((item) => !isDestinationConflictFailure(item))
        .map(summarizeTrashRestoreFailure);

      const conflictItems = initialFailed.filter(isDestinationConflictFailure);
      if (conflictItems.length > 0) {
        const conflictIds: number[] = [];
        conflictItems.forEach((item) => {
          if (typeof item.id === 'number') {
            conflictIds.push(item.id);
            return;
          }
          failedReasons.push(summarizeTrashRestoreFailure(item));
        });

        if (conflictIds.length > 0) {
          const conflictPolicy = await promptTrashConflictPolicy(conflictIds.length);
          if (!conflictPolicy) {
            failedReasons.push(
              ...conflictItems
                .filter((item) => typeof item.id === 'number')
                .map(summarizeTrashRestoreFailure)
            );
            const unresolvedCount = conflictItems.length;
            const summaryMessage = t('fileOperations.restoreSummaryWithUnresolved', {
              succeeded: succeededCount,
              skipped: skippedCount,
              failed: failedReasons.length,
              unresolved: unresolvedCount,
            });
            message.warning(summaryMessage);
            return;
          }

          const retriedResult = await performTrashRestoreRequest(conflictIds, conflictPolicy);
          const retriedSucceeded = retriedResult.succeeded ?? [];
          const retriedSkipped = retriedResult.skipped ?? [];
          const retriedFailed = retriedResult.failed ?? [];

          succeededCount += retriedSucceeded.length;
          skippedCount += retriedSkipped.length;
          failedReasons.push(...retriedFailed.map(summarizeTrashRestoreFailure));
        }
      }

      const failedCount = failedReasons.length;
      const summaryMessage = t('fileOperations.restoreSummary', {
        succeeded: succeededCount,
        skipped: skippedCount,
        failed: failedCount,
      });
      if (failedCount > 0) {
        const firstFailure = failedReasons[0] ? ` - ${failedReasons[0]}` : '';
        message.warning(`${summaryMessage}${firstFailure}`);
        return;
      }
      message.success(summaryMessage);
    } catch (error) {
      message.error(error instanceof Error ? error.message : t('fileOperations.trashRestoreFailed'));
    }
  }, [message, performTrashRestoreRequest, promptTrashConflictPolicy, summarizeTrashRestoreFailure, t]);

  const handleTrashDelete = useCallback(async (ids: number[]) => {
    if (ids.length === 0) {
      return;
    }

    if (!selectedSpace) {
      message.error(t('fileOperations.selectedSpaceRequired'));
      return;
    }

    try {
      const response = await apiFetch(`/api/spaces/${selectedSpace.id}/files/trash-delete`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ids }),
      });
      if (!response.ok) {
        const errorMessage = await readErrorMessage(response, t('fileOperations.trashDeleteFailed'));
        throw new Error(errorMessage);
      }

      const payload = (await response.json()) as TrashDeleteResponsePayload;
      const succeededCount = payload.succeeded?.length ?? 0;
      const failed = payload.failed ?? [];
      if (failed.length > 0) {
        const firstFailure = failed[0]?.reason ? ` - ${failed[0].reason}` : '';
        message.warning(`${t('fileOperations.trashDeleteSummary', {
          succeeded: succeededCount,
          failed: failed.length,
        })}${firstFailure}`);
        return;
      }

      message.success(t('fileOperations.trashDeleteSuccess', { count: succeededCount }));
    } catch (error) {
      message.error(error instanceof Error ? error.message : t('fileOperations.trashDeleteFailed'));
    }
  }, [message, readErrorMessage, selectedSpace, t]);

  const handleTrashEmpty = useCallback(async () => {
    if (!selectedSpace) {
      message.error(t('fileOperations.selectedSpaceRequired'));
      return;
    }

    try {
      const response = await apiFetch(`/api/spaces/${selectedSpace.id}/files/trash-empty`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
      });
      if (!response.ok) {
        const errorMessage = await readErrorMessage(response, t('fileOperations.trashEmptyFailed'));
        throw new Error(errorMessage);
      }

      const payload = (await response.json()) as TrashEmptyResponsePayload;
      const removed = typeof payload.removed === 'number' ? payload.removed : 0;
      const failed = payload.failed ?? [];
      if (failed.length > 0) {
        const firstFailure = failed[0]?.reason ? ` - ${failed[0].reason}` : '';
        message.warning(`${t('fileOperations.trashEmptySummary', {
          removed,
          failed: failed.length,
        })}${firstFailure}`);
        return;
      }
      message.success(t('fileOperations.trashEmptySuccess', { count: removed }));
    } catch (error) {
      message.error(error instanceof Error ? error.message : t('fileOperations.trashEmptyFailed'));
    }
  }, [message, readErrorMessage, selectedSpace, t]);

  // 이동 처리 (cross-Space 지원)
  const handleMove = useCallback(
    async (sources: string[], destination: string, destinationSpace?: Space) => {
      if (sources.length === 0) return;
      if (!selectedSpace) {
        message.error(t('fileOperations.selectedSpaceRequired'));
        return;
      }

      const dstSpace = destinationSpace ?? selectedSpace;

      try {
        const { summary, succeededSources, failedReasons, abortedByUser } = await executeTransfer(
          'move',
          sources,
          destination,
          destinationSpace
        );

        const shouldRefresh = summary.succeeded > 0 || summary.failed > 0;
        if (shouldRefresh) {
          await refreshContents();
          invalidateTree([
            ...succeededSources.map((source) => createInvalidationTarget(getParentPath(source), selectedSpace)),
            createInvalidationTarget(destination, dstSpace),
          ]);
        }

        const summaryMessage = t('fileOperations.transferSummary', {
          mode: t('fileOperations.moveVerb'),
          succeeded: summary.succeeded,
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
      } catch (error) {
        message.error(error instanceof Error ? error.message : t('fileOperations.moveFailed'));
      }
    },
    [selectedSpace, refreshContents, message, invalidateTree, executeTransfer, t]
  );

  // 복사 처리 (cross-Space 지원)
  const handleCopy = useCallback(
    async (sources: string[], destination: string, destinationSpace?: Space) => {
      if (sources.length === 0) return;
      if (!selectedSpace) {
        message.error(t('fileOperations.selectedSpaceRequired'));
        return;
      }

      const dstSpace = destinationSpace ?? selectedSpace;

      try {
        const { summary, failedReasons, abortedByUser } = await executeTransfer(
          'copy',
          sources,
          destination,
          destinationSpace
        );

        const shouldRefresh = summary.succeeded > 0 || summary.failed > 0;
        if (shouldRefresh) {
          await refreshContents();
          invalidateTree([createInvalidationTarget(destination, dstSpace)]);
        }

        const summaryMessage = t('fileOperations.transferSummary', {
          mode: t('fileOperations.copyVerb'),
          succeeded: summary.succeeded,
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
      } catch (error) {
        message.error(error instanceof Error ? error.message : t('fileOperations.copyFailed'));
      }
    },
    [selectedSpace, refreshContents, message, invalidateTree, executeTransfer, t]
  );

  // 단일 휴지통 이동 처리
  const handleDelete = useCallback(
    async (record: { path: string; name: string; isDir: boolean }) => {
      const singleDeleteContent = record.isDir
        ? `${t('fileOperations.moveToTrashSingleConfirmContent', { name: record.name })} (${t('fileOperations.moveToTrashFolderNotice')})`
        : t('fileOperations.moveToTrashSingleConfirmContent', { name: record.name });
      modal.confirm({
        title: t('fileOperations.moveToTrashConfirmTitle'),
        content: singleDeleteContent,
        okText: t('folderContent.move'),
        okType: 'danger',
        cancelText: t('folderContent.cancel'),
        onOk: async () => {
          if (!selectedSpace) {
            message.error(t('fileOperations.selectedSpaceRequired'));
            return;
          }
          try {
            const response = await apiFetch(`/api/spaces/${selectedSpace.id}/files/delete`, {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ path: normalizeRelativePath(record.path) }),
            });

            if (!response.ok) {
              const error = await response.json();
              throw new Error(error.message || t('fileOperations.moveToTrashFailed'));
            }

            message.success(t('fileOperations.movedToTrashSuccess'));
            await refreshContents();
            invalidateTree([createInvalidationTarget(getParentPath(record.path), selectedSpace)]);
          } catch (error) {
            message.error(error instanceof Error ? error.message : t('fileOperations.moveToTrashFailed'));
          }
        },
      });
    },
    [selectedSpace, refreshContents, message, modal, invalidateTree, t]
  );

  return {
    handleRename,
    handleCreateFolder,
    handleDelete,
    handleBulkDelete,
    fetchTrashItems,
    handleTrashRestore,
    handleTrashDelete,
    handleTrashEmpty,
    handleMove,
    handleCopy,
    handleBulkDownload,
    handleFileUpload,
    transfers,
    cancelUpload,
    retryTransfer,
    dismissTransfer,
  };
}
