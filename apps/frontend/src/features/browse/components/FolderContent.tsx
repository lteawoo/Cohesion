import React, { useEffect, useState, useRef, useCallback, useLayoutEffect, useMemo } from 'react';
import { Empty, App, Grid, Button, Menu, theme, Breadcrumb, Spin, Alert } from 'antd';
import { DownloadOutlined, CopyOutlined, DeleteOutlined, EditOutlined, CloseOutlined, MoreOutlined } from '@ant-design/icons';
import { useLocation, useNavigate } from 'react-router';
import { useBrowseStore } from '@/stores/browseStore';
import type { FileNode, ViewMode, SortConfig } from '../types';
import { useFileSelection } from '../hooks/useFileSelection';
import { useBreadcrumb } from '../hooks/useBreadcrumb';
import { useFileOperations, type TrashItem } from '../hooks/useFileOperations';
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
import TrashModal from './FolderContent/TrashModal';
import UploadOverlay from './FolderContent/UploadOverlay';
import BoxSelectionOverlay from './FolderContent/BoxSelectionOverlay';
import { useAuth } from '@/features/auth/useAuth';
import BottomSheet from '@/components/common/BottomSheet';
import { formatDate, formatSize } from '../constants';
import { useSearchExplorerSource } from '@/features/search/hooks/useSearchExplorerSource';
import type { SearchFileResult } from '@/features/search/types';
import { useTranslation } from 'react-i18next';

const LONG_PRESS_DURATION_MS = 420;
const TOUCH_PAN_THRESHOLD_PX = 8;
const PATH_BAR_HEIGHT = 36;
const EXPLORER_SIDE_PADDING = 16;
const PATH_BAR_CONTENT_OVERLAY_HEIGHT = PATH_BAR_HEIGHT - EXPLORER_SIDE_PADDING;
type NavigationState = { entries: string[]; index: number };
type BrowseLocationState = { fromSearchQuery?: string };
const EMPTY_SELECTION = new Set<string>();

function detectTouchInputSupport(): boolean {
  if (typeof window === 'undefined' || typeof navigator === 'undefined') {
    return false;
  }

  const maxTouchPoints = navigator.maxTouchPoints ?? 0;
  const hasCoarsePointer =
    typeof window.matchMedia === 'function' &&
    window.matchMedia('(any-pointer: coarse)').matches;

  return maxTouchPoints > 0 || hasCoarsePointer;
}

