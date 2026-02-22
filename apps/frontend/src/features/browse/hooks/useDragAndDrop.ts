import { useState, useCallback } from 'react';
import { App } from 'antd';
import type { FileNode, DragData } from '../types';

interface UseDragAndDropParams {
  onMove: (sources: string[], destination: string) => Promise<void>;
  onFileUpload: (files: File[], targetPath: string) => Promise<void>;
  selectedItems: Set<string>;
  currentPath: string;
}

interface UseDragAndDropReturn {
  isDragging: boolean;
  dragOverFolder: string | null;
  handleItemDragStart: (e: React.DragEvent, record: FileNode) => void;
  handleItemDragEnd: (e: React.DragEvent) => void;
  handleFolderDragOver: (e: React.DragEvent, folder: FileNode) => void;
  handleFolderDragLeave: (e: React.DragEvent) => void;
  handleFolderDrop: (e: React.DragEvent, folder: FileNode) => void;
  handleDragEnter: (e: React.DragEvent) => void;
  handleDragLeave: (e: React.DragEvent) => void;
  handleDragOver: (e: React.DragEvent) => void;
  handleDrop: (e: React.DragEvent) => void;
}

export function useDragAndDrop({
  onMove,
  onFileUpload,
  selectedItems,
  currentPath,
}: UseDragAndDropParams): UseDragAndDropReturn {
  const { modal } = App.useApp();
  const [isDragging, setIsDragging] = useState(false);
  const [dragOverFolder, setDragOverFolder] = useState<string | null>(null);

  const confirmAndMove = useCallback(
    (sourcePaths: string[], destinationPath: string) => {
      modal.confirm({
        title: '이동 확인',
        content: `선택한 ${sourcePaths.length}개 항목을 이동하시겠습니까?`,
        okText: '이동',
        cancelText: '취소',
        onOk: async () => {
          await onMove(sourcePaths, destinationPath);
        },
      });
    },
    [modal, onMove]
  );

  // 아이템 드래그 시작 핸들러
  const handleItemDragStart = useCallback(
    (e: React.DragEvent, record: FileNode) => {
      // 드래그 시작 시 선택 처리
      const newSelectedItems = new Set(selectedItems);
      if (!newSelectedItems.has(record.path)) {
        // 선택되지 않은 항목을 드래그하면 해당 항목만 선택
        newSelectedItems.clear();
        newSelectedItems.add(record.path);
      }

      // dataTransfer에 경로 목록 저장
      const dragData: DragData = {
        type: 'cohesion-internal',
        paths: Array.from(newSelectedItems),
      };
      e.dataTransfer.setData('application/json', JSON.stringify(dragData));
      e.dataTransfer.effectAllowed = 'move';
    },
    [selectedItems]
  );

  // 아이템 드래그 종료 핸들러
  const handleItemDragEnd = useCallback(() => {
    setDragOverFolder(null);
  }, []);

  // 폴더 위 드래그 오버 핸들러
  const handleFolderDragOver = useCallback((e: React.DragEvent, folder: FileNode) => {
    if (!folder.isDir) return;

    e.preventDefault();
    e.stopPropagation();

    // 외부 파일인지 내부 이동인지 확인
    const hasFiles = e.dataTransfer.types.includes('Files');
    if (hasFiles) {
      // 외부 파일은 폴더 드롭 불가
      return;
    }

    e.dataTransfer.dropEffect = 'move';
    setDragOverFolder(folder.path);
  }, []);

  // 폴더 위 드래그 떠남 핸들러
  const handleFolderDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setDragOverFolder(null);
  }, []);

  // 폴더 위 드롭 핸들러
  const handleFolderDrop = useCallback(
    async (e: React.DragEvent, folder: FileNode) => {
      if (!folder.isDir) return;

      e.preventDefault();
      e.stopPropagation();
      setDragOverFolder(null);

      try {
        const dataText = e.dataTransfer.getData('application/json');
        if (dataText) {
          const dragData: DragData = JSON.parse(dataText);
          if (dragData.type === 'cohesion-internal') {
            const sourcePaths = dragData.paths;

            // 자기 자신으로 이동 방지
            if (sourcePaths.includes(folder.path)) {
              return;
            }

            // 해당 폴더로 이동
            confirmAndMove(sourcePaths, folder.path);
          }
        }
      } catch {
        // Error handled silently
      }
    },
    [confirmAndMove]
  );

  // 드래그 이벤트 핸들러 (외부 파일 업로드용)
  const handleDragEnter = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();

    // 외부 파일만 isDragging 상태 활성화
    const hasFiles = e.dataTransfer.types.includes('Files');
    if (hasFiles) {
      setIsDragging(true);
    }
  }, []);

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    // 자식 요소로 이동할 때는 무시
    if (e.currentTarget === e.target) {
      setIsDragging(false);
    }
  }, []);

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
  }, []);

  const handleDrop = useCallback(
    async (e: React.DragEvent) => {
      e.preventDefault();
      e.stopPropagation();
      setIsDragging(false);

      // 1. 외부 파일 업로드 체크
      const files = Array.from(e.dataTransfer.files);
      if (files.length > 0) {
        await onFileUpload(files, currentPath);
        return;
      }

      // 2. 내부 파일/폴더 이동 (빈 영역에 드롭 시 현재 폴더에 이동)
      try {
        const dataText = e.dataTransfer.getData('application/json');
        if (dataText) {
          const dragData: DragData = JSON.parse(dataText);
          if (dragData.type === 'cohesion-internal') {
            // 같은 폴더에 드롭하면 무시
            const sourcePaths = dragData.paths;
            const allInSameFolder = sourcePaths.every((p) => {
              const parentPath = p.substring(0, p.lastIndexOf('/'));
              return parentPath === currentPath;
            });
            if (!allInSameFolder) {
              // 현재 폴더로 이동
              confirmAndMove(sourcePaths, currentPath);
            }
          }
        }
      } catch {
        // JSON 파싱 실패 시 무시
      }
    },
    [confirmAndMove, onFileUpload, currentPath]
  );

  return {
    isDragging,
    dragOverFolder,
    handleItemDragStart,
    handleItemDragEnd,
    handleFolderDragOver,
    handleFolderDragLeave,
    handleFolderDrop,
    handleDragEnter,
    handleDragLeave,
    handleDragOver,
    handleDrop,
  };
}
