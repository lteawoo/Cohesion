import { useCallback } from 'react';
import { useContextMenuStore } from '@/stores/contextMenuStore';
import { buildSingleItemMenu, buildMultiItemMenu, buildEmptyAreaMenu, type ContextMenuCallbacks } from '../constants';
import type { FileNode } from '../types';
import { useTranslation } from 'react-i18next';

interface UseContextMenuParams {
  selectedItems: Set<string>;
  sortedContent: FileNode[];
  canWriteFiles: boolean;
  onSetSelection: (items: Set<string>, index: number) => void;
  callbacks: ContextMenuCallbacks;
}

interface UseContextMenuReturn {
  handleContextMenu: (e: React.MouseEvent, record: FileNode) => void;
  handleEmptyAreaContextMenu: (e: React.MouseEvent) => void;
}

export function useContextMenu({
  selectedItems,
  sortedContent,
  canWriteFiles,
  onSetSelection,
  callbacks,
}: UseContextMenuParams): UseContextMenuReturn {
  const { t } = useTranslation();
  const translate = useCallback(
    (key: string, options?: Record<string, unknown>) => String(t(key, options)),
    [t]
  );
  const openContextMenu = useContextMenuStore((state) => state.openContextMenu);

  const handleContextMenu = useCallback(
    (e: React.MouseEvent, record: FileNode) => {
      e.preventDefault();

      // 우클릭한 항목이 선택된 항목에 포함되어 있고, 다중 선택 상태인 경우
      const isSelectedItem = selectedItems.has(record.path);
      const isMultiSelect = selectedItems.size > 1;

      if (isSelectedItem && isMultiSelect) {
        // 다중 선택 메뉴
        const menuItems = buildMultiItemMenu(selectedItems.size, {
          onBulkDownload: callbacks.onBulkDownload,
          onCopy: callbacks.onCopy,
          onMove: callbacks.onMove,
          onBulkDelete: callbacks.onBulkDelete,
        }, translate, { canWriteFiles });

        openContextMenu(e.clientX, e.clientY, menuItems);
      } else if (isSelectedItem && selectedItems.size === 1) {
        // 단일 선택 메뉴 (선택된 항목 우클릭)
        const menuItems = buildSingleItemMenu(record, callbacks, translate, { canWriteFiles });
        openContextMenu(e.clientX, e.clientY, menuItems);
      } else {
        // 선택되지 않은 항목 우클릭 - 해당 항목만 선택하고 단일 메뉴 표시
        const index = sortedContent.findIndex((item) => item.path === record.path);
        onSetSelection(new Set([record.path]), index);

        const menuItems = buildSingleItemMenu(record, callbacks, translate, { canWriteFiles });
        openContextMenu(e.clientX, e.clientY, menuItems);
      }
    },
    [selectedItems, sortedContent, canWriteFiles, onSetSelection, callbacks, openContextMenu, translate]
  );

  const handleEmptyAreaContextMenu = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault();
      // 빈 영역 클릭 감지: 카드나 테이블 행이 아닌 경우
      const target = e.target as HTMLElement;
      const isCard = target.closest('.ant-card');
      const isTableRow = target.closest('tr');

      // 카드나 테이블 행을 클릭하지 않은 경우 빈 영역 메뉴 표시
      if (!isCard && !isTableRow) {
        const emptyAreaMenuItems = buildEmptyAreaMenu(callbacks.onCreateFolder, translate, { canWriteFiles });
        if (!emptyAreaMenuItems || emptyAreaMenuItems.length === 0) {
          return;
        }
        openContextMenu(e.clientX, e.clientY, emptyAreaMenuItems);
      }
    },
    [callbacks, canWriteFiles, openContextMenu, translate]
  );

  return {
    handleContextMenu,
    handleEmptyAreaContextMenu,
  };
}
