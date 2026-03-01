import { useCallback, useEffect, useState } from 'react';
import { App } from 'antd';
import type { Space } from '@/features/space/types';
import type { TrashItem } from './useFileOperations';
import { useTranslation } from 'react-i18next';

interface TrashOpenRequest {
  spaceId: number;
  nonce: number;
}

interface UseTrashModalManagerParams {
  selectedSpace?: Space;
  isSearchMode: boolean;
  trashOpenRequest: TrashOpenRequest | null;
  clearTrashOpenRequest: () => void;
  fetchTrashItems: () => Promise<TrashItem[]>;
  handleTrashRestore: (ids: number[]) => Promise<void>;
  handleTrashDelete: (ids: number[]) => Promise<void>;
  handleTrashEmpty: () => Promise<void>;
  refreshCurrentFolder: () => Promise<void>;
}

interface UseTrashModalManagerResult {
  isTrashModalOpen: boolean;
  trashItems: TrashItem[];
  selectedTrashIds: number[];
  isTrashLoading: boolean;
  isTrashProcessing: boolean;
  setSelectedTrashIds: (ids: number[]) => void;
  handleOpenTrash: () => void;
  handleCloseTrash: () => void;
  handleTrashRestoreConfirm: () => void;
  handleTrashDeleteConfirm: () => void;
  handleTrashEmptyConfirm: () => void;
}

export function useTrashModalManager({
  selectedSpace,
  isSearchMode,
  trashOpenRequest,
  clearTrashOpenRequest,
  fetchTrashItems,
  handleTrashRestore,
  handleTrashDelete,
  handleTrashEmpty,
  refreshCurrentFolder,
}: UseTrashModalManagerParams): UseTrashModalManagerResult {
  const { t } = useTranslation();
  const { message, modal } = App.useApp();
  const [isTrashModalOpen, setIsTrashModalOpen] = useState(false);
  const [trashItems, setTrashItems] = useState<TrashItem[]>([]);
  const [selectedTrashIds, setSelectedTrashIds] = useState<number[]>([]);
  const [isTrashLoading, setIsTrashLoading] = useState(false);
  const [isTrashProcessing, setIsTrashProcessing] = useState(false);

  const loadTrashItems = useCallback(async () => {
    if (!selectedSpace) {
      return;
    }
    setIsTrashLoading(true);
    try {
      const items = await fetchTrashItems();
      setTrashItems(items);
    } catch (error) {
      message.error(error instanceof Error ? error.message : t('folderContent.trashListLoadFailed'));
    } finally {
      setIsTrashLoading(false);
    }
  }, [fetchTrashItems, message, selectedSpace, t]);

  const handleOpenTrash = useCallback(() => {
    if (!selectedSpace) {
      message.error(t('folderContent.noSelectedSpace'));
      return;
    }
    setIsTrashModalOpen(true);
    setSelectedTrashIds([]);
    void loadTrashItems();
  }, [loadTrashItems, message, selectedSpace, t]);

  const handleCloseTrash = useCallback(() => {
    if (isTrashProcessing) {
      return;
    }
    setIsTrashModalOpen(false);
    setSelectedTrashIds([]);
  }, [isTrashProcessing]);

  const handleTrashRestoreConfirm = useCallback(() => {
    if (selectedTrashIds.length === 0) {
      message.warning(t('folderContent.selectRestoreItems'));
      return;
    }

    modal.confirm({
      title: t('folderContent.restoreConfirmTitle'),
      content: t('folderContent.restoreConfirmContent', { count: selectedTrashIds.length }),
      okText: t('folderContent.restore'),
      cancelText: t('folderContent.cancel'),
      onOk: async () => {
        setIsTrashProcessing(true);
        try {
          await handleTrashRestore(selectedTrashIds);
          await loadTrashItems();
          await refreshCurrentFolder();
          setSelectedTrashIds([]);
        } finally {
          setIsTrashProcessing(false);
        }
      },
    });
  }, [
    handleTrashRestore,
    loadTrashItems,
    message,
    modal,
    refreshCurrentFolder,
    selectedTrashIds,
    t,
  ]);

  const handleTrashDeleteConfirm = useCallback(() => {
    if (selectedTrashIds.length === 0) {
      message.warning(t('folderContent.selectPermanentDeleteItems'));
      return;
    }

    modal.confirm({
      title: t('folderContent.permanentDeleteConfirmTitle'),
      content: t('folderContent.permanentDeleteConfirmContent', { count: selectedTrashIds.length }),
      okText: t('folderContent.permanentDelete'),
      okType: 'danger',
      cancelText: t('folderContent.cancel'),
      onOk: async () => {
        setIsTrashProcessing(true);
        try {
          await handleTrashDelete(selectedTrashIds);
          await loadTrashItems();
          await refreshCurrentFolder();
          setSelectedTrashIds([]);
        } finally {
          setIsTrashProcessing(false);
        }
      },
    });
  }, [
    handleTrashDelete,
    loadTrashItems,
    message,
    modal,
    refreshCurrentFolder,
    selectedTrashIds,
    t,
  ]);

  const handleTrashEmptyConfirm = useCallback(() => {
    if (trashItems.length === 0) {
      message.info(t('folderContent.trashEmptyInfo'));
      return;
    }

    modal.confirm({
      title: t('folderContent.emptyTrashConfirmTitle'),
      content: t('folderContent.emptyTrashConfirmContent', { count: trashItems.length }),
      okText: t('folderContent.emptyTrash'),
      okType: 'danger',
      cancelText: t('folderContent.cancel'),
      onOk: async () => {
        setIsTrashProcessing(true);
        try {
          await handleTrashEmpty();
          await loadTrashItems();
          await refreshCurrentFolder();
          setSelectedTrashIds([]);
        } finally {
          setIsTrashProcessing(false);
        }
      },
    });
  }, [
    handleTrashEmpty,
    loadTrashItems,
    message,
    modal,
    refreshCurrentFolder,
    t,
    trashItems.length,
  ]);

  useEffect(() => {
    if (!selectedSpace || isSearchMode) {
      setIsTrashModalOpen(false);
      setTrashItems([]);
      setSelectedTrashIds([]);
    }
  }, [isSearchMode, selectedSpace]);

  useEffect(() => {
    if (!trashOpenRequest || !selectedSpace) {
      return;
    }
    if (trashOpenRequest.spaceId !== selectedSpace.id) {
      return;
    }
    handleOpenTrash();
    clearTrashOpenRequest();
  }, [clearTrashOpenRequest, handleOpenTrash, selectedSpace, trashOpenRequest]);

  return {
    isTrashModalOpen,
    trashItems,
    selectedTrashIds,
    isTrashLoading,
    isTrashProcessing,
    setSelectedTrashIds,
    handleOpenTrash,
    handleCloseTrash,
    handleTrashRestoreConfirm,
    handleTrashDeleteConfirm,
    handleTrashEmptyConfirm,
  };
}
