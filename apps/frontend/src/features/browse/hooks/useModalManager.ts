import { useState, useCallback } from 'react';
import type { FileNode } from '../types';

// Modal 타입 정의
export interface RenameModalData {
  record?: FileNode;
  newName: string;
}

export interface CreateFolderModalData {
  folderName: string;
}

export interface DestinationModalData {
  mode: 'move' | 'copy';
  sources: string[];
}

// Modal 상태 타입
interface ModalState<T> {
  visible: boolean;
  data: T;
}

// 전체 Modal Registry
interface ModalRegistry {
  rename: ModalState<RenameModalData>;
  createFolder: ModalState<CreateFolderModalData>;
  destination: ModalState<DestinationModalData>;
}

// Modal 이름 타입
type ModalName = keyof ModalRegistry;

// 기본값 정의
const defaultModalData: {
  [K in ModalName]: ModalRegistry[K]['data'];
} = {
  rename: { newName: '' },
  createFolder: { folderName: '' },
  destination: { mode: 'move', sources: [] },
};

export function useModalManager() {
  const [modals, setModals] = useState<ModalRegistry>({
    rename: { visible: false, data: defaultModalData.rename },
    createFolder: { visible: false, data: defaultModalData.createFolder },
    destination: { visible: false, data: defaultModalData.destination },
  });

  const openModal = useCallback(<K extends ModalName>(
    name: K,
    data?: Partial<ModalRegistry[K]['data']>
  ) => {
    setModals((prev) => ({
      ...prev,
      [name]: {
        visible: true,
        data: { ...defaultModalData[name], ...data },
      },
    }));
  }, []);

  const closeModal = useCallback(<K extends ModalName>(name: K) => {
    setModals((prev) => ({
      ...prev,
      [name]: {
        visible: false,
        data: defaultModalData[name],
      },
    }));
  }, []);

  const updateModalData = useCallback(<K extends ModalName>(
    name: K,
    data: Partial<ModalRegistry[K]['data']>
  ) => {
    setModals((prev) => ({
      ...prev,
      [name]: {
        ...prev[name],
        data: { ...prev[name].data, ...data },
      },
    }));
  }, []);

  return {
    modals,
    openModal,
    closeModal,
    updateModalData,
  };
}
