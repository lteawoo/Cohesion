import React, { useEffect, useState, useRef, useCallback, useLayoutEffect, useMemo } from 'react';
import { Empty, App, Grid, Button, Menu, theme, Breadcrumb } from 'antd';
import { DownloadOutlined, CopyOutlined, DeleteOutlined, EditOutlined, CloseOutlined, MoreOutlined } from '@ant-design/icons';
import { useBrowseStore } from '@/stores/browseStore';
import type { FileNode, ViewMode, SortConfig } from '../types';
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
import FolderContentTable from './FolderContent/FolderContentTable';
import FolderContentGrid from './FolderContent/FolderContentGrid';
import RenameModal from './FolderContent/RenameModal';
import CreateFolderModal from './FolderContent/CreateFolderModal';
import UploadOverlay from './FolderContent/UploadOverlay';
import BoxSelectionOverlay from './FolderContent/BoxSelectionOverlay';
import { useAuth } from '@/features/auth/useAuth';
import BottomSheet from '@/components/common/BottomSheet';

const LONG_PRESS_DURATION_MS = 420;
const PATH_BAR_HEIGHT = 36;
const EXPLORER_SIDE_PADDING = 16;
type NavigationState = { entries: string[]; index: number };

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
  const rootContainerRef = useRef<HTMLDivElement>(null);
  const selectionContainerRef = useRef<HTMLDivElement>(null);
  const pathBarViewportRef = useRef<HTMLDivElement>(null);
  const pathBarMeasureRef = useRef<HTMLDivElement>(null);
  const itemsRef = useRef<Map<string, HTMLElement>>(new Map());

  // Local state
  const [viewMode, setViewMode] = useState<ViewMode>('grid');
  const [sortConfig, setSortConfig] = useState<SortConfig>({
    sortBy: 'name',
    sortOrder: 'ascend',
  });
  const [isMobileSelectionMode, setIsMobileSelectionMode] = useState(false);
  const [isMobileActionsOpen, setIsMobileActionsOpen] = useState(false);
  const [isPathOverflow, setIsPathOverflow] = useState(false);
  const [navigationState, setNavigationState] = useState<NavigationState>({ entries: [], index: -1 });
  const longPressTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const lastLongPressRef = useRef<{ path: string; expiresAt: number } | null>(null);
  const suppressTapUntilRef = useRef(0);
  const selectedItemsRef = useRef<Set<string>>(new Set());
  const historySpaceIdRef = useRef<number | undefined>(undefined);
  const isHistoryTraversalRef = useRef(false);

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

  const handleNavigate = useCallback((path: string, space = selectedSpace) => {
    setPath(path, space);
  }, [selectedSpace, setPath]);

  const { breadcrumbItems } = useBreadcrumb({
    selectedPath,
    selectedSpace,
    onNavigate: handleNavigate,
  });

  const compactBreadcrumbItems = useMemo(() => {
    if (!isPathOverflow || breadcrumbItems.length <= 3) {
      return breadcrumbItems;
    }

    const first = breadcrumbItems[0];
    const tail = breadcrumbItems.slice(-2);
    return [
      first,
      { key: 'collapsed-ellipsis', title: <span style={{ opacity: 0.72 }}>...</span> },
      ...tail,
    ];
  }, [breadcrumbItems, isPathOverflow]);

  // 정렬된 콘텐츠 (폴더 우선 + sortConfig)
  const sortedContent = useSortedContent(content, sortConfig);

  const { handleContextMenu, handleEmptyAreaContextMenu } = useContextMenu({
    selectedItems,
    sortedContent,
    canWriteFiles,
    onSetSelection: setSelection,
    callbacks: {
      onDownload: (path: string) => {
        if (selectedSpace) {
          window.location.href = `/api/spaces/${selectedSpace.id}/files/download?path=${encodeURIComponent(path)}`;
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
    if (selectedSpace) {
      useBrowseStore.getState().fetchSpaceContents(selectedSpace.id, selectedPath);
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

  useEffect(() => {
    if (!selectedSpace) {
      setNavigationState({ entries: [], index: -1 });
      historySpaceIdRef.current = undefined;
      return;
    }

    if (historySpaceIdRef.current !== selectedSpace.id) {
      historySpaceIdRef.current = selectedSpace.id;
      setNavigationState({ entries: [selectedPath], index: 0 });
      isHistoryTraversalRef.current = false;
      return;
    }

    if (isHistoryTraversalRef.current) {
      isHistoryTraversalRef.current = false;
      return;
    }

    setNavigationState((prev) => {
      if (prev.index >= 0 && prev.entries[prev.index] === selectedPath) {
        return prev;
      }
      const entries = prev.index >= 0
        ? [...prev.entries.slice(0, prev.index + 1), selectedPath]
        : [selectedPath];
      return { entries, index: entries.length - 1 };
    });
  }, [selectedPath, selectedSpace]);

  const handleGoBack = useCallback(() => {
    if (navigationState.index <= 0) {
      return;
    }
    const nextIndex = navigationState.index - 1;
    const targetPath = navigationState.entries[nextIndex];
    isHistoryTraversalRef.current = true;
    setNavigationState((prev) => ({ ...prev, index: nextIndex }));
    handleNavigate(targetPath);
  }, [navigationState, handleNavigate]);

  const handleGoForward = useCallback(() => {
    if (navigationState.index < 0 || navigationState.index >= navigationState.entries.length - 1) {
      return;
    }
    const nextIndex = navigationState.index + 1;
    const targetPath = navigationState.entries[nextIndex];
    isHistoryTraversalRef.current = true;
    setNavigationState((prev) => ({ ...prev, index: nextIndex }));
    handleNavigate(targetPath);
  }, [navigationState, handleNavigate]);

  const isAnyModalOpen =
    modals.destination.visible || modals.rename.visible || modals.createFolder.visible;

  // Box selection (Grid 뷰 전용)
  const { isSelecting, selectionBox, wasRecentlySelecting } = useBoxSelection({
    enabled: viewMode === 'grid' && !isMobile && !isAnyModalOpen,
    startAreaRef: rootContainerRef,
    startAreaOutsetPx: 16,
    containerRef: selectionContainerRef,
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

  useLayoutEffect(() => {
    const viewport = pathBarViewportRef.current;
    const measure = pathBarMeasureRef.current;
    if (!viewport || !measure) {
      return;
    }

    const checkOverflow = () => {
      const hasOverflow = measure.scrollWidth > viewport.clientWidth;
      setIsPathOverflow(prev => (prev === hasOverflow ? prev : hasOverflow));
    };

    checkOverflow();

    const ro = new ResizeObserver(() => {
      checkOverflow();
    });
    ro.observe(viewport);
    ro.observe(measure);
    window.addEventListener('resize', checkOverflow);

    return () => {
      ro.disconnect();
      window.removeEventListener('resize', checkOverflow);
    };
  }, [breadcrumbItems]);

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
  const showDesktopSelectionBar = !isMobile && selectedItems.size > 0;
  const topRowHeight = isMobile ? 44 : 52;
  const topRowOffset = 8;
  const topRowSlotHeight = topRowHeight + topRowOffset;
  const rootRect = rootContainerRef.current?.getBoundingClientRect();
  const selectionRect = selectionContainerRef.current?.getBoundingClientRect();
  const selectionScrollLeft = selectionContainerRef.current?.scrollLeft ?? 0;
  const selectionScrollTop = selectionContainerRef.current?.scrollTop ?? 0;
  const overlayOffsetX = rootRect && selectionRect
    ? (selectionRect.left - rootRect.left) - selectionScrollLeft
    : 0;
  const overlayOffsetY = rootRect && selectionRect
    ? (selectionRect.top - rootRect.top) - selectionScrollTop
    : 0;
  const moveActionIcon = (
    <span
      className="material-symbols-rounded move-action-icon"
      style={{
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
      handleNavigate(record.path);
      handleClearSelection();
      return;
    }
  }, [isAnyModalOpen, isMobile, handleItemClick, sortedContent, isMobileSelectionMode, toggleMobileSelection, handleNavigate, handleClearSelection]);

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
    const isSelectionToolbar = target.closest('[data-selection-toolbar="true"]');
    const isMobileSelectionBar = target.closest('[data-mobile-selection-bar="true"]');
    const isPathBar = target.closest('[data-path-bar="true"]');

    // 모달 내부 클릭은 React portal 이벤트 버블링으로 들어오므로 선택 해제 대상에서 제외
    if (isModalContent) {
      return;
    }

    if (isSelectionToolbar) {
      return;
    }

    if (isMobileSelectionBar) {
      return;
    }

    if (isPathBar) {
      return;
    }

    // 카드, 테이블 행, 버튼, 입력 필드가 아닌 빈 영역만 선택 해제
    if (!isCard && !isTableRow && !isButton && !isInput) {
      handleClearSelection();
      return;
    }
  };

  // Early return if no space selected
  if (!selectedSpace) {
    return (
      <div style={{ height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <Empty description="왼쪽 트리나 스페이스에서 폴더를 선택하세요." />
      </div>
    );
  }

  return (
    <div
      ref={rootContainerRef}
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

      <div style={{ height: topRowSlotHeight }}>
        {showMobileSelectionBar ? (
          <div
            data-selection-toolbar="true"
            data-selection-exclude="true"
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
              flexWrap: 'nowrap',
              gap: 8,
              width: '100%',
              height: topRowHeight,
              marginTop: topRowOffset,
              padding: '0 12px',
              background: token.colorBgElevated,
              border: `1px solid ${token.colorBorder}`,
              borderRadius: 8,
              overflowX: 'auto',
              overflowY: 'hidden',
            }}
          >
            <Button
              size="small"
              icon={<CloseOutlined />}
              aria-label="선택 해제"
              title="선택 해제"
              onClick={() => {
                armToolbarInteractionGuard();
                handleClearSelection();
              }}
            />
            <span style={{ fontWeight: 600, color: token.colorText, whiteSpace: 'nowrap' }}>{selectedItems.size}개 선택됨</span>
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
                icon={<CopyOutlined />}
                onClick={() => {
                  armToolbarInteractionGuard();
                  openModal('destination', { mode: 'copy', sources: Array.from(selectedItems) });
                }}
              />
            )}
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
                icon={<MoreOutlined />}
                style={{ marginLeft: 'auto' }}
                onClick={() => {
                  armToolbarInteractionGuard();
                  setIsMobileActionsOpen(true);
                }}
              />
            )}
          </div>
        ) : showDesktopSelectionBar ? (
          <div
            data-selection-toolbar="true"
            data-selection-exclude="true"
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'flex-start',
              flexWrap: 'nowrap',
              gap: 8,
              width: '100%',
              height: topRowHeight,
              marginTop: topRowOffset,
              padding: '0 12px',
              background: token.colorBgElevated,
              border: `1px solid ${token.colorBorder}`,
              borderRadius: 8,
              overflowX: 'auto',
              overflowY: 'hidden',
            }}
          >
            <Button size="small" icon={<CloseOutlined />} aria-label="선택 해제" title="선택 해제" onClick={handleClearSelection} />
            <span style={{ fontWeight: 600, color: token.colorText, whiteSpace: 'nowrap' }}>
              {selectedItems.size}개 선택됨
            </span>
            <Button size="small" icon={<DownloadOutlined />} onClick={() => handleBulkDownload(Array.from(selectedItems))}>
              다운로드
            </Button>
            {canWriteFiles && (
              <Button size="small" icon={<CopyOutlined />} onClick={() => openModal('destination', { mode: 'copy', sources: Array.from(selectedItems) })}>
                복사
              </Button>
            )}
            {canWriteFiles && (
              <Button size="small" icon={moveActionIcon} onClick={() => openModal('destination', { mode: 'move', sources: Array.from(selectedItems) })}>
                이동
              </Button>
            )}
            {canWriteFiles && selectedItems.size === 1 && (
              <Button
                size="small"
                icon={<EditOutlined />}
                onClick={() => {
                  const path = Array.from(selectedItems)[0];
                  const record = sortedContent.find(item => item.path === path);
                  if (record) {
                    openModal('rename', { record, newName: record.name });
                  }
                }}
              >
                이름 변경
              </Button>
            )}
            {canWriteFiles && (
              <Button size="small" icon={<DeleteOutlined />} danger onClick={() => handleBulkDelete(Array.from(selectedItems))}>
                삭제
              </Button>
            )}
          </div>
        ) : (
          <div style={{ height: topRowHeight }}>
              <FolderContentToolbar
                viewMode={viewMode}
                sortConfig={sortConfig}
                canUpload={canWriteFiles}
                canGoBack={navigationState.index > 0}
                canGoForward={navigationState.index >= 0 && navigationState.index < navigationState.entries.length - 1}
                compact
                onGoBack={handleGoBack}
                onGoForward={handleGoForward}
                onUpload={handleUploadClick}
                onViewModeChange={setViewMode}
                onSortChange={setSortConfig}
              />
          </div>
        )}
      </div>

      <div
        ref={selectionContainerRef}
        style={{
          position: 'relative',
          flex: 1,
          minWidth: 0,
          minHeight: 0,
          overflowY: 'auto',
          overflowX: 'hidden',
          paddingBottom: PATH_BAR_HEIGHT + 6,
        }}
      >
        {viewMode === 'table' ? (
          <FolderContentTable
            dataSource={sortedContent}
            loading={isLoading}
            selectedItems={selectedItems}
            dragOverFolder={dragOverFolder}
            onItemClick={handleItemTap}
            onItemDoubleClick={handleNavigate}
            onItemTouchStart={(record) => handleMobileLongPressStart(record)}
            onItemTouchEnd={handleMobileLongPressEnd}
            onItemTouchCancel={handleMobileLongPressEnd}
            onContextMenu={handleItemContextMenu}
            onItemDragStart={handleItemDragStart}
            onItemDragEnd={handleItemDragEnd}
            onFolderDragOver={handleFolderDragOver}
            onFolderDragLeave={handleFolderDragLeave}
            onFolderDrop={handleFolderDrop}
            disableDrag
            canWriteFiles={canWriteFiles}
            onItemDownload={(record) => handleBulkDownload([record.path])}
            onItemCopy={(record) => openModal('destination', { mode: 'copy', sources: [record.path] })}
            onItemMove={(record) => openModal('destination', { mode: 'move', sources: [record.path] })}
            onItemRename={(record) => openModal('rename', { record, newName: record.name })}
            onItemDelete={handleDelete}
          />
        ) : (
          <>
            <FolderContentGrid
              dataSource={sortedContent}
              loading={isLoading}
              selectedItems={selectedItems}
              dragOverFolder={dragOverFolder}
              onItemClick={handleItemTap}
              onItemDoubleClick={handleNavigate}
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
            />
          </>
        )}
      </div>

      <BoxSelectionOverlay
        visible={isSelecting && selectionBox !== null}
        startX={selectionBox?.startX ?? 0}
        startY={selectionBox?.startY ?? 0}
        currentX={selectionBox?.currentX ?? 0}
        currentY={selectionBox?.currentY ?? 0}
        offsetX={overlayOffsetX}
        offsetY={overlayOffsetY}
      />

      <div
        data-path-bar="true"
        data-selection-exclude="true"
        onPointerDown={(e) => e.stopPropagation()}
        onClick={(e) => e.stopPropagation()}
        style={{
          position: 'absolute',
          left: -EXPLORER_SIDE_PADDING,
          right: -EXPLORER_SIDE_PADDING,
          bottom: -EXPLORER_SIDE_PADDING,
          zIndex: 4,
          height: PATH_BAR_HEIGHT,
          display: 'flex',
          alignItems: 'center',
          padding: '0 16px',
          borderTop: `1px solid ${token.colorBorder}`,
          background: token.colorBgContainer,
          color: token.colorText,
          fontSize: 12,
          lineHeight: 1,
          overflowX: 'auto',
          overflowY: 'hidden',
        }}
      >
        <div ref={pathBarViewportRef} style={{ minWidth: 0, width: '100%', color: token.colorText, overflow: 'hidden' }}>
          <Breadcrumb items={compactBreadcrumbItems} />
        </div>
        <div
          ref={pathBarMeasureRef}
          style={{
            position: 'absolute',
            visibility: 'hidden',
            pointerEvents: 'none',
            whiteSpace: 'nowrap',
            height: 0,
            overflow: 'hidden',
          }}
          aria-hidden="true"
        >
          <Breadcrumb items={breadcrumbItems} />
        </div>
      </div>

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

      <BottomSheet
        open={showMobileSelectionBar && isMobileActionsOpen}
        onClose={() => setIsMobileActionsOpen(false)}
        snapPoints={[1]}
        initialSnapIndex={0}
      >
        <div style={{ padding: '8px 0 4px' }}>
          <div
            style={{
              padding: '0 12px 10px',
              fontSize: 13,
              fontWeight: 600,
              color: token.colorTextSecondary,
            }}
          >
            {selectedItems.size}개 선택됨
          </div>
          <Menu
            selectable={false}
            items={[
              {
                key: 'download',
                icon: <DownloadOutlined />,
                label: '다운로드',
                onClick: () => {
                  armToolbarInteractionGuard();
                  setIsMobileActionsOpen(false);
                  handleBulkDownload(Array.from(selectedItems));
                },
              },
              ...(canWriteFiles
                ? [{
                    key: 'copy',
                    icon: <CopyOutlined />,
                    label: '복사',
                    onClick: () => {
                      armToolbarInteractionGuard();
                      setIsMobileActionsOpen(false);
                      openModal('destination', { mode: 'copy', sources: Array.from(selectedItems) });
                    },
                  }]
                : []),
              ...(canWriteFiles
                ? [{
                    key: 'move',
                    icon: moveActionIcon,
                    label: '이동',
                    onClick: () => {
                      armToolbarInteractionGuard();
                      setIsMobileActionsOpen(false);
                      openModal('destination', { mode: 'move', sources: Array.from(selectedItems) });
                    },
                  }]
                : []),
              ...(canWriteFiles && selectedItems.size === 1
                ? [{
                    key: 'rename',
                    icon: <EditOutlined />,
                    label: '이름 변경',
                    onClick: () => {
                      armToolbarInteractionGuard();
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
                      armToolbarInteractionGuard();
                      setIsMobileActionsOpen(false);
                      handleBulkDelete(Array.from(selectedItems));
                    },
                  }]
                : []),
            ]}
          />
        </div>
      </BottomSheet>

      <UploadOverlay visible={canWriteFiles && isDragging} />
    </div>
  );
};

export default FolderContent;
