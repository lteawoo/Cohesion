import React, { useEffect, useState, useRef, useCallback, useLayoutEffect, useMemo } from 'react';
import { Empty, App, Grid, Button, Menu, theme, Breadcrumb, Spin, Alert } from 'antd';
import { DownloadOutlined, CopyOutlined, DeleteOutlined, EditOutlined, CloseOutlined, MoreOutlined } from '@ant-design/icons';
import { useLocation, useNavigate } from 'react-router';
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
import { useBrowseHistoryNavigation } from '../hooks/useBrowseHistoryNavigation';
import { useSearchModeContent } from '../hooks/useSearchModeContent';
import { useTrashModalManager } from '../hooks/useTrashModalManager';
import DestinationPickerModal from './DestinationPickerModal';
import FolderContentToolbar from './FolderContent/FolderContentToolbar';
import FolderContentTable from './FolderContent/FolderContentTable';
import FolderContentGrid from './FolderContent/FolderContentGrid';
import TransferPanel from './FolderContent/TransferPanel';
import RenameModal from './FolderContent/RenameModal';
import CreateFolderModal from './FolderContent/CreateFolderModal';
import TrashModal from './FolderContent/TrashModal';
import UploadOverlay from './FolderContent/UploadOverlay';
import BoxSelectionOverlay from './FolderContent/BoxSelectionOverlay';
import { useAuth } from '@/features/auth/useAuth';
import BottomSheet from '@/components/common/BottomSheet';
import { useTranslation } from 'react-i18next';

const LONG_PRESS_DURATION_MS = 420;
const TOUCH_PAN_THRESHOLD_PX = 8;
const DESKTOP_TOUCH_DOUBLE_TAP_WINDOW_MS = 320;
const DESKTOP_TOUCH_DOUBLE_CLICK_GUARD_MS = 480;
const DESKTOP_TOUCH_EVENT_WINDOW_MS = 700;
const PATH_BAR_HEIGHT = 36;
const EXPLORER_SIDE_PADDING = 16;
const PATH_BAR_CONTENT_OVERLAY_HEIGHT = PATH_BAR_HEIGHT - EXPLORER_SIDE_PADDING;
type BrowseLocationState = { fromSearchQuery?: string };
const EMPTY_SELECTION = new Set<string>();

