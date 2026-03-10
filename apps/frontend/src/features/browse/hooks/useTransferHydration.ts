import { useEffect, useRef } from 'react';
import { useTransferCenterStore } from '@/stores/transferCenterStore';
import type {
  ArchiveTransferItem,
  BrowserTransferItem,
  DownloadTransferItem,
  UploadTransferItem,
} from '@/stores/transferCenterStore';
import { wasPageReloaded, type Translate } from './transferOperationsShared';

interface UseTransferHydrationParams {
  t: Translate;
  upsertTransfer: (transfer: BrowserTransferItem) => void;
  resumeArchiveTransfer: (transfer: ArchiveTransferItem, notify: boolean) => void;
}

export function useTransferHydration({
  t,
  upsertTransfer,
  resumeArchiveTransfer,
}: UseTransferHydrationParams): void {
  const initialPersistedTransfersRef = useRef<BrowserTransferItem[] | null>(null);
  const reconciledPersistedTransfersRef = useRef(false);

  if (initialPersistedTransfersRef.current === null) {
    initialPersistedTransfersRef.current = useTransferCenterStore.getState().transfers.map((transfer) => ({ ...transfer }));
  }

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

        resumeArchiveTransfer(archiveTransfer, false);
      }
    });
  }, [resumeArchiveTransfer, t, upsertTransfer]);
}
