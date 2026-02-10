import React, { useEffect, useState, useRef, useMemo } from 'react';
import { Empty, message } from 'antd';
import { useBrowseStore } from '@/stores/browseStore';
import type { FileNode, ViewMode, SortConfig } from '../types';
import { buildTableColumns } from '../constants';
import { useFileSelection } from '../hooks/useFileSelection';
import { useBreadcrumb } from '../hooks/useBreadcrumb';
import { useFileOperations } from '../hooks/useFileOperations';
import { useDragAndDrop } from '../hooks/useDragAndDrop';
import { useContextMenu } from '../hooks/useContextMenu';
import { useBoxSelection } from '../hooks/useBoxSelection';
import { useModalManager } from '../hooks/useModalManager';
import { useSortedContent } from '../hooks/useSortedContent';
import DestinationPickerModal from './DestinationPickerModal';
import FolderContentToolbar from './FolderContent/FolderContentToolbar';
import FolderContentSelectionBar from './FolderContent/FolderContentSelectionBar';
import FolderContentTable from './FolderContent/FolderContentTable';
import FolderContentGrid from './FolderContent/FolderContentGrid';
import RenameModal from './FolderContent/RenameModal';
import CreateFolderModal from './FolderContent/CreateFolderModal';
import UploadOverlay from './FolderContent/UploadOverlay';
import BoxSelectionOverlay from './FolderContent/BoxSelectionOverlay';

