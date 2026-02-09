import React, { useEffect, useState, useRef, useMemo } from 'react';
import { Empty, message } from 'antd';
import { useBrowseStore } from '@/stores/browseStore';
import type { FileNode, ViewMode, SortConfig, RenameModalState } from '../types';
import { buildTableColumns } from '../constants';
import { useFileSelection } from '../hooks/useFileSelection';
import { useBreadcrumb } from '../hooks/useBreadcrumb';
import { useFileOperations } from '../hooks/useFileOperations';
import { useDragAndDrop } from '../hooks/useDragAndDrop';
import { useContextMenu } from '../hooks/useContextMenu';
import DestinationPickerModal from './DestinationPickerModal';
import FolderContentToolbar from './FolderContent/FolderContentToolbar';
import FolderContentSelectionBar from './FolderContent/FolderContentSelectionBar';
import FolderContentTable from './FolderContent/FolderContentTable';
import FolderContentGrid from './FolderContent/FolderContentGrid';
import RenameModal from './FolderContent/RenameModal';
import CreateFolderModal from './FolderContent/CreateFolderModal';
import UploadOverlay from './FolderContent/UploadOverlay';

interface FolderContentProps {}

const FolderContent: React.FC<FolderContentProps> = () => {
  // Store selectors
  const selectedPath = useBrowseStore((state) => state.selectedPath);
  const selectedSpace = useBrowseStore((state) => state.selectedSpace);
  const content = useBrowseStore((state) => state.content);
  const isLoading = useBrowseStore((state) => state.isLoading);
  const setPath = useBrowseStore((state) => state.setPath);

  console.log('[FolderContent] Render - selectedPath:', selectedPath);

  const fileInputRef = useRef<HTMLInputElement>(null);

  // Local state
  const [viewMode, setViewMode] = useState<ViewMode>('grid');
  const [sortConfig, setSortConfig] = useState<SortConfig>({
    sortBy: 'name',
    sortOrder: 'ascend',
  });
  const [renameModal, setRenameModal] = useState<RenameModalState>({
    visible: false,
    newName: '',
  });
  const [createFolderModal, setCreateFolderModal] = useState<{
    visible: boolean;
    folderName: string;
  }>({ visible: false, folderName: '' });
  const [destinationModal, setDestinationModal] = useState<{
    visible: boolean;
    mode: 'move' | 'copy';
  }>({ visible: false, mode: 'move' });

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
      onCopy: () => setDestinationModal({ visible: true, mode: 'copy' }),
      onMove: () => setDestinationModal({ visible: true, mode: 'move' }),
      onRename: (record: FileNode) => {
        setRenameModal({
          visible: true,
          record,
          newName: record.name,
        });
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
  const sortedContent = useMemo(() => {
    if (!Array.isArray(content)) {
      return [];
    }

    const sorted = [...content].sort((a, b) => {
      // 1. 폴더 우선 정렬
      if (a.isDir !== b.isDir) {
        return a.isDir ? -1 : 1;
      }

      // 2. sortBy에 따른 정렬
      let result = 0;
      if (sortConfig.sortBy === 'name') {
        result = a.name.localeCompare(b.name);
      } else if (sortConfig.sortBy === 'modTime') {
        result = new Date(a.modTime).getTime() - new Date(b.modTime).getTime();
      } else if (sortConfig.sortBy === 'size') {
        result = a.size - b.size;
      }

      // 3. sortOrder 적용
      return sortConfig.sortOrder === 'ascend' ? result : -result;
    });

    return sorted;
  }, [content, sortConfig]);

  // Columns for table view
  const columns = useMemo(() => buildTableColumns(setPath), [setPath]);

  // Modal wrapper handlers
  const handleRenameConfirm = async () => {
    if (!renameModal.record || !renameModal.newName.trim()) {
      message.error('새 이름을 입력하세요');
      return;
    }
    await performRename(renameModal.record.path, renameModal.newName.trim());
    setRenameModal({ visible: false, newName: '' });
    clearSelection();
  };

  const handleCreateFolderConfirm = async () => {
    if (!createFolderModal.folderName.trim()) {
      message.error('폴더 이름을 입력하세요');
      return;
    }
    await performCreateFolder(selectedPath, createFolderModal.folderName.trim());
    setCreateFolderModal({ visible: false, folderName: '' });
  };

  const handleMoveConfirm = async (destination: string) => {
    await handleMove(Array.from(selectedItems), destination);
    setDestinationModal({ visible: false, mode: 'move' });
    clearSelection();
  };

  const handleCopyConfirm = async (destination: string) => {
    await handleCopy(Array.from(selectedItems), destination);
    setDestinationModal({ visible: false, mode: 'copy' });
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
    const target = e.target as HTMLElement;
    const isCard = target.closest('.ant-card');
    const isTableRow = target.closest('tr');
    if (!isCard && !isTableRow) {
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
        onCopy={() => setDestinationModal({ visible: true, mode: 'copy' })}
        onMove={() => setDestinationModal({ visible: true, mode: 'move' })}
        onRename={() => {
          const path = Array.from(selectedItems)[0];
          const record = sortedContent.find(item => item.path === path);
          if (record) {
            setRenameModal({
              visible: true,
              record,
              newName: record.name,
            });
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
        />
      )}

      <RenameModal
        visible={renameModal.visible}
        initialName={renameModal.newName}
        onConfirm={handleRenameConfirm}
        onCancel={() => setRenameModal({ visible: false, newName: '' })}
        onChange={(newName) => setRenameModal({ ...renameModal, newName })}
      />

      <CreateFolderModal
        visible={createFolderModal.visible}
        folderName={createFolderModal.folderName}
        onConfirm={handleCreateFolderConfirm}
        onCancel={() => setCreateFolderModal({ visible: false, folderName: '' })}
        onChange={(folderName) => setCreateFolderModal({ ...createFolderModal, folderName })}
      />

      <DestinationPickerModal
        visible={destinationModal.visible}
        mode={destinationModal.mode}
        sourceCount={selectedItems.size}
        sources={Array.from(selectedItems)}
        currentPath={selectedPath}
        selectedSpace={selectedSpace}
        onConfirm={destinationModal.mode === 'move' ? handleMoveConfirm : handleCopyConfirm}
        onCancel={() => setDestinationModal({ visible: false, mode: 'move' })}
      />

      <UploadOverlay visible={isDragging} />
    </div>
  );
};

export default FolderContent;
