import { useState, useCallback } from 'react';
import type { FileNode } from '../types';

interface UseFileSelectionReturn {
  selectedItems: Set<string>;
  lastSelectedIndex: number;
  handleItemClick: (e: React.MouseEvent, record: FileNode, index: number, sortedContent: FileNode[]) => void;
  handleContainerClick: (e: React.MouseEvent) => void;
  clearSelection: () => void;
  setSelection: (items: Set<string>) => void;
}

export function useFileSelection(): UseFileSelectionReturn {
  const [selectedItems, setSelectedItems] = useState<Set<string>>(new Set());
  const [lastSelectedIndex, setLastSelectedIndex] = useState<number>(-1);

  const handleItemClick = useCallback(
    (e: React.MouseEvent, record: FileNode, index: number, sortedContent: FileNode[]) => {
      // Ctrl/Cmd + 클릭: 토글
      if (e.ctrlKey || e.metaKey) {
        e.preventDefault();
        const newSelected = new Set(selectedItems);
        if (newSelected.has(record.path)) {
          newSelected.delete(record.path);
        } else {
          newSelected.add(record.path);
        }
        setSelectedItems(newSelected);
        setLastSelectedIndex(index);
      }
      // Shift + 클릭: 범위 선택
      else if (e.shiftKey && lastSelectedIndex >= 0) {
        e.preventDefault();
        const start = Math.min(lastSelectedIndex, index);
        const end = Math.max(lastSelectedIndex, index);
        const newSelected = new Set(selectedItems);
        for (let i = start; i <= end; i++) {
          if (sortedContent[i]) {
            newSelected.add(sortedContent[i].path);
          }
        }
        setSelectedItems(newSelected);
      }
      // 일반 클릭: 단일 선택
      else {
        setSelectedItems(new Set([record.path]));
        setLastSelectedIndex(index);
      }
    },
    [selectedItems, lastSelectedIndex]
  );

  const handleContainerClick = useCallback((e: React.MouseEvent) => {
    const target = e.target as HTMLElement;
    // 카드나 테이블 행을 클릭하지 않은 경우
    const isCard = target.closest('.ant-card');
    const isTableRow = target.closest('tr');
    if (!isCard && !isTableRow) {
      setSelectedItems(new Set());
      setLastSelectedIndex(-1);
    }
  }, []);

  const clearSelection = useCallback(() => {
    setSelectedItems(new Set());
    setLastSelectedIndex(-1);
  }, []);

  const setSelection = useCallback((items: Set<string>) => {
    setSelectedItems(items);
  }, []);

  return {
    selectedItems,
    lastSelectedIndex,
    handleItemClick,
    handleContainerClick,
    clearSelection,
    setSelection,
  };
}
