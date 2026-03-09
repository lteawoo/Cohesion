import { create } from 'zustand';
import { createJSONStorage, persist } from 'zustand/middleware';
import type { StateStorage } from 'zustand/middleware';

export type BrowserTransferStatus =
  | 'uploading'
  | 'queued'
  | 'running'
  | 'ready'
  | 'completed'
  | 'handed_off'
  | 'failed'
  | 'expired'
  | 'canceled';

export type BrowserTransferKind = 'upload' | 'archive' | 'download';
interface BaseTransferItem {
  id: string;
  kind: BrowserTransferKind;
  name: string;
  status: BrowserTransferStatus;
  message?: string;
  updatedAt: number;
  spaceId?: number;
}

export interface UploadTransferItem extends BaseTransferItem {
  kind: 'upload';
  loaded: number;
  total: number;
  progressPercent: number;
}

export interface ArchiveTransferItem extends BaseTransferItem {
  kind: 'archive';
  jobId?: string;
  processedItems: number;
  totalItems: number;
  processedSourceBytes: number;
  totalSourceBytes: number;
}

export interface DownloadTransferItem extends BaseTransferItem {
  kind: 'download';
  loaded?: number;
  total?: number;
}

export type BrowserTransferItem = UploadTransferItem | ArchiveTransferItem | DownloadTransferItem;
type TransferInsert = Omit<BrowserTransferItem, 'updatedAt'> & { updatedAt?: number };

const ACTIVE_TRANSFER_STATUSES: BrowserTransferStatus[] = ['uploading', 'queued', 'running'];
const CLEARABLE_TRANSFER_STATUSES: BrowserTransferStatus[] = ['completed', 'handed_off'];
const TERMINAL_HISTORY_LIMIT = 24;
const TRANSFER_CENTER_STORAGE_KEY = 'cohesion-transfer-center-v1';

const noopStorage: StateStorage = {
  getItem: () => null,
  setItem: () => undefined,
  removeItem: () => undefined,
};

function isActiveStatus(status: BrowserTransferStatus): boolean {
  return ACTIVE_TRANSFER_STATUSES.includes(status);
}

function isClearableStatus(status: BrowserTransferStatus): boolean {
  return CLEARABLE_TRANSFER_STATUSES.includes(status);
}

function orderTransfers(transfers: BrowserTransferItem[]): BrowserTransferItem[] {
  return [...transfers].sort((left, right) => {
    const leftGroup = isActiveStatus(left.status) ? 0 : 1;
    const rightGroup = isActiveStatus(right.status) ? 0 : 1;
    if (leftGroup !== rightGroup) {
      return leftGroup - rightGroup;
    }
    return right.updatedAt - left.updatedAt;
  });
}

function applyTerminalRetention(transfers: BrowserTransferItem[]): BrowserTransferItem[] {
  const active = transfers.filter((transfer) => isActiveStatus(transfer.status));
  const terminal = transfers.filter((transfer) => !isActiveStatus(transfer.status));
  const overflow = terminal.length - TERMINAL_HISTORY_LIMIT;

  if (overflow <= 0) {
    return orderTransfers([...active, ...terminal]);
  }

  const evictionCandidates = [...terminal]
    .filter((transfer) => isClearableStatus(transfer.status))
    .sort((left, right) => left.updatedAt - right.updatedAt)
    .slice(0, overflow);
  const evictedIds = new Set(evictionCandidates.map((transfer) => transfer.id));

  return orderTransfers([
    ...active,
    ...terminal.filter((transfer) => !evictedIds.has(transfer.id)),
  ]);
}

interface TransferCenterStore {
  isOpen: boolean;
  transfers: BrowserTransferItem[];
  upsertTransfer: (transfer: TransferInsert) => void;
  dismissTransfer: (transferId: string) => void;
  clearCompletedTransfers: () => void;
  setOpen: (open: boolean) => void;
  toggleOpen: () => void;
  reset: () => void;
}

type PersistedTransferCenterState = Pick<TransferCenterStore, 'transfers'>;

export function isActiveTransferStatus(status: BrowserTransferStatus): boolean {
  return isActiveStatus(status);
}

export function isClearableTransferStatus(status: BrowserTransferStatus): boolean {
  return isClearableStatus(status);
}

export const useTransferCenterStore = create<TransferCenterStore>()(
  persist(
    (set) => ({
      isOpen: false,
      transfers: [],

      upsertTransfer: (transfer) => {
        set((state) => {
          const nextTransfer: BrowserTransferItem = {
            ...transfer,
            updatedAt: transfer.updatedAt ?? Date.now(),
          } as BrowserTransferItem;
          const transferIndex = state.transfers.findIndex((item) => item.id === transfer.id);
          const nextTransfers = transferIndex === -1
            ? [nextTransfer, ...state.transfers]
            : state.transfers.map((item, index) => (index === transferIndex ? nextTransfer : item));

          return {
            isOpen: transferIndex === -1 ? true : state.isOpen,
            transfers: applyTerminalRetention(nextTransfers),
          };
        });
      },

      dismissTransfer: (transferId) => {
        set((state) => {
          const nextTransfers = state.transfers.filter((transfer) => transfer.id !== transferId);
          return {
            isOpen: nextTransfers.length === 0 ? false : state.isOpen,
            transfers: nextTransfers,
          };
        });
      },

      clearCompletedTransfers: () => {
        set((state) => {
          const nextTransfers = state.transfers.filter((transfer) => !isClearableStatus(transfer.status));
          return {
            isOpen: nextTransfers.length === 0 ? false : state.isOpen,
            transfers: nextTransfers,
          };
        });
      },

      setOpen: (open) => {
        set({ isOpen: open });
      },

      toggleOpen: () => {
        set((state) => ({ isOpen: !state.isOpen }));
      },

      reset: () => {
        set({ isOpen: false, transfers: [] });
      },
    }),
    {
      name: TRANSFER_CENTER_STORAGE_KEY,
      storage: createJSONStorage<PersistedTransferCenterState>(() => (
        typeof window === 'undefined' ? noopStorage : window.sessionStorage
      )),
      partialize: (state) => ({
        transfers: state.transfers,
      }),
      merge: (persistedState, currentState) => {
        const persistedTransfers = (persistedState as PersistedTransferCenterState | undefined)?.transfers ?? [];
        return {
          ...currentState,
          transfers: applyTerminalRetention(persistedTransfers),
        };
      },
    }
  )
);
