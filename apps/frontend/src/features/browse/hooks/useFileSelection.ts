import { useState, useCallback, useRef } from 'react';
import type { FileNode } from '../types';

interface UseFileSelectionReturn {
  selectedItems: Set<string>;
  handleItemClick: (e: React.MouseEvent, record: FileNode, index: number, sortedContent: FileNode[]) => void;
  handleContainerClick: (e: React.MouseEvent) => void;
  clearSelection: () => void;
  setSelection: (items: Set<string>, index?: number) => void;
}

export function useFileSelection(): UseFileSelectionReturn {
  const [selectedItems, setSelectedItems] = useState<Set<string>>(new Set());
  const lastSelectedIndexRef = useRef<number>(-1);

  const handleItemClick = useCallback(
    (e: React.MouseEvent, record: FileNode, index: number, sortedContent: FileNode[]) => {
      const isToggle = e.ctrlKey || e.metaKey;
      const isRange = e.shiftKey;

      // Shift + 클릭: 앵커 기준 범위 선택 (기본은 범위로 치환, Ctrl/Cmd+Shift는 범위 추가)
      if (isRange) {
        e.preventDefault();
        const anchor = lastSelectedIndexRef.current >= 0 ? lastSelectedIndexRef.current : index;
        const start = Math.min(anchor, index);
        const end = Math.max(anchor, index);
        const rangeItems = new Set<string>();
        for (let i = start; i <= end; i++) {
          if (sortedContent[i]) {
            rangeItems.add(sortedContent[i].path);
          }
        }

        if (isToggle) {
          setSelectedItems((prev) => new Set([...prev, ...rangeItems]));
        } else {
          setSelectedItems(rangeItems);
        }

        // Shift 확장 시 anchor는 유지하고, 마지막 포커스 위치만 업데이트
        return;
      }

      // Ctrl/Cmd + 클릭: 토글
      if (isToggle) {
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
        return;
      }

      // 일반 클릭: 단일 선택
      {
        setSelectedItems(new Set([record.path]));
        lastSelectedIndexRef.current = index;
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
    }
  }, []);

  const clearSelection = useCallback(() => {
    setSelectedItems(new Set());
    lastSelectedIndexRef.current = -1;
  }, []);

  const setSelection = useCallback((items: Set<string>, index?: number) => {
    setSelectedItems(items);
    if (typeof index === 'number' && index >= 0) {
      lastSelectedIndexRef.current = index;
      return;
    }

    if (items.size === 0) {
      lastSelectedIndexRef.current = -1;
    }
  }, []);

  return {
    selectedItems,
    handleItemClick,
    handleContainerClick,
    clearSelection,
    setSelection,
  };
}
