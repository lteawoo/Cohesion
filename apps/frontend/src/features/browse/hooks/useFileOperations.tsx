import { useCallback } from 'react';
import { App, Checkbox, Radio, Space as AntSpace, Typography } from 'antd';
import { useBrowseStore } from '@/stores/browseStore';
import { useTransferCenterStore } from '@/stores/transferCenterStore';
import type {
  BrowserTransferItem,
  BrowserTransferStatus,
} from '@/stores/transferCenterStore';
import type { Space } from '@/features/space/types';
import { apiFetch } from '@/api/client';
import { useTranslation } from 'react-i18next';
import { useArchiveTransfers } from './useArchiveTransfers';
import { useDirectDownloadTransfers } from './useDirectDownloadTransfers';
import {
  createInvalidationTarget,
  getParentPath,
  isDestinationConflictFailure,
  normalizeRelativePath,
  type ArchiveDownloadJobResponse,
  type DownloadTransferOptions,
  type TransferConflictSelection,
  type TransferConflictPolicy,
  type TransferMode,
  type TransferOperationResult,
  type TransferResponsePayload,
  type TrashConflictPolicy,
  type TrashDeleteResponsePayload,
  type TrashEmptyResponsePayload,
  type TrashItem,
  type TrashListResponsePayload,
  type TrashRestoreFailurePayload,
  type TrashRestoreResponsePayload,
  type UploadConflictPolicy,
  type UploadSource,
} from './transferOperationsShared';
import { useTransferHydration } from './useTransferHydration';
import { useUploadTransfers } from './useUploadTransfers';

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

export function useFileOperations(selectedPath: string, selectedSpace?: Space): UseFileOperationsReturn {
  const { t } = useTranslation();
  const { message, modal } = App.useApp();
  const content = useBrowseStore((state) => state.content);
  const fetchSpaceContents = useBrowseStore((state) => state.fetchSpaceContents);
  const invalidateTree = useBrowseStore((state) => state.invalidateTree);
  const transfers = useTransferCenterStore((state) => state.transfers);
  const upsertTransfer = useTransferCenterStore((state) => state.upsertTransfer);
  const dismissTransferFromStore = useTransferCenterStore((state) => state.dismissTransfer);
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

  const refreshContents = useCallback(async () => {
    if (!selectedSpace) return;
    await fetchSpaceContents(selectedSpace.id, normalizeRelativePath(selectedPath));
  }, [selectedPath, selectedSpace, fetchSpaceContents]);

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

  const {
    handleFileUpload: handleQueuedUpload,
    cancelUploadTransfer,
    dismissUploadTransfer,
  } = useUploadTransfers({
    selectedSpace,
    t,
    message,
    refreshContents,
    promptConflictPolicy,
    readErrorMessage,
    setUploadTransferStatus,
  });

  const {
    startDirectDownload,
    cancelDirectDownload,
    dismissDirectDownload,
  } = useDirectDownloadTransfers({
    selectedSpace,
    t,
    readErrorMessage,
    setDownloadTransferStatus,
  });

  const {
    enqueueArchiveDownload,
    cancelArchiveTransfer,
    dismissArchiveTransfer,
    retryArchiveTransfer,
    resumeArchiveTransfer,
  } = useArchiveTransfers({
    selectedSpace,
    t,
    message,
    readErrorMessage,
    setArchiveTransferStatus,
  });

  useTransferHydration({
    t,
    upsertTransfer,
    resumeArchiveTransfer,
  });

  const dismissTransfer = useCallback((transferId: string) => {
    dismissUploadTransfer(transferId);
    dismissDirectDownload(transferId);
    dismissArchiveTransfer(transferId);
    dismissTransferFromStore(transferId);
  }, [
    dismissArchiveTransfer,
    dismissDirectDownload,
    dismissTransferFromStore,
    dismissUploadTransfer,
  ]);

  const cancelUpload = useCallback((transferId: string) => {
    if (cancelUploadTransfer(transferId)) {
      return;
    }
    if (cancelDirectDownload(transferId)) {
      return;
    }
    cancelArchiveTransfer(transferId);
  }, [cancelArchiveTransfer, cancelDirectDownload, cancelUploadTransfer]);

  const retryTransfer = useCallback((transferId: string) => {
    retryArchiveTransfer(transferId);
  }, [retryArchiveTransfer]);

  const handleFileUpload = useCallback(async (files: UploadSource, targetPath: string) => {
    await handleQueuedUpload(files, targetPath);
  }, [handleQueuedUpload]);

  const handleBulkDownload = useCallback(
    async (paths: string[]) => {
      if (paths.length === 0) {
        return;
      }
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
          const downloadName = singleItem?.name
            ?? relativePaths[0].split('/').pop()
            ?? t('fileOperations.archiveFallbackName');
          await startDirectDownload(relativePaths[0], downloadName);
          return;
        }

        const fallbackArchiveName = singleItem?.isDir
          ? `${singleItem.name}.zip`
          : t('fileOperations.archiveFallbackName');
        await enqueueArchiveDownload(relativePaths, fallbackArchiveName);
      } catch (error) {
        message.error(error instanceof Error ? error.message : t('fileOperations.downloadFailed'));
      }
    },
    [content, enqueueArchiveDownload, message, selectedSpace, startDirectDownload, t]
  );

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
          const errorMessage = await readErrorMessage(response, t('fileOperations.renameFailed'));
          throw new Error(errorMessage);
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
          const errorMessage = await readErrorMessage(response, t('fileOperations.createFolderFailed'));
          throw new Error(errorMessage);
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
              const errorMessage = await readErrorMessage(response, t('fileOperations.moveToTrashFailed'));
              throw new Error(errorMessage);
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
              const errorMessage = await readErrorMessage(response, t('fileOperations.moveToTrashFailed'));
              throw new Error(errorMessage);
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
