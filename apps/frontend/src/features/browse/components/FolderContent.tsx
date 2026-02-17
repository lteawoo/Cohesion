import React, { useEffect, useState, useRef, useMemo, useCallback } from 'react';
import { Empty, App, Grid, Button, Drawer, Menu, theme } from 'antd';
import { DownloadOutlined, CopyOutlined, DeleteOutlined, EditOutlined, CloseOutlined, MoreOutlined } from '@ant-design/icons';
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
import { useAuth } from '@/features/auth/useAuth';

const LONG_PRESS_DURATION_MS = 420;

const FolderContent: React.FC = () => {
  const { message } = App.useApp();
  const { token } = theme.useToken();
  const screens = Grid.useBreakpoint();
  const isMobile = !screens.lg;
  const { user } = useAuth();
  const permissions = user?.permissions ?? [];
  const canWriteFiles = permissions.includes('file.write');

  // Store selectors
  const selectedPath = useBrowseStore((state) => state.selectedPath);
  const selectedSpace = useBrowseStore((state) => state.selectedSpace);
  const content = useBrowseStore((state) => state.content);
  const isLoading = useBrowseStore((state) => state.isLoading);
  const setPath = useBrowseStore((state) => state.setPath);

  const fileInputRef = useRef<HTMLInputElement>(null);
  const gridContainerRef = useRef<HTMLDivElement>(null);
  const itemsRef = useRef<Map<string, HTMLElement>>(new Map());

  // Local state
  const [viewMode, setViewMode] = useState<ViewMode>('grid');
  const [sortConfig, setSortConfig] = useState<SortConfig>({
    sortBy: 'name',
    sortOrder: 'ascend',
  });
  const [isMobileSelectionMode, setIsMobileSelectionMode] = useState(false);
  const [isMobileActionsOpen, setIsMobileActionsOpen] = useState(false);
  const longPressTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const lastLongPressRef = useRef<{ path: string; expiresAt: number } | null>(null);
  const suppressTapUntilRef = useRef(0);
  const selectedItemsRef = useRef<Set<string>>(new Set());

  // Modal management
  const { modals, openModal, closeModal, updateModalData } = useModalManager();
  const prevNavRef = useRef<{ path: string; spaceId?: number }>({ path: '', spaceId: undefined });

  // Custom hooks
  const { selectedItems, handleItemClick, setSelection, clearSelection } = useFileSelection();

  const clearLongPressTimer = useCallback(() => {
    if (longPressTimerRef.current) {
      clearTimeout(longPressTimerRef.current);
      longPressTimerRef.current = null;
    }
  }, []);

  const handleClearSelection = useCallback(() => {
    clearSelection();
    setIsMobileSelectionMode(false);
    setIsMobileActionsOpen(false);
  }, [clearSelection]);

  const {
    handleRename: performRename,
    handleCreateFolder: performCreateFolder,
    handleDelete,
    handleBulkDelete,
    handleMove,
    handleCopy,
    handleBulkDownload,
    handleFileUpload,
  } = useFileOperations(selectedPath, selectedSpace);

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
        if (selectedSpace) {
          const relativePath = path.replace(selectedSpace.space_path, '').replace(/^\//, '');
          window.location.href = `/api/spaces/${selectedSpace.id}/files/download?path=${encodeURIComponent(relativePath)}`;
        }
      },
      onCopy: () => openModal('destination', { mode: 'copy', sources: Array.from(selectedItems) }),
      onMove: () => openModal('destination', { mode: 'move', sources: Array.from(selectedItems) }),
      onRename: (record: FileNode) => {
        openModal('rename', { record, newName: record.name });
      },
      onDelete: handleDelete,
      onBulkDownload: () => handleBulkDownload(Array.from(selectedItems)),
      onBulkDelete: () => handleBulkDelete(Array.from(selectedItems)),
      onCreateFolder: () => openModal('createFolder', { folderName: '' }),
    },
  });

  useEffect(() => {
    if (selectedPath && selectedSpace) {
      const relativePath = selectedPath
        .replace(selectedSpace.space_path, '')
        .replace(/^\//, '');
      useBrowseStore.getState().fetchSpaceContents(selectedSpace.id, relativePath);
    }

    const currentNav = { path: selectedPath, spaceId: selectedSpace?.id };
    const prevNav = prevNavRef.current;
    const hasNavigated = prevNav.path !== currentNav.path || prevNav.spaceId !== currentNav.spaceId;

    // 이동/복사 모달이 열려 있을 때는 selection을 유지해야 source 목록이 안정적으로 유지됨
    if (hasNavigated && !modals.destination.visible) {
      // Effect 내부의 동기 setState 호출을 피하고 다음 프레임에 반영
      requestAnimationFrame(() => {
        handleClearSelection();
      });
    }

    prevNavRef.current = currentNav;
  }, [selectedPath, selectedSpace, handleClearSelection, modals.destination.visible]);

  // 정렬된 콘텐츠 (폴더 우선 + sortConfig)
  const sortedContent = useSortedContent(content, sortConfig);
  const isAnyModalOpen =
    modals.destination.visible || modals.rename.visible || modals.createFolder.visible;

  // Columns for table view
  const columns = useMemo(() => buildTableColumns(setPath, sortConfig), [setPath, sortConfig]);

  // Box selection (Grid 뷰 전용)
  const { isSelecting, selectionBox, wasRecentlySelecting } = useBoxSelection({
    enabled: viewMode === 'grid' && !isMobile && !isAnyModalOpen,
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

  useEffect(() => {
    selectedItemsRef.current = selectedItems;
  }, [selectedItems]);

  useEffect(() => {
    return () => {
      clearLongPressTimer();
    };
  }, [clearLongPressTimer]);

  const toggleMobileSelection = useCallback((path: string) => {
    const next = new Set(selectedItemsRef.current);
    if (next.has(path)) {
      next.delete(path);
    } else {
      next.add(path);
    }
    setSelection(next);
    if (next.size === 0) {
      setIsMobileSelectionMode(false);
    }
  }, [setSelection]);

  const armToolbarInteractionGuard = useCallback(() => {
    suppressTapUntilRef.current = Date.now() + 700;
  }, []);

  const showMobileSelectionBar =
    isMobile && selectedItems.size > 0;
  const moveActionIcon = (
    <span
      className="material-symbols-rounded"
      style={{
        fontSize: 18,
        lineHeight: 1,
        fontVariationSettings: '"FILL" 1, "wght" 500, "GRAD" 0, "opsz" 20',
      }}
      aria-hidden="true"
    >
      drive_file_move
    </span>
  );

  const handleMobileLongPressStart = useCallback((record: FileNode) => {
    if (!isMobile || isMobileSelectionMode) {
      return;
    }
    clearLongPressTimer();
    longPressTimerRef.current = setTimeout(() => {
      lastLongPressRef.current = {
        path: record.path,
        expiresAt: Date.now() + 900,
      };
      setIsMobileSelectionMode(true);
      setSelection(new Set([record.path]));
    }, LONG_PRESS_DURATION_MS);
  }, [isMobile, isMobileSelectionMode, clearLongPressTimer, setSelection]);

  const handleMobileLongPressEnd = useCallback(() => {
    clearLongPressTimer();
  }, [clearLongPressTimer]);

  const handleItemTap = useCallback((e: React.MouseEvent<HTMLElement>, record: FileNode, index: number) => {
    if (Date.now() < suppressTapUntilRef.current) {
      e.preventDefault();
      e.stopPropagation();
      return;
    }

    if (isAnyModalOpen) {
      e.preventDefault();
      e.stopPropagation();
      return;
    }

    if (!isMobile) {
      handleItemClick(e, record, index, sortedContent);
      return;
    }

    const lastLongPress = lastLongPressRef.current;
    if (lastLongPress) {
      if (Date.now() > lastLongPress.expiresAt) {
        lastLongPressRef.current = null;
      } else if (lastLongPress.path === record.path) {
        lastLongPressRef.current = null;
        e.preventDefault();
        e.stopPropagation();
        return;
      }
    }

    if (isMobileSelectionMode) {
      toggleMobileSelection(record.path);
      return;
    }

    if (record.isDir) {
      setPath(record.path);
      handleClearSelection();
      return;
    }
  }, [isAnyModalOpen, isMobile, handleItemClick, sortedContent, isMobileSelectionMode, toggleMobileSelection, setPath, handleClearSelection]);

  const handleItemContextMenu = useCallback((e: React.MouseEvent<HTMLElement>, record: FileNode) => {
    if (isMobile || isAnyModalOpen) {
      e.preventDefault();
      return;
    }
    handleContextMenu(e, record);
  }, [isMobile, isAnyModalOpen, handleContextMenu]);

  const handleContainerContextMenu = useCallback((e: React.MouseEvent) => {
    if (isMobile || isAnyModalOpen) {
      e.preventDefault();
      return;
    }
    handleEmptyAreaContextMenu(e);
  }, [isMobile, isAnyModalOpen, handleEmptyAreaContextMenu]);

  // Modal wrapper handlers
  const handleRenameConfirm = async () => {
    const { record, newName } = modals.rename.data;
    if (!record || !newName.trim()) {
      message.error('새 이름을 입력하세요');
      return;
    }
    await performRename(record.path, newName.trim());
    closeModal('rename');
    handleClearSelection();
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

  const handleMoveConfirm = async (destination: string, destinationSpace?: import('@/features/space/types').Space) => {
    await handleMove(modals.destination.data.sources, destination, destinationSpace);
    closeModal('destination');
    handleClearSelection();
  };

  const handleCopyConfirm = async (destination: string, destinationSpace?: import('@/features/space/types').Space) => {
    await handleCopy(modals.destination.data.sources, destination, destinationSpace);
    closeModal('destination');
    handleClearSelection();
  };

  const handleDestinationCancel = () => {
    suppressTapUntilRef.current = Date.now() + 700;
    const preservedSources = [...modals.destination.data.sources];
    closeModal('destination');

    // 모달 닫힘 과정에서 발생할 수 있는 클릭 이벤트 후 selection 복원
    setTimeout(() => {
      setSelection(new Set(preservedSources));
    }, 0);
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
    if (Date.now() < suppressTapUntilRef.current) {
      return;
    }

    // 모바일 선택모드에서는 배경 탭으로 선택 해제하지 않는다.
    if (isMobile && isMobileSelectionMode) {
      return;
    }

    // 어떤 모달이든 열려 있으면 selection을 유지 (모달 조작 중 해제 방지)
    if (isAnyModalOpen) {
      return;
    }

    // 박스 선택 직후에는 무시
    if (wasRecentlySelecting) {
      return;
    }

    const target = e.target as HTMLElement;
    const isCard = target.closest('.ant-card');
    const isTableRow = target.closest('tr');
    const isButton = target.closest('button');
    const isInput = target.closest('input');
    const isModalContent = target.closest('.ant-modal');
    const isMobileSelectionBar = target.closest('[data-mobile-selection-bar="true"]');

    // 모달 내부 클릭은 React portal 이벤트 버블링으로 들어오므로 선택 해제 대상에서 제외
    if (isModalContent) {
      return;
    }

    if (isMobileSelectionBar) {
      return;
    }

    // 카드, 테이블 행, 버튼, 입력 필드가 아닌 빈 영역만 선택 해제
    if (!isCard && !isTableRow && !isButton && !isInput) {
      handleClearSelection();
      return;
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
      style={{ display: 'flex', flexDirection: 'column', gap: '16px', position: 'relative', height: '100%', minHeight: 0 }}
      onDragEnter={canWriteFiles ? handleDragEnter : undefined}
      onDragLeave={canWriteFiles ? handleDragLeave : undefined}
      onDragOver={canWriteFiles ? handleDragOver : undefined}
      onDrop={canWriteFiles ? handleDrop : undefined}
      onContextMenu={handleContainerContextMenu}
      onClick={handleContainerClick}
    >
      <input
        ref={fileInputRef}
        type="file"
        style={{ display: 'none' }}
        onChange={handleFileSelect}
      />

      {showMobileSelectionBar ? (
        <div
          data-mobile-selection-bar="true"
          onClick={(e) => e.stopPropagation()}
          onTouchStart={(e) => e.stopPropagation()}
          onPointerDownCapture={(e) => {
            const target = e.target as HTMLElement;
            const isButtonTarget = Boolean(target.closest('button'));
            if (!isButtonTarget) {
              e.preventDefault();
            }
            e.stopPropagation();
            suppressTapUntilRef.current = Date.now() + 700;
          }}
          onTouchStartCapture={(e) => {
            e.stopPropagation();
            suppressTapUntilRef.current = Date.now() + 700;
          }}
          onClickCapture={(e) => {
            const target = e.target as HTMLElement;
            const isButtonTarget = Boolean(target.closest('button'));
            if (!isButtonTarget) {
              e.preventDefault();
              e.stopPropagation();
              suppressTapUntilRef.current = Date.now() + 700;
            }
          }}
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'flex-start',
            flexWrap: 'wrap',
            gap: 8,
            padding: '10px 12px',
            background: token.colorBgElevated,
            border: `1px solid ${token.colorBorder}`,
            borderRadius: 8,
          }}
        >
          <Button
            size="small"
            icon={<CloseOutlined />}
            onClick={() => {
              armToolbarInteractionGuard();
              handleClearSelection();
            }}
          />
          <span style={{ fontWeight: 600, color: token.colorText }}>{selectedItems.size}개 항목</span>
          {canWriteFiles && (
            <Button
              size="small"
              icon={moveActionIcon}
              onClick={() => {
                armToolbarInteractionGuard();
                openModal('destination', { mode: 'move', sources: Array.from(selectedItems) });
              }}
            />
          )}
          {canWriteFiles && (
            <Button
              size="small"
              icon={<CopyOutlined />}
              onClick={() => {
                armToolbarInteractionGuard();
                openModal('destination', { mode: 'copy', sources: Array.from(selectedItems) });
              }}
            />
          )}
          <Button
            size="small"
            icon={<DownloadOutlined />}
            onClick={() => {
              armToolbarInteractionGuard();
              handleBulkDownload(Array.from(selectedItems));
            }}
          />
          {canWriteFiles && (
            <Button
              size="small"
              icon={<MoreOutlined />}
              style={{ marginLeft: 'auto' }}
              onClick={() => {
                armToolbarInteractionGuard();
                setIsMobileActionsOpen(true);
              }}
            />
          )}
        </div>
      ) : (
        <FolderContentToolbar
          breadcrumbItems={breadcrumbItems}
          viewMode={viewMode}
          sortConfig={sortConfig}
          canUpload={canWriteFiles}
          onUpload={handleUploadClick}
          onViewModeChange={setViewMode}
          onSortChange={setSortConfig}
        />
      )}

      {!isMobile && (
      <FolderContentSelectionBar
        selectedCount={selectedItems.size}
        showRename={selectedItems.size === 1}
        canWrite={canWriteFiles}
        onDownload={() => handleBulkDownload(Array.from(selectedItems))}
        onCopy={() => openModal('destination', { mode: 'copy', sources: Array.from(selectedItems) })}
        onMove={() => openModal('destination', { mode: 'move', sources: Array.from(selectedItems) })}
        onRename={() => {
          const path = Array.from(selectedItems)[0];
          const record = sortedContent.find(item => item.path === path);
          if (record) {
            openModal('rename', { record, newName: record.name });
          }
        }}
        onDelete={() => handleBulkDelete(Array.from(selectedItems))}
        onClear={handleClearSelection}
      />
      )}

      {viewMode === 'table' ? (
        <div
          style={{ flex: 1, minWidth: 0, minHeight: 0, overflowY: 'auto', overflowX: 'hidden' }}
        >
          <FolderContentTable
            dataSource={sortedContent}
            columns={columns}
            loading={isLoading}
            selectedItems={selectedItems}
            dragOverFolder={dragOverFolder}
            onSelectionChange={setSelection}
            onItemClick={handleItemTap}
            onItemDoubleClick={setPath}
            onItemTouchStart={(record) => handleMobileLongPressStart(record)}
            onItemTouchEnd={handleMobileLongPressEnd}
            onItemTouchCancel={handleMobileLongPressEnd}
            onContextMenu={handleItemContextMenu}
            onItemDragStart={handleItemDragStart}
            onItemDragEnd={handleItemDragEnd}
            onFolderDragOver={handleFolderDragOver}
            onFolderDragLeave={handleFolderDragLeave}
            onFolderDrop={handleFolderDrop}
            sortConfig={sortConfig}
            onSortChange={setSortConfig}
            isMobile={isMobile}
            isSelectionMode={isMobileSelectionMode}
            disableDrag={isMobile || !canWriteFiles}
          />
        </div>
      ) : (
        <div
          ref={gridContainerRef}
          style={{ position: 'relative', flex: 1, minWidth: 0, overflowY: 'auto', overflowX: 'hidden' }}
        >
          <FolderContentGrid
            dataSource={sortedContent}
            loading={isLoading}
            selectedItems={selectedItems}
            dragOverFolder={dragOverFolder}
            onItemClick={handleItemTap}
            onItemDoubleClick={setPath}
            onItemTouchStart={(record) => handleMobileLongPressStart(record)}
            onItemTouchEnd={handleMobileLongPressEnd}
            onItemTouchCancel={handleMobileLongPressEnd}
            onContextMenu={handleItemContextMenu}
            onItemDragStart={handleItemDragStart}
            onItemDragEnd={handleItemDragEnd}
            onFolderDragOver={handleFolderDragOver}
            onFolderDragLeave={handleFolderDragLeave}
            onFolderDrop={handleFolderDrop}
            itemsRef={itemsRef}
            disableDraggable={isSelecting || isMobile || !canWriteFiles}
            spaceId={selectedSpace?.id}
            spacePath={selectedSpace?.space_path}
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
        onCancel={() => {
          suppressTapUntilRef.current = Date.now() + 700;
          closeModal('rename');
        }}
        onChange={(newName) => updateModalData('rename', { newName })}
      />

      <CreateFolderModal
        visible={modals.createFolder.visible}
        folderName={modals.createFolder.data.folderName}
        onConfirm={handleCreateFolderConfirm}
        onCancel={() => {
          suppressTapUntilRef.current = Date.now() + 700;
          closeModal('createFolder');
        }}
        onChange={(folderName) => updateModalData('createFolder', { folderName })}
      />

      <DestinationPickerModal
        visible={modals.destination.visible}
        mode={modals.destination.data.mode}
        sourceCount={modals.destination.data.sources.length}
        sources={modals.destination.data.sources}
        currentPath={selectedPath}
        onConfirm={modals.destination.data.mode === 'move' ? handleMoveConfirm : handleCopyConfirm}
        onCancel={handleDestinationCancel}
      />

      <Drawer
        placement="bottom"
        title={null}
        open={showMobileSelectionBar && isMobileActionsOpen}
        onClose={() => setIsMobileActionsOpen(false)}
        closable={false}
        size="default"
        styles={{ body: { padding: 0 } }}
      >
        <div style={{ padding: '8px 0 4px' }}>
          <div
            style={{
              width: 36,
              height: 4,
              borderRadius: 999,
              background: token.colorBorderSecondary,
              margin: '0 auto 8px',
            }}
          />
          <Menu
            selectable={false}
            items={[
              ...(canWriteFiles && selectedItems.size === 1
                ? [{
                    key: 'rename',
                    icon: <EditOutlined />,
                    label: '이름 변경',
                    onClick: () => {
                      const path = Array.from(selectedItems)[0];
                      const record = sortedContent.find(item => item.path === path);
                      if (record) {
                        setIsMobileActionsOpen(false);
                        openModal('rename', { record, newName: record.name });
                      }
                    },
                  }]
                : []),
              ...(canWriteFiles
                ? [{
                    key: 'delete',
                    icon: <DeleteOutlined />,
                    label: '삭제',
                    danger: true,
                    onClick: () => {
                      setIsMobileActionsOpen(false);
                      handleBulkDelete(Array.from(selectedItems));
                    },
                  }]
                : []),
            ]}
          />
        </div>
      </Drawer>

      <UploadOverlay visible={canWriteFiles && isDragging} />
    </div>
  );
};

export default FolderContent;