const FolderContent: React.FC = () => {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const { token } = theme.useToken();
  const location = useLocation();
  const navigate = useNavigate();
  const screens = Grid.useBreakpoint();
  const layoutMode = screens.lg ? 'desktop' : 'mobile';
  const isSearchMode = location.pathname === '/search';
  const locationState = location.state as BrowseLocationState | null;
  const incomingSearchQuery = typeof locationState?.fromSearchQuery === 'string'
    ? locationState.fromSearchQuery.trim()
    : '';
  const searchModeHelpText = t('folderContent.searchHint');
  const { user } = useAuth();
  const permissions = user?.permissions ?? [];
  const canWriteFiles = !isSearchMode && permissions.includes('file.write');

  // Store selectors
  const selectedPath = useBrowseStore((state) => state.selectedPath);
  const selectedSpace = useBrowseStore((state) => state.selectedSpace);
  const content = useBrowseStore((state) => state.content);
  const isLoading = useBrowseStore((state) => state.isLoading);
  const browseError = useBrowseStore((state) => state.error);
  const setPath = useBrowseStore((state) => state.setPath);
  const fetchSpaceContents = useBrowseStore((state) => state.fetchSpaceContents);
  const trashOpenRequest = useBrowseStore((state) => state.trashOpenRequest);
  const clearTrashOpenRequest = useBrowseStore((state) => state.clearTrashOpenRequest);

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
  const [overlayOffset, setOverlayOffset] = useState({ x: 0, y: 0 });
  const longPressTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const lastLongPressRef = useRef<{ path: string; expiresAt: number } | null>(null);
  const desktopTouchTapRef = useRef<{ path: string; at: number } | null>(null);
  const desktopTouchDoubleClickGuardRef = useRef<{ path: string; expiresAt: number } | null>(null);
  const recentDesktopTouchRef = useRef(0);
  const suppressTapUntilRef = useRef(0);
  const touchStartPointRef = useRef<{ x: number; y: number } | null>(null);
  const isTouchPanningRef = useRef(false);
  const selectedItemsRef = useRef<Set<string>>(new Set());
  const isMobileLayout = layoutMode === 'mobile';
  const isTouchInteraction = isMobileLayout;
  const isDesktopTouchFallbackEnabled = !isTouchInteraction && typeof navigator !== 'undefined' && navigator.maxTouchPoints > 0;

  // Modal management
  const { modals, openModal, closeModal, updateModalData } = useModalManager();
  const prevNavRef = useRef<{ path: string; spaceId?: number }>({ path: '', spaceId: undefined });

  // Custom hooks
  const { selectedItems, handleItemClick, setSelection, clearSelection } = useFileSelection();
  const {
    searchSource,
    sourceContent,
    openSearchResultByRecordPath,
    renderSearchName,
    renderSearchMeta,
    renderSearchGridMeta,
    activeErrorMessage,
    activeLoading,
  } = useSearchModeContent({
    isSearchMode,
    browseContent: content,
    browseErrorMessage: browseError?.message ?? null,
    browseLoading: isLoading,
  });

  const clearLongPressTimer = useCallback(() => {
    if (longPressTimerRef.current) {
      clearTimeout(longPressTimerRef.current);
      longPressTimerRef.current = null;
    }
  }, []);

  const clearDesktopTouchTapState = useCallback(() => {
    desktopTouchTapRef.current = null;
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
    fetchTrashItems,
    handleTrashRestore,
    handleTrashDelete,
    handleTrashEmpty,
    handleMove,
    handleCopy,
    handleBulkDownload,
    handleFileUpload,
    cancelUpload,
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

  const { breadcrumbItems: browseBreadcrumbItems } = useBreadcrumb({
    selectedPath,
    selectedSpace,
    onNavigate: handleNavigate,
  });
  const breadcrumbItems = useMemo(() => {
    if (!isSearchMode) {
      return browseBreadcrumbItems;
    }

    const queryText = searchSource.query.trim();
    if (!queryText) {
      return [{ key: 'search-root', title: <span>{t('folderContent.searchBreadcrumb')}</span> }];
    }

    return [
      { key: 'search-root', title: <span>{t('folderContent.searchBreadcrumb')}</span> },
      { key: `search-query:${queryText}`, title: <span>"{queryText}"</span> },
    ];
  }, [browseBreadcrumbItems, isSearchMode, searchSource.query, t]);

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
  const sortedContent = useSortedContent(sourceContent, sortConfig);

  const contentByPath = useMemo(() => {
    const next = new Map<string, FileNode>();
    sortedContent.forEach((item) => {
      next.set(item.path, item);
    });
    return next;
  }, [sortedContent]);
  const effectiveViewMode: ViewMode = viewMode;

  const { handleContextMenu, handleEmptyAreaContextMenu } = useContextMenu({
    selectedItems,
    sortedContent,
    canWriteFiles,
    onSetSelection: setSelection,
    callbacks: {
      onDownload: (path: string) => {
        void handleBulkDownload([path]);
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
    if (!isSearchMode && selectedSpace) {
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
  }, [selectedPath, selectedSpace, handleClearSelection, isSearchMode, modals.destination.visible]);

  const handleNavigateSearch = useCallback((query: string) => {
    navigate(`/search?q=${encodeURIComponent(query)}`);
  }, [navigate]);

  const {
    canGoBack,
    canGoForward,
    goBack: handleGoBack,
    goForward: handleGoForward,
  } = useBrowseHistoryNavigation({
    isSearchMode,
    selectedPath,
    selectedSpaceId: selectedSpace?.id,
    incomingSearchQuery,
    onNavigate: handleNavigate,
    onNavigateSearch: handleNavigateSearch,
  });

  const refreshCurrentFolder = useCallback(async () => {
    if (!selectedSpace || isSearchMode) {
      return;
    }
    await fetchSpaceContents(selectedSpace.id, selectedPath);
  }, [fetchSpaceContents, isSearchMode, selectedPath, selectedSpace]);

  const {
    isTrashModalOpen,
    trashItems,
    selectedTrashIds,
    isTrashLoading,
    isTrashProcessing,
    setSelectedTrashIds,
    handleCloseTrash,
    handleTrashRestoreConfirm,
    handleTrashDeleteConfirm,
    handleTrashEmptyConfirm,
  } = useTrashModalManager({
    selectedSpace,
    isSearchMode,
    trashOpenRequest,
    clearTrashOpenRequest,
    fetchTrashItems,
    handleTrashRestore,
    handleTrashDelete,
    handleTrashEmpty,
    refreshCurrentFolder,
  });

  const isAnyModalOpen =
    modals.destination.visible || modals.rename.visible || modals.createFolder.visible || isTrashModalOpen;

  // Box selection (Grid 뷰 전용)
  const { isSelecting, selectionBox, wasRecentlySelecting } = useBoxSelection({
    enabled: !isSearchMode && viewMode === 'grid' && !isTouchInteraction && !isAnyModalOpen,
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
      clearDesktopTouchTapState();
    };
  }, [clearDesktopTouchTapState, clearLongPressTimer]);

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

  useLayoutEffect(() => {
    const rootElement = rootContainerRef.current;
    const selectionElement = selectionContainerRef.current;
    if (!rootElement || !selectionElement) {
      const resetFrame = window.requestAnimationFrame(() => {
        setOverlayOffset((prev) => (prev.x === 0 && prev.y === 0 ? prev : { x: 0, y: 0 }));
      });
      return () => {
        window.cancelAnimationFrame(resetFrame);
      };
    }

    const updateOffsets = () => {
      const rootRect = rootElement.getBoundingClientRect();
      const selectionRect = selectionElement.getBoundingClientRect();
      const nextX = (selectionRect.left - rootRect.left) - selectionElement.scrollLeft;
      const nextY = (selectionRect.top - rootRect.top) - selectionElement.scrollTop;
      setOverlayOffset((prev) => (
        prev.x === nextX && prev.y === nextY ? prev : { x: nextX, y: nextY }
      ));
    };

    const frame = window.requestAnimationFrame(() => {
      updateOffsets();
    });

    const resizeObserver = new ResizeObserver(() => {
      updateOffsets();
    });
    resizeObserver.observe(rootElement);
    resizeObserver.observe(selectionElement);

    selectionElement.addEventListener('scroll', updateOffsets, { passive: true });
    window.addEventListener('resize', updateOffsets);

    return () => {
      window.cancelAnimationFrame(frame);
      resizeObserver.disconnect();
      selectionElement.removeEventListener('scroll', updateOffsets);
      window.removeEventListener('resize', updateOffsets);
    };
  }, [selectedSpace]);

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
    !isSearchMode && isTouchInteraction && selectedItems.size > 0;
  const showDesktopSelectionBar = !isSearchMode && !isTouchInteraction && selectedItems.size > 0;
  const topRowHeight = isMobileLayout ? 44 : 52;
  const topRowOffset = 8;
  const topRowSlotHeight = topRowHeight + topRowOffset;
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

  const hasContent = sortedContent.length > 0;
  const shouldShowCenteredLoading = activeLoading && !hasContent;
  const shouldShowInlineError = !activeLoading && hasContent && Boolean(activeErrorMessage);
  const shouldShowFullError = !activeLoading && !hasContent && Boolean(activeErrorMessage);
  const shouldShowEmpty = !activeLoading && !activeErrorMessage && !hasContent;
  const shouldShowLoadingOverlay = !isSearchMode && isLoading && hasContent;
  const showSearchSummary = isSearchMode && searchSource.hasEnoughQuery && hasContent;
  const showSearchLoadMore = showSearchSummary && searchSource.hasMore && searchSource.canLoadMore;

  const handleRetryContentLoad = useCallback(() => {
    if (isSearchMode || !selectedSpace) {
      return;
    }
    void fetchSpaceContents(selectedSpace.id, selectedPath);
  }, [fetchSpaceContents, isSearchMode, selectedPath, selectedSpace]);

  const handleMobileLongPressStart = useCallback((record: FileNode) => {
    if (isSearchMode || !isTouchInteraction || isMobileSelectionMode) {
      return;
    }
    clearLongPressTimer();
    longPressTimerRef.current = setTimeout(() => {
      clearDesktopTouchTapState();
      lastLongPressRef.current = {
        path: record.path,
        expiresAt: Date.now() + 900,
      };
      setIsMobileSelectionMode(true);
      setSelection(new Set([record.path]));
    }, LONG_PRESS_DURATION_MS);
  }, [isMobileSelectionMode, isSearchMode, isTouchInteraction, clearLongPressTimer, clearDesktopTouchTapState, setSelection]);

  const handleTouchInteractionEnd = useCallback(() => {
    clearLongPressTimer();
  }, [clearLongPressTimer]);

  const handleSelectionTouchStartCapture = useCallback((event: React.TouchEvent<HTMLDivElement>) => {
    if (!isTouchInteraction && !isDesktopTouchFallbackEnabled) {
      return;
    }
    const touch = event.touches[0];
    if (!touch) {
      return;
    }
    if (isDesktopTouchFallbackEnabled) {
      recentDesktopTouchRef.current = Date.now();
    }
    touchStartPointRef.current = { x: touch.clientX, y: touch.clientY };
    isTouchPanningRef.current = false;
  }, [isDesktopTouchFallbackEnabled, isTouchInteraction]);

  const handleSelectionTouchMoveCapture = useCallback((event: React.TouchEvent<HTMLDivElement>) => {
    if (!isTouchInteraction && !isDesktopTouchFallbackEnabled) {
      return;
    }
    const touch = event.touches[0];
    const startPoint = touchStartPointRef.current;
    if (!touch || !startPoint) {
      return;
    }
    const deltaX = Math.abs(touch.clientX - startPoint.x);
    const deltaY = Math.abs(touch.clientY - startPoint.y);
    if (deltaX > TOUCH_PAN_THRESHOLD_PX || deltaY > TOUCH_PAN_THRESHOLD_PX) {
      isTouchPanningRef.current = true;
      clearLongPressTimer();
      clearDesktopTouchTapState();
    }
  }, [isDesktopTouchFallbackEnabled, isTouchInteraction, clearDesktopTouchTapState, clearLongPressTimer]);

  const handleSelectionTouchEndCapture = useCallback(() => {
    if (!isTouchInteraction && !isDesktopTouchFallbackEnabled) {
      return;
    }
    clearLongPressTimer();
    if (isTouchPanningRef.current) {
      suppressTapUntilRef.current = Math.max(suppressTapUntilRef.current, Date.now() + 250);
      clearDesktopTouchTapState();
    }
    isTouchPanningRef.current = false;
    touchStartPointRef.current = null;
  }, [isDesktopTouchFallbackEnabled, isTouchInteraction, clearDesktopTouchTapState, clearLongPressTimer]);

  const handleItemActivate = useCallback((path: string) => {
    const record = contentByPath.get(path);
    if (!record) {
      return;
    }

    if (record.isDir) {
      handleNavigate(path);
      return;
    }

    void handleBulkDownload([record.path]);
  }, [contentByPath, handleBulkDownload, handleNavigate]);

  const handleItemDoubleClick = useCallback((path: string) => {
    const guard = desktopTouchDoubleClickGuardRef.current;
    if (guard?.path === path) {
      if (Date.now() <= guard.expiresAt) {
        return;
      }
      desktopTouchDoubleClickGuardRef.current = null;
    }

    handleItemActivate(path);
  }, [handleItemActivate]);

  const handleItemTap = useCallback((e: React.MouseEvent<HTMLElement>, record: FileNode, index: number) => {
    const target = e.target as HTMLElement;
    if (target.closest('[data-item-action="true"]')) {
      clearDesktopTouchTapState();
      e.preventDefault();
      e.stopPropagation();
      return;
    }

    if (Date.now() < suppressTapUntilRef.current) {
      clearDesktopTouchTapState();
      e.preventDefault();
      e.stopPropagation();
      return;
    }

    if (isAnyModalOpen) {
      clearDesktopTouchTapState();
      e.preventDefault();
      e.stopPropagation();
      return;
    }

    if (isSearchMode) {
      clearDesktopTouchTapState();
      openSearchResultByRecordPath(record.path);
      return;
    }

    if (!isTouchInteraction) {
      const isDesktopTouchEvent = isDesktopTouchFallbackEnabled
        && (Date.now() - recentDesktopTouchRef.current <= DESKTOP_TOUCH_EVENT_WINDOW_MS);
      const hasModifier = e.altKey || e.ctrlKey || e.metaKey || e.shiftKey;
      if (isDesktopTouchEvent && !hasModifier) {
        const now = Date.now();
        const lastTap = desktopTouchTapRef.current;
        if (lastTap && lastTap.path === record.path && (now - lastTap.at) <= DESKTOP_TOUCH_DOUBLE_TAP_WINDOW_MS) {
          clearDesktopTouchTapState();
          desktopTouchDoubleClickGuardRef.current = {
            path: record.path,
            expiresAt: now + DESKTOP_TOUCH_DOUBLE_CLICK_GUARD_MS,
          };
          handleItemActivate(record.path);
          return;
        }
        desktopTouchTapRef.current = { path: record.path, at: now };
      } else {
        clearDesktopTouchTapState();
      }
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
      clearDesktopTouchTapState();
      toggleMobileSelection(record.path);
      return;
    }

    if (record.isDir) {
      clearDesktopTouchTapState();
      handleNavigate(record.path);
      handleClearSelection();
      return;
    }

    clearDesktopTouchTapState();
    setSelection(new Set([record.path]), index);
    setIsMobileActionsOpen(false);
  }, [
    clearDesktopTouchTapState,
    isAnyModalOpen,
    isDesktopTouchFallbackEnabled,
    isTouchInteraction,
    isMobileSelectionMode,
    isSearchMode,
    handleItemActivate,
    handleClearSelection,
    handleItemClick,
    handleNavigate,
    openSearchResultByRecordPath,
    setSelection,
    sortedContent,
    toggleMobileSelection,
  ]);

  const handleItemContextMenu = useCallback((e: React.MouseEvent<HTMLElement>, record: FileNode) => {
    if (isSearchMode || isTouchInteraction || isAnyModalOpen) {
      e.preventDefault();
      return;
    }
    handleContextMenu(e, record);
  }, [isSearchMode, isTouchInteraction, isAnyModalOpen, handleContextMenu]);

  const handleContainerContextMenu = useCallback((e: React.MouseEvent) => {
    if (isSearchMode || isTouchInteraction || isAnyModalOpen) {
      e.preventDefault();
      return;
    }
    handleEmptyAreaContextMenu(e);
  }, [isSearchMode, isTouchInteraction, isAnyModalOpen, handleEmptyAreaContextMenu]);

  // Modal wrapper handlers
  const handleRenameConfirm = async () => {
    const { record, newName } = modals.rename.data;
    if (!record || !newName.trim()) {
      message.error(t('folderContent.renameRequired'));
      return;
    }
    await performRename(record.path, newName.trim());
    closeModal('rename');
    handleClearSelection();
  };

  const handleCreateFolderConfirm = async () => {
    const { folderName } = modals.createFolder.data;
    if (!folderName.trim()) {
      message.error(t('folderContent.folderNameRequired'));
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
      void handleFileUpload(Array.from(files), selectedPath);
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
    if (isTouchInteraction && isMobileSelectionMode) {
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

  // Browse 모드에서만 Space 선택 필수
  if (!selectedSpace && !isSearchMode) {
    return (
      <div style={{ height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <Empty description={t('folderContent.selectFolderFromTree')} />
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
        multiple
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
                e.stopPropagation();
                suppressTapUntilRef.current = Date.now() + 700;
              }
            }}
            onTouchStartCapture={(e) => {
              const target = e.target as HTMLElement;
              const isButtonTarget = Boolean(target.closest('button'));
              if (!isButtonTarget) {
                e.stopPropagation();
                suppressTapUntilRef.current = Date.now() + 700;
              }
            }}
            onClickCapture={(e) => {
              const target = e.target as HTMLElement;
              const isButtonTarget = Boolean(target.closest('button'));
              if (!isButtonTarget) {
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
              aria-label={t('folderContent.clearSelection')}
              title={t('folderContent.clearSelection')}
              onClick={() => {
                armToolbarInteractionGuard();
                handleClearSelection();
              }}
            />
            <span style={{ fontWeight: 600, color: token.colorText, whiteSpace: 'nowrap' }}>
              {t('folderContent.selectedCount', { count: selectedItems.size })}
            </span>
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
            <Button
              size="small"
              icon={<CloseOutlined />}
              aria-label={t('folderContent.clearSelection')}
              title={t('folderContent.clearSelection')}
              onClick={handleClearSelection}
            />
            <span style={{ fontWeight: 600, color: token.colorText, whiteSpace: 'nowrap' }}>
              {t('folderContent.selectedCount', { count: selectedItems.size })}
            </span>
            <Button size="small" icon={<DownloadOutlined />} onClick={() => handleBulkDownload(Array.from(selectedItems))}>
              {t('folderContent.download')}
            </Button>
            {canWriteFiles && (
              <Button size="small" icon={<CopyOutlined />} onClick={() => openModal('destination', { mode: 'copy', sources: Array.from(selectedItems) })}>
                {t('folderContent.copy')}
              </Button>
            )}
            {canWriteFiles && (
              <Button size="small" icon={moveActionIcon} onClick={() => openModal('destination', { mode: 'move', sources: Array.from(selectedItems) })}>
                {t('folderContent.move')}
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
                {t('folderContent.rename')}
              </Button>
            )}
            {canWriteFiles && (
              <Button size="small" icon={<DeleteOutlined />} danger onClick={() => handleBulkDelete(Array.from(selectedItems))}>
                {t('folderContent.moveToTrash')}
              </Button>
            )}
          </div>
        ) : (
          <div style={{ height: topRowHeight }}>
              <FolderContentToolbar
                viewMode={effectiveViewMode}
                sortConfig={sortConfig}
                canUpload={canWriteFiles}
                canGoBack={canGoBack}
                canGoForward={canGoForward}
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
        onScroll={isTouchInteraction || isDesktopTouchFallbackEnabled ? handleTouchInteractionEnd : undefined}
        onTouchStartCapture={isTouchInteraction || isDesktopTouchFallbackEnabled ? handleSelectionTouchStartCapture : undefined}
        onTouchMoveCapture={isTouchInteraction || isDesktopTouchFallbackEnabled ? handleSelectionTouchMoveCapture : undefined}
        onTouchEndCapture={isTouchInteraction || isDesktopTouchFallbackEnabled ? handleSelectionTouchEndCapture : undefined}
        onTouchCancelCapture={isTouchInteraction || isDesktopTouchFallbackEnabled ? handleSelectionTouchEndCapture : undefined}
        style={{
          position: 'relative',
          flex: 1,
          minWidth: 0,
          minHeight: 0,
          overflowY: 'auto',
          overflowX: 'hidden',
          touchAction: isTouchInteraction ? 'pan-y' : undefined,
          WebkitOverflowScrolling: 'touch',
          paddingBottom: PATH_BAR_CONTENT_OVERLAY_HEIGHT + 6,
        }}
      >
        {shouldShowCenteredLoading ? (
          <div style={{ height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <Spin size="large" />
          </div>
        ) : shouldShowFullError ? (
          <div style={{ height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <Empty description={activeErrorMessage ?? (isSearchMode ? t('folderContent.loadSearchFailed') : t('folderContent.loadFolderFailed'))}>
              {!isSearchMode && (
                <Button size="small" onClick={handleRetryContentLoad}>{t('folderContent.retry')}</Button>
              )}
            </Empty>
          </div>
        ) : shouldShowEmpty ? (
          <div style={{ height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <Empty
              description={
                isSearchMode
                  ? (searchSource.hasEnoughQuery ? t('folderContent.noSearchResults') : searchModeHelpText)
                  : t('folderContent.folderEmpty')
              }
            />
          </div>
        ) : (
          <div style={{ position: 'relative', minHeight: '100%' }}>
            {shouldShowInlineError && (
              <div style={{ marginBottom: 12 }}>
                <Alert
                  type="warning"
                  showIcon
                  title={isSearchMode ? t('folderContent.partialSearchFailed') : t('folderContent.latestFolderLoadFailed')}
                  description={activeErrorMessage ?? undefined}
                  action={
                    !isSearchMode
                      ? <Button size="small" onClick={handleRetryContentLoad}>{t('folderContent.retryShort')}</Button>
                      : undefined
                  }
                />
              </div>
            )}
            <TransferPanel
              isMobile={isMobileLayout}
              onCancelUpload={cancelUpload}
            />
            {showSearchSummary && (
              <div
                style={{
                  marginBottom: 12,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  gap: 12,
                  flexWrap: 'wrap',
                }}
              >
                <span style={{ fontSize: 13, fontWeight: 600, color: token.colorTextSecondary }}>
                  {t('folderContent.searchSummary', {
                    count: searchSource.resultCount,
                  })}
                </span>
                {showSearchLoadMore && (
                  <Button size="small" onClick={searchSource.loadMore}>
                    {t('folderContent.searchLoadMore')}
                  </Button>
                )}
              </div>
            )}
            {effectiveViewMode === 'table' ? (
              <FolderContentTable
                dataSource={sortedContent}
                loading={false}
                selectedItems={isSearchMode ? EMPTY_SELECTION : selectedItems}
                dragOverFolder={isSearchMode ? null : dragOverFolder}
                onItemClick={handleItemTap}
                onItemDoubleClick={isSearchMode ? openSearchResultByRecordPath : handleItemDoubleClick}
                onItemTouchStart={(record) => handleMobileLongPressStart(record)}
                onItemTouchEnd={handleTouchInteractionEnd}
                onItemTouchCancel={handleTouchInteractionEnd}
                onContextMenu={handleItemContextMenu}
                onItemDragStart={handleItemDragStart}
                onItemDragEnd={handleItemDragEnd}
                onFolderDragOver={handleFolderDragOver}
                onFolderDragLeave={handleFolderDragLeave}
                onFolderDrop={handleFolderDrop}
                disableDrag
                canWriteFiles={canWriteFiles}
                onItemDownload={isSearchMode ? undefined : (record) => handleBulkDownload([record.path])}
                onItemCopy={isSearchMode ? undefined : (record) => openModal('destination', { mode: 'copy', sources: [record.path] })}
                onItemMove={isSearchMode ? undefined : (record) => openModal('destination', { mode: 'move', sources: [record.path] })}
                onItemRename={isSearchMode ? undefined : (record) => openModal('rename', { record, newName: record.name })}
                onItemDelete={isSearchMode ? undefined : handleDelete}
                showActions={!isSearchMode}
                rowKeyResolver={isSearchMode ? ((record) => record.path) : undefined}
                renderName={isSearchMode ? renderSearchName : undefined}
                renderMeta={isSearchMode ? renderSearchMeta : undefined}
                emptyText={isSearchMode ? (searchSource.hasEnoughQuery ? t('folderContent.noSearchResults') : searchModeHelpText) : undefined}
              />
            ) : (
              <FolderContentGrid
                dataSource={sortedContent}
                loading={false}
                selectedItems={isSearchMode ? EMPTY_SELECTION : selectedItems}
                dragOverFolder={isSearchMode ? null : dragOverFolder}
                onItemClick={handleItemTap}
                onItemDoubleClick={isSearchMode ? openSearchResultByRecordPath : handleItemDoubleClick}
                onItemTouchStart={(record) => handleMobileLongPressStart(record)}
                onItemTouchEnd={handleTouchInteractionEnd}
                onItemTouchCancel={handleTouchInteractionEnd}
                onContextMenu={handleItemContextMenu}
                onItemDragStart={handleItemDragStart}
                onItemDragEnd={handleItemDragEnd}
                onFolderDragOver={handleFolderDragOver}
                onFolderDragLeave={handleFolderDragLeave}
                onFolderDrop={handleFolderDrop}
                itemsRef={itemsRef}
                disableDraggable={isSearchMode || isSelecting || isTouchInteraction || !canWriteFiles}
                renderName={isSearchMode ? renderSearchName : undefined}
                renderMeta={isSearchMode ? renderSearchGridMeta : undefined}
                emptyText={isSearchMode ? (searchSource.hasEnoughQuery ? t('folderContent.noSearchResults') : searchModeHelpText) : undefined}
                spaceId={isSearchMode ? undefined : selectedSpace?.id}
              />
            )}
            {shouldShowLoadingOverlay && (
              <div
                style={{
                  position: 'absolute',
                  inset: 0,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  background: 'var(--ant-color-bg-container, rgba(255, 255, 255, 0.52))',
                  pointerEvents: 'none',
                  zIndex: 3,
                }}
              >
                <Spin size="small" />
              </div>
            )}
          </div>
        )}
      </div>

      <BoxSelectionOverlay
        visible={isSelecting && selectionBox !== null}
        startX={selectionBox?.startX ?? 0}
        startY={selectionBox?.startY ?? 0}
        currentX={selectionBox?.currentX ?? 0}
        currentY={selectionBox?.currentY ?? 0}
        offsetX={overlayOffset.x}
        offsetY={overlayOffset.y}
      />

      <div
        data-path-bar="true"
        data-selection-exclude="true"
        onPointerDown={(e) => e.stopPropagation()}
        onClick={(e) => e.stopPropagation()}
        onContextMenu={(e) => {
          e.preventDefault();
          e.stopPropagation();
        }}
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
        currentSpace={selectedSpace}
        onConfirm={modals.destination.data.mode === 'move' ? handleMoveConfirm : handleCopyConfirm}
        onCancel={handleDestinationCancel}
      />

      <TrashModal
        open={isTrashModalOpen}
        spaceName={selectedSpace?.space_name}
        items={trashItems}
        selectedIds={selectedTrashIds}
        loading={isTrashLoading}
        processing={isTrashProcessing}
        onSelectionChange={(ids) => setSelectedTrashIds(ids)}
        onRestore={handleTrashRestoreConfirm}
        onDelete={handleTrashDeleteConfirm}
        onEmpty={handleTrashEmptyConfirm}
        onClose={handleCloseTrash}
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
            {t('folderContent.selectedCount', { count: selectedItems.size })}
          </div>
          <Menu
            selectable={false}
            items={[
              {
                key: 'download',
                icon: <DownloadOutlined />,
                label: t('folderContent.download'),
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
                    label: t('folderContent.copy'),
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
                    label: t('folderContent.move'),
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
                    label: t('folderContent.rename'),
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
                    label: t('folderContent.moveToTrash'),
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
