import { useState, useCallback, useRef } from 'react';
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
  const lastSelectedIndexRef = useRef<number>(-1);

  const handleItemClick = useCallback(
    (e: React.MouseEvent, record: FileNode, index: number, sortedContent: FileNode[]) => {
      // Ctrl/Cmd + 클릭: 토글
      if (e.ctrlKey || e.metaKey) {
        e.preventDefault();
        setSelectedItems((prev) => {
          const next = new Set(prev);
          if (next.has(record.path)) {
            next.delete(record.path);
          } else {
            next.add(record.path);
          }
          return next;
        });
        lastSelectedIndexRef.current = index;
        setLastSelectedIndex(index);
      }
      // Shift + 클릭: 범위 선택
      else if (e.shiftKey && lastSelectedIndexRef.current >= 0) {
        e.preventDefault();
        const anchor = lastSelectedIndexRef.current;
        const start = Math.min(anchor, index);
        const end = Math.max(anchor, index);
        setSelectedItems((prev) => {
          const next = new Set(prev);
          for (let i = start; i <= end; i++) {
            if (sortedContent[i]) {
              next.add(sortedContent[i].path);
            }
          }
          return next;
        });
        lastSelectedIndexRef.current = index;
        setLastSelectedIndex(index);
      }
      // 일반 클릭: 단일 선택
      else {
        setSelectedItems(new Set([record.path]));
        lastSelectedIndexRef.current = index;
        setLastSelectedIndex(index);
      }
    },
    []
  );

  const handleContainerClick = useCallback((e: React.MouseEvent) => {
    const target = e.target as HTMLElement;
    // 카드나 테이블 행을 클릭하지 않은 경우
    const isCard = target.closest('.ant-card');
    const isTableRow = target.closest('tr');
    if (!isCard && !isTableRow) {
      setSelectedItems(new Set());
      lastSelectedIndexRef.current = -1;
      setLastSelectedIndex(-1);
    }
  }, []);

  const clearSelection = useCallback(() => {
    setSelectedItems(new Set());
    lastSelectedIndexRef.current = -1;
    setLastSelectedIndex(-1);
  }, []);

  const setSelection = useCallback((items: Set<string>) => {
    setSelectedItems(items);
    if (items.size === 0) {
      lastSelectedIndexRef.current = -1;
      setLastSelectedIndex(-1);
    }
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