const FolderContent: React.FC = () => {
  // Store selectors
  const selectedPath = useBrowseStore((state) => state.selectedPath);
  const selectedSpace = useBrowseStore((state) => state.selectedSpace);
  const content = useBrowseStore((state) => state.content);
  const isLoading = useBrowseStore((state) => state.isLoading);
  const setPath = useBrowseStore((state) => state.setPath);

  console.log('[FolderContent] Render - selectedPath:', selectedPath);

  const fileInputRef = useRef<HTMLInputElement>(null);
  const gridContainerRef = useRef<HTMLDivElement>(null);
  const itemsRef = useRef<Map<string, HTMLElement>>(new Map());

  // Local state
  const [viewMode, setViewMode] = useState<ViewMode>('grid');
  const [sortConfig, setSortConfig] = useState<SortConfig>({
    sortBy: 'name',
    sortOrder: 'ascend',
  });

  // Modal management
  const { modals, openModal, closeModal, updateModalData } = useModalManager();

  // Custom hooks
  const { selectedItems, handleItemClick, setSelection, clearSelection } = useFileSelection();

  const {
    handleRename: performRename,
    handleCreateFolder: performCreateFolder,
    handleDelete,
    handleBulkDelete,
    handleMove,
    handleCopy,
    handleBulkDownload,
    handleFileUpload,
  } = useFileOperations(selectedPath);

  const {
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
  } = useDragAndDrop({
    onMove: handleMove,
    onFileUpload: handleFileUpload,
    selectedItems,
    currentPath: selectedPath,
  });

  const { breadcrumbItems } = useBreadcrumb({
    selectedPath,
    selectedSpace,
    onNavigate: setPath,
  });

  const { handleContextMenu, handleEmptyAreaContextMenu } = useContextMenu({
    selectedItems,
    sortedContent: content,
    onSetSelection: setSelection,
    callbacks: {
      onDownload: (path: string) => {
        window.location.href = `/api/browse/download?path=${encodeURIComponent(path)}`;
      },
      onCopy: () => openModal('destination', { mode: 'copy' }),
      onMove: () => openModal('destination', { mode: 'move' }),
      onRename: (record: FileNode) => {
        openModal('rename', { record, newName: record.name });
      },
      onDelete: handleDelete,
      onBulkDownload: () => handleBulkDownload(Array.from(selectedItems)),
      onBulkDelete: () => handleBulkDelete(Array.from(selectedItems)),
    },
  });

  useEffect(() => {
    console.log('[FolderContent] useEffect triggered - selectedPath:', selectedPath);
    if (selectedPath) {
      console.log('[FolderContent] Fetching contents for:', selectedPath);
      useBrowseStore.getState().fetchDirectoryContents(selectedPath);
    }
    // 경로 변경 시 선택 해제
    clearSelection();
  }, [selectedPath, clearSelection]);

  // 정렬된 콘텐츠 (폴더 우선 + sortConfig)
  const sortedContent = useSortedContent(content, sortConfig);

  // Columns for table view
  const columns = useMemo(() => buildTableColumns(setPath), [setPath]);

  // Box selection (Grid 뷰 전용)
  const { isSelecting, selectionBox, wasRecentlySelecting } = useBoxSelection({
    enabled: viewMode === 'grid',
    containerRef: gridContainerRef,
    itemsRef,
    selectedItems,
    onSelectionChange: setSelection,
  });

  // itemsRef 메모리 정리 (sortedContent 변경 시)
  useEffect(() => {
    const currentPaths = new Set(sortedContent.map(item => item.path));
    const itemsMap = itemsRef.current;

    // Map에서 더 이상 존재하지 않는 path 제거
    itemsMap.forEach((_, path) => {
      if (!currentPaths.has(path)) {
        itemsMap.delete(path);
      }
    });
  }, [sortedContent]);

  // Modal wrapper handlers
  const handleRenameConfirm = async () => {
    const { record, newName } = modals.rename.data;
    if (!record || !newName.trim()) {
      message.error('새 이름을 입력하세요');
      return;
    }
    await performRename(record.path, newName.trim());
    closeModal('rename');
    clearSelection();
  };

  const handleCreateFolderConfirm = async () => {
    const { folderName } = modals.createFolder.data;
    if (!folderName.trim()) {
      message.error('폴더 이름을 입력하세요');
      return;
    }
    await performCreateFolder(selectedPath, folderName.trim());
    closeModal('createFolder');
  };

  const handleMoveConfirm = async (destination: string) => {
    await handleMove(Array.from(selectedItems), destination);
    closeModal('destination');
    clearSelection();
  };

  const handleCopyConfirm = async (destination: string) => {
    await handleCopy(Array.from(selectedItems), destination);
    closeModal('destination');
    clearSelection();
  };

  // File input handlers
  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files;
    if (files && files.length > 0) {
      handleFileUpload(files[0], selectedPath);
      e.target.value = '';
    }
  };

  const handleUploadClick = () => {
    fileInputRef.current?.click();
  };

  // Container click handler
  const handleContainerClick = (e: React.MouseEvent) => {
    // 박스 선택 직후에는 무시
    if (wasRecentlySelecting) {
      return;
    }

    const target = e.target as HTMLElement;
    const isCard = target.closest('.ant-card');
    const isTableRow = target.closest('tr');
    const isButton = target.closest('button');
    const isInput = target.closest('input');

    // 카드, 테이블 행, 버튼, 입력 필드가 아닌 빈 영역만 선택 해제
    if (!isCard && !isTableRow && !isButton && !isInput) {
      clearSelection();
    }
  };

  // Early return if no path selected
  if (!selectedPath) {
    return (
      <div style={{ height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <Empty description="왼쪽 트리나 스페이스에서 폴더를 선택하세요." />
      </div>
    );
  }

  return (
    <div
      style={{ display: 'flex', flexDirection: 'column', gap: '16px', position: 'relative', height: '100%' }}
      onDragEnter={handleDragEnter}
      onDragLeave={handleDragLeave}
      onDragOver={handleDragOver}
      onDrop={handleDrop}
      onContextMenu={handleEmptyAreaContextMenu}
      onClick={handleContainerClick}
    >
      <input
        ref={fileInputRef}
        type="file"
        style={{ display: 'none' }}
        onChange={handleFileSelect}
      />

      <FolderContentToolbar
        breadcrumbItems={breadcrumbItems}
        viewMode={viewMode}
        sortConfig={sortConfig}
        onUpload={handleUploadClick}
        onViewModeChange={setViewMode}
        onSortChange={setSortConfig}
      />

      <FolderContentSelectionBar
        selectedCount={selectedItems.size}
        showRename={selectedItems.size === 1}
        onDownload={() => handleBulkDownload(Array.from(selectedItems))}
        onCopy={() => openModal('destination', { mode: 'copy' })}
        onMove={() => openModal('destination', { mode: 'move' })}
        onRename={() => {
          const path = Array.from(selectedItems)[0];
          const record = sortedContent.find(item => item.path === path);
          if (record) {
            openModal('rename', { record, newName: record.name });
          }
        }}
        onDelete={() => handleBulkDelete(Array.from(selectedItems))}
        onClear={clearSelection}
      />

      {viewMode === 'table' ? (
        <FolderContentTable
          dataSource={sortedContent}
          columns={columns}
          loading={isLoading}
          selectedItems={selectedItems}
          dragOverFolder={dragOverFolder}
          onSelectionChange={setSelection}
          onItemClick={(e, record, index) => handleItemClick(e, record, index, sortedContent)}
          onItemDoubleClick={setPath}
          onContextMenu={handleContextMenu}
          onItemDragStart={handleItemDragStart}
          onItemDragEnd={handleItemDragEnd}
          onFolderDragOver={handleFolderDragOver}
          onFolderDragLeave={handleFolderDragLeave}
          onFolderDrop={handleFolderDrop}
          sortConfig={sortConfig}
          onSortChange={setSortConfig}
        />
      ) : (
        <div ref={gridContainerRef} style={{ flex: 1, overflow: 'auto' }}>
          <FolderContentGrid
            dataSource={sortedContent}
            loading={isLoading}
            selectedItems={selectedItems}
            dragOverFolder={dragOverFolder}
            onItemClick={(e, record, index) => handleItemClick(e, record, index, sortedContent)}
            onItemDoubleClick={setPath}
            onContextMenu={handleContextMenu}
            onItemDragStart={handleItemDragStart}
            onItemDragEnd={handleItemDragEnd}
            onFolderDragOver={handleFolderDragOver}
            onFolderDragLeave={handleFolderDragLeave}
            onFolderDrop={handleFolderDrop}
            itemsRef={itemsRef}
            disableDraggable={isSelecting}
          />
          <BoxSelectionOverlay
            visible={isSelecting && selectionBox !== null}
            startX={selectionBox?.startX ?? 0}
            startY={selectionBox?.startY ?? 0}
            currentX={selectionBox?.currentX ?? 0}
            currentY={selectionBox?.currentY ?? 0}
          />
        </div>
      )}

      <RenameModal
        visible={modals.rename.visible}
        initialName={modals.rename.data.newName}
        onConfirm={handleRenameConfirm}
        onCancel={() => closeModal('rename')}
        onChange={(newName) => updateModalData('rename', { newName })}
      />

      <CreateFolderModal
        visible={modals.createFolder.visible}
        folderName={modals.createFolder.data.folderName}
        onConfirm={handleCreateFolderConfirm}
        onCancel={() => closeModal('createFolder')}
        onChange={(folderName) => updateModalData('createFolder', { folderName })}
      />

      <DestinationPickerModal
        visible={modals.destination.visible}
        mode={modals.destination.data.mode}
        sourceCount={selectedItems.size}
        sources={Array.from(selectedItems)}
        currentPath={selectedPath}
        selectedSpace={selectedSpace}
        onConfirm={modals.destination.data.mode === 'move' ? handleMoveConfirm : handleCopyConfirm}
        onCancel={() => closeModal('destination')}
      />

      <UploadOverlay visible={isDragging} />
    </div>
  );
};

export default FolderContent;