const FolderContent: React.FC = () => {
  const { t } = useTranslation();
  const { message, modal } = App.useApp();
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
  const [navigationState, setNavigationState] = useState<NavigationState>({ entries: [], index: -1 });
  const [isTrashModalOpen, setIsTrashModalOpen] = useState(false);
  const [trashItems, setTrashItems] = useState<TrashItem[]>([]);
  const [selectedTrashIds, setSelectedTrashIds] = useState<number[]>([]);
  const [isTrashLoading, setIsTrashLoading] = useState(false);
  const [isTrashProcessing, setIsTrashProcessing] = useState(false);
  const [hasTouchInput, setHasTouchInput] = useState<boolean>(() => detectTouchInputSupport());
  const longPressTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const lastLongPressRef = useRef<{ path: string; expiresAt: number } | null>(null);
  const suppressTapUntilRef = useRef(0);
  const touchStartPointRef = useRef<{ x: number; y: number } | null>(null);
  const isTouchPanningRef = useRef(false);
  const selectedItemsRef = useRef<Set<string>>(new Set());
  const historySpaceIdRef = useRef<number | undefined>(undefined);
  const isHistoryTraversalRef = useRef(false);
  const interactionMode = hasTouchInput ? 'touch' : 'pointer';
  const isMobileLayout = layoutMode === 'mobile';
  const isTouchInteraction = interactionMode === 'touch';

  // Modal management
  const { modals, openModal, closeModal, updateModalData } = useModalManager();
  const prevNavRef = useRef<{ path: string; spaceId?: number }>({ path: '', spaceId: undefined });

  // Custom hooks
  const { selectedItems, handleItemClick, setSelection, clearSelection } = useFileSelection();
  const searchSource = useSearchExplorerSource(isSearchMode);
  const searchItemsByRowPath = useMemo(() => {
    const next = new Map<string, SearchFileResult>();
    searchSource.results.forEach((item) => {
      next.set(`${item.spaceId}::${item.path}`, item);
    });
    return next;
  }, [searchSource.results]);
  const searchContent = useMemo<FileNode[]>(() => {
    return searchSource.results.map((item) => ({
      name: item.name,
      path: `${item.spaceId}::${item.path}`,
      isDir: item.isDir,
      modTime: item.modTime,
      size: item.size,
    }));
  }, [searchSource.results]);
  const sourceContent = isSearchMode ? searchContent : content;

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
    fetchTrashItems,
    handleTrashRestore,
    handleTrashDelete,
    handleTrashEmpty,
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

  useEffect(() => {
    let frame: number | null = null;
    const scheduleNavigationUpdate = (updater: React.SetStateAction<NavigationState>) => {
      frame = window.requestAnimationFrame(() => {
        setNavigationState(updater);
      });
    };

    if (isSearchMode) {
      scheduleNavigationUpdate({ entries: [], index: -1 });
      historySpaceIdRef.current = undefined;
      return () => {
        if (frame !== null) {
          window.cancelAnimationFrame(frame);
        }
      };
    }

    if (!selectedSpace) {
      scheduleNavigationUpdate({ entries: [], index: -1 });
      historySpaceIdRef.current = undefined;
      return () => {
        if (frame !== null) {
          window.cancelAnimationFrame(frame);
        }
      };
    }

    if (historySpaceIdRef.current !== selectedSpace.id) {
      historySpaceIdRef.current = selectedSpace.id;
      scheduleNavigationUpdate({ entries: [selectedPath], index: 0 });
      isHistoryTraversalRef.current = false;
      return () => {
        if (frame !== null) {
          window.cancelAnimationFrame(frame);
        }
      };
    }

    if (isHistoryTraversalRef.current) {
      isHistoryTraversalRef.current = false;
      return () => {
        if (frame !== null) {
          window.cancelAnimationFrame(frame);
        }
      };
    }

    scheduleNavigationUpdate((prev) => {
      if (prev.index >= 0 && prev.entries[prev.index] === selectedPath) {
        return prev;
      }
      const entries = prev.index >= 0
        ? [...prev.entries.slice(0, prev.index + 1), selectedPath]
        : [selectedPath];
      return { entries, index: entries.length - 1 };
    });

    return () => {
      if (frame !== null) {
        window.cancelAnimationFrame(frame);
      }
    };
  }, [isSearchMode, selectedPath, selectedSpace]);

  const handleGoBack = useCallback(() => {
    if (navigationState.index > 0) {
      const nextIndex = navigationState.index - 1;
      const targetPath = navigationState.entries[nextIndex];
      isHistoryTraversalRef.current = true;
      setNavigationState((prev) => ({ ...prev, index: nextIndex }));
      handleNavigate(targetPath);
      return;
    }

    if (!incomingSearchQuery) {
      return;
    }

    navigate(`/search?q=${encodeURIComponent(incomingSearchQuery)}`);
  }, [handleNavigate, incomingSearchQuery, navigate, navigationState]);

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
    if (!selectedSpace || isSearchMode) {
      setIsTrashModalOpen(false);
      setTrashItems([]);
      setSelectedTrashIds([]);
    }
  }, [isSearchMode, selectedSpace]);

  useEffect(() => {
    return () => {
      clearLongPressTimer();
    };
  }, [clearLongPressTimer]);

  useEffect(() => {
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
      return;
    }

    const media = window.matchMedia('(any-pointer: coarse)');
    const updateTouchCapability = () => {
      const maxTouchPoints = typeof navigator !== 'undefined' ? (navigator.maxTouchPoints ?? 0) : 0;
      setHasTouchInput(maxTouchPoints > 0 || media.matches);
    };

    updateTouchCapability();

    if (typeof media.addEventListener === 'function') {
      media.addEventListener('change', updateTouchCapability);
      return () => {
        media.removeEventListener('change', updateTouchCapability);
      };
    }

    if (typeof media.addListener === 'function') {
      media.addListener(updateTouchCapability);
      return () => {
        media.removeListener(updateTouchCapability);
      };
    }
  }, []);

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

  const resolveSearchResult = useCallback((recordPath: string): SearchFileResult | null => {
    return searchItemsByRowPath.get(recordPath) ?? null;
  }, [searchItemsByRowPath]);

  const openSearchResultByRecordPath = useCallback((recordPath: string) => {
    const item = resolveSearchResult(recordPath);
    if (!item) {
      return;
    }
    searchSource.openResult(item);
  }, [resolveSearchResult, searchSource]);

  const renderSearchMeta = useCallback((record: FileNode) => {
    const item = resolveSearchResult(record.path);
    if (!item) {
      return `${record.isDir ? '-' : formatSize(record.size)} | ${formatDate(record.modTime)}`;
    }
    const sizeText = item.isDir ? '-' : formatSize(item.size);
    return `${sizeText} | ${formatDate(item.modTime)} | ${item.spaceName}`;
  }, [resolveSearchResult]);

  const isSearching = isSearchMode && searchSource.isSearching;
  const activeErrorMessage = isSearchMode
    ? searchSource.errorMessage
    : (browseError?.message ?? null);
  const activeLoading = isSearchMode ? isSearching : isLoading;
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
      lastLongPressRef.current = {
        path: record.path,
        expiresAt: Date.now() + 900,
      };
      setIsMobileSelectionMode(true);
      setSelection(new Set([record.path]));
    }, LONG_PRESS_DURATION_MS);
  }, [isMobileSelectionMode, isSearchMode, isTouchInteraction, clearLongPressTimer, setSelection]);

  const handleMobileLongPressEnd = useCallback(() => {
    clearLongPressTimer();
  }, [clearLongPressTimer]);

  const handleSelectionTouchStartCapture = useCallback((event: React.TouchEvent<HTMLDivElement>) => {
    if (!isTouchInteraction) {
      return;
    }
    const touch = event.touches[0];
    if (!touch) {
      return;
    }
    touchStartPointRef.current = { x: touch.clientX, y: touch.clientY };
    isTouchPanningRef.current = false;
  }, [isTouchInteraction]);

  const handleSelectionTouchMoveCapture = useCallback((event: React.TouchEvent<HTMLDivElement>) => {
    if (!isTouchInteraction) {
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
    }
  }, [isTouchInteraction, clearLongPressTimer]);

  const handleSelectionTouchEndCapture = useCallback(() => {
    if (!isTouchInteraction) {
      return;
    }
    clearLongPressTimer();
    if (isTouchPanningRef.current) {
      suppressTapUntilRef.current = Math.max(suppressTapUntilRef.current, Date.now() + 250);
    }
    isTouchPanningRef.current = false;
    touchStartPointRef.current = null;
  }, [isTouchInteraction, clearLongPressTimer]);

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

    if (isSearchMode) {
      openSearchResultByRecordPath(record.path);
      return;
    }

    if (!isTouchInteraction) {
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
  }, [
    isAnyModalOpen,
    isTouchInteraction,
    isMobileSelectionMode,
    isSearchMode,
    handleClearSelection,
    handleItemClick,
    handleNavigate,
    openSearchResultByRecordPath,
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

  const refreshCurrentFolder = useCallback(async () => {
    if (!selectedSpace || isSearchMode) {
      return;
    }
    await fetchSpaceContents(selectedSpace.id, selectedPath);
  }, [fetchSpaceContents, isSearchMode, selectedPath, selectedSpace]);

  const loadTrashItems = useCallback(async () => {
    if (!selectedSpace) {
      return;
    }
    setIsTrashLoading(true);
    try {
      const items = await fetchTrashItems();
      setTrashItems(items);
    } catch (error) {
      message.error(error instanceof Error ? error.message : t('folderContent.trashListLoadFailed'));
    } finally {
      setIsTrashLoading(false);
    }
  }, [fetchTrashItems, message, selectedSpace, t]);

  const handleOpenTrash = useCallback(() => {
    if (!selectedSpace) {
      message.error(t('folderContent.noSelectedSpace'));
      return;
    }
    setIsTrashModalOpen(true);
    setSelectedTrashIds([]);
    void loadTrashItems();
  }, [loadTrashItems, message, selectedSpace, t]);

  useEffect(() => {
    if (!trashOpenRequest || !selectedSpace) {
      return;
    }
    if (trashOpenRequest.spaceId !== selectedSpace.id) {
      return;
    }
    handleOpenTrash();
    clearTrashOpenRequest();
  }, [clearTrashOpenRequest, handleOpenTrash, selectedSpace, trashOpenRequest]);

  const handleCloseTrash = useCallback(() => {
    if (isTrashProcessing) {
      return;
    }
    setIsTrashModalOpen(false);
    setSelectedTrashIds([]);
  }, [isTrashProcessing]);

  const handleTrashRestoreConfirm = useCallback(() => {
    if (selectedTrashIds.length === 0) {
      message.warning(t('folderContent.selectRestoreItems'));
      return;
    }

    modal.confirm({
      title: t('folderContent.restoreConfirmTitle'),
      content: t('folderContent.restoreConfirmContent', { count: selectedTrashIds.length }),
      okText: t('folderContent.restore'),
      cancelText: t('folderContent.cancel'),
      onOk: async () => {
        setIsTrashProcessing(true);
        try {
          await handleTrashRestore(selectedTrashIds);
          await loadTrashItems();
          await refreshCurrentFolder();
          setSelectedTrashIds([]);
        } finally {
          setIsTrashProcessing(false);
        }
      },
    });
  }, [
    handleTrashRestore,
    loadTrashItems,
    message,
    modal,
    refreshCurrentFolder,
    selectedTrashIds,
    t,
  ]);

  const handleTrashDeleteConfirm = useCallback(() => {
    if (selectedTrashIds.length === 0) {
      message.warning(t('folderContent.selectPermanentDeleteItems'));
      return;
    }

    modal.confirm({
      title: t('folderContent.permanentDeleteConfirmTitle'),
      content: t('folderContent.permanentDeleteConfirmContent', { count: selectedTrashIds.length }),
      okText: t('folderContent.permanentDelete'),
      okType: 'danger',
      cancelText: t('folderContent.cancel'),
      onOk: async () => {
        setIsTrashProcessing(true);
        try {
          await handleTrashDelete(selectedTrashIds);
          await loadTrashItems();
          await refreshCurrentFolder();
          setSelectedTrashIds([]);
        } finally {
          setIsTrashProcessing(false);
        }
      },
    });
  }, [
    handleTrashDelete,
    loadTrashItems,
    message,
    modal,
    refreshCurrentFolder,
    selectedTrashIds,
    t,
  ]);

  const handleTrashEmptyConfirm = useCallback(() => {
    if (trashItems.length === 0) {
      message.info(t('folderContent.trashEmptyInfo'));
      return;
    }

    modal.confirm({
      title: t('folderContent.emptyTrashConfirmTitle'),
      content: t('folderContent.emptyTrashConfirmContent', { count: trashItems.length }),
      okText: t('folderContent.emptyTrash'),
      okType: 'danger',
      cancelText: t('folderContent.cancel'),
      onOk: async () => {
        setIsTrashProcessing(true);
        try {
          await handleTrashEmpty();
          await loadTrashItems();
          await refreshCurrentFolder();
          setSelectedTrashIds([]);
        } finally {
          setIsTrashProcessing(false);
        }
      },
    });
  }, [
    handleTrashEmpty,
    loadTrashItems,
    message,
    modal,
    refreshCurrentFolder,
    trashItems.length,
    t,
  ]);

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
                canGoBack={navigationState.index > 0 || (!isSearchMode && Boolean(incomingSearchQuery))}
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
        onScroll={isTouchInteraction ? handleMobileLongPressEnd : undefined}
        onTouchStartCapture={isTouchInteraction ? handleSelectionTouchStartCapture : undefined}
        onTouchMoveCapture={isTouchInteraction ? handleSelectionTouchMoveCapture : undefined}
        onTouchEndCapture={isTouchInteraction ? handleSelectionTouchEndCapture : undefined}
        onTouchCancelCapture={isTouchInteraction ? handleSelectionTouchEndCapture : undefined}
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
                  message={isSearchMode ? t('folderContent.partialSearchFailed') : t('folderContent.latestFolderLoadFailed')}
                  description={activeErrorMessage ?? undefined}
                  action={
                    !isSearchMode
                      ? <Button size="small" onClick={handleRetryContentLoad}>{t('folderContent.retryShort')}</Button>
                      : undefined
                  }
                />
              </div>
            )}
            {effectiveViewMode === 'table' ? (
              <FolderContentTable
                dataSource={sortedContent}
                loading={false}
                selectedItems={isSearchMode ? EMPTY_SELECTION : selectedItems}
                dragOverFolder={isSearchMode ? null : dragOverFolder}
                onItemClick={handleItemTap}
                onItemDoubleClick={isSearchMode ? openSearchResultByRecordPath : handleNavigate}
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
                onItemDownload={isSearchMode ? undefined : (record) => handleBulkDownload([record.path])}
                onItemCopy={isSearchMode ? undefined : (record) => openModal('destination', { mode: 'copy', sources: [record.path] })}
                onItemMove={isSearchMode ? undefined : (record) => openModal('destination', { mode: 'move', sources: [record.path] })}
                onItemRename={isSearchMode ? undefined : (record) => openModal('rename', { record, newName: record.name })}
                onItemDelete={isSearchMode ? undefined : handleDelete}
                showActions={!isSearchMode}
                rowKeyResolver={isSearchMode ? ((record) => record.path) : undefined}
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
                onItemDoubleClick={isSearchMode ? openSearchResultByRecordPath : handleNavigate}
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
                disableDraggable={isSearchMode || isSelecting || isTouchInteraction || !canWriteFiles}
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
