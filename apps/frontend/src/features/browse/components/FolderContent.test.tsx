import { render } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { MouseEvent as ReactMouseEvent } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useTransferCenterStore } from '@/stores/transferCenterStore';
import FolderContent from './FolderContent';

const h = vi.hoisted(() => {
  const mockNavigate = vi.fn();
  const mockSetPath = vi.fn();
  const mockFetchSpaceContents = vi.fn();
  const mockClearTrashOpenRequest = vi.fn();
  const mockHandleBulkDownload = vi.fn();
  const mockHandleItemClick = vi.fn();
  const mockSetSelection = vi.fn();
  const mockClearSelection = vi.fn();
  const mockOpenSearchResult = vi.fn();
  const mockLoadMoreSearchResults = vi.fn();
  const mockCancelUpload = vi.fn();
  const mockDismissTransfer = vi.fn();
  const transferItems: Array<{ id: string; kind: 'upload' | 'archive'; name: string; status: string }> = [];
  const locationPathState = { pathname: '/browse' };
  const breakpointState = { lg: true };
  const selectedSpace = { id: 1, name: 'Workspace' };
  const contentItems = [
    {
      name: 'docs',
      path: '/docs',
      isDir: true,
      modTime: '2026-03-01T00:00:00.000Z',
      size: 0,
    },
    {
      name: 'report.pdf',
      path: '/report.pdf',
      isDir: false,
      modTime: '2026-03-01T00:00:00.000Z',
      size: 2048,
    },
  ];
  const storeState = {
    selectedPath: '/',
    selectedSpace,
    content: contentItems,
    isLoading: false,
    error: null,
    setPath: mockSetPath,
    fetchSpaceContents: mockFetchSpaceContents,
    trashOpenRequest: null,
    clearTrashOpenRequest: mockClearTrashOpenRequest,
  };
  const useBrowseStoreMock = vi.fn((selector: (state: typeof storeState) => unknown) => selector(storeState));
  (useBrowseStoreMock as unknown as { getState: () => typeof storeState }).getState = () => storeState;
  const searchModeState = {
    query: '',
    results: [] as Array<{ name: string; path: string }>,
    errorMessage: null as string | null,
    isSearching: false,
    resultCount: 0,
    currentLimit: 80,
    hasMore: false,
    hasEnoughQuery: false,
    canLoadMore: false,
    loadMore: mockLoadMoreSearchResults,
  };

  return {
    mockNavigate,
    mockSetPath,
    mockFetchSpaceContents,
    mockHandleBulkDownload,
    mockHandleItemClick,
    mockSetSelection,
    mockClearSelection,
    mockOpenSearchResult,
    mockLoadMoreSearchResults,
    mockCancelUpload,
    mockDismissTransfer,
    transferItems,
    locationPathState,
    breakpointState,
    selectedSpace,
    contentItems,
    storeState,
    useBrowseStoreMock,
    searchModeState,
  };
});

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

vi.mock('react-router', () => ({
  useLocation: () => ({ pathname: h.locationPathState.pathname, state: null }),
  useNavigate: () => h.mockNavigate,
}));

vi.mock('antd', async () => {
  const actual = await vi.importActual<Record<string, unknown>>('antd');
  return {
    ...actual,
    App: {
      useApp: () => ({
        message: {
          error: vi.fn(),
        },
      }),
    },
    Grid: {
      useBreakpoint: () => h.breakpointState,
    },
    theme: {
      useToken: () => ({ token: {} }),
    },
  };
});

vi.mock('@/stores/browseStore', () => ({
  useBrowseStore: h.useBrowseStoreMock,
}));

vi.mock('@/features/auth/useAuth', () => ({
  useAuth: () => ({
    user: {
      permissions: ['file.write'],
    },
  }),
}));

vi.mock('../hooks/useFileSelection', () => ({
  useFileSelection: () => ({
    selectedItems: new Set<string>(),
    handleItemClick: h.mockHandleItemClick,
    setSelection: h.mockSetSelection,
    clearSelection: h.mockClearSelection,
  }),
}));

vi.mock('../hooks/useBreadcrumb', () => ({
  useBreadcrumb: () => ({
    breadcrumbItems: [{ key: 'root', title: 'root' }],
  }),
}));

vi.mock('../hooks/useFileOperations', () => ({
  useFileOperations: () => ({
    handleRename: vi.fn(),
    handleCreateFolder: vi.fn(),
    handleDelete: vi.fn(),
    handleBulkDelete: vi.fn(),
    fetchTrashItems: vi.fn(),
    handleTrashRestore: vi.fn(),
    handleTrashDelete: vi.fn(),
    handleTrashEmpty: vi.fn(),
    handleMove: vi.fn(),
    handleCopy: vi.fn(),
    handleBulkDownload: h.mockHandleBulkDownload,
    handleFileUpload: vi.fn(),
    transfers: h.transferItems,
    cancelUpload: h.mockCancelUpload,
    dismissTransfer: h.mockDismissTransfer,
  }),
}));

vi.mock('../hooks/useDragAndDrop', () => ({
  useDragAndDrop: () => ({
    isDragging: false,
    dragOverFolder: null,
    handleItemDragStart: vi.fn(),
    handleItemDragEnd: vi.fn(),
    handleFolderDragOver: vi.fn(),
    handleFolderDragLeave: vi.fn(),
    handleFolderDrop: vi.fn(),
    handleDragEnter: vi.fn(),
    handleDragLeave: vi.fn(),
    handleDragOver: vi.fn(),
    handleDrop: vi.fn(),
  }),
}));

vi.mock('../hooks/useContextMenu', () => ({
  useContextMenu: () => ({
    handleContextMenu: vi.fn(),
    handleEmptyAreaContextMenu: vi.fn(),
  }),
}));

vi.mock('../hooks/useBoxSelection', () => ({
  useBoxSelection: () => ({
    isSelecting: false,
    selectionBox: null,
    wasRecentlySelecting: false,
  }),
}));

vi.mock('../hooks/useModalManager', () => ({
  useModalManager: () => ({
    modals: {
      destination: { visible: false, data: { mode: 'copy', sources: [] } },
      rename: { visible: false, data: { record: null, newName: '' } },
      createFolder: { visible: false, data: { folderName: '' } },
    },
    openModal: vi.fn(),
    closeModal: vi.fn(),
    updateModalData: vi.fn(),
  }),
}));

vi.mock('../hooks/useSortedContent', () => ({
  useSortedContent: (content: unknown[]) => content,
}));

vi.mock('../hooks/useBrowseHistoryNavigation', () => ({
  useBrowseHistoryNavigation: () => ({
    canGoBack: false,
    canGoForward: false,
    goBack: vi.fn(),
    goForward: vi.fn(),
  }),
}));

vi.mock('../hooks/useSearchModeContent', () => ({
  useSearchModeContent: ({ browseContent, browseErrorMessage, browseLoading }: {
    browseContent: unknown[];
    browseErrorMessage: string | null;
    browseLoading: boolean;
  }) => ({
    searchSource: h.searchModeState,
    sourceContent: browseContent,
    openSearchResultByRecordPath: h.mockOpenSearchResult,
    renderSearchName: (record: { name: string }) => record.name,
    renderSearchMeta: () => 'meta',
    renderSearchGridMeta: () => 'grid-meta',
    activeErrorMessage: browseErrorMessage,
    activeLoading: browseLoading,
  }),
}));

vi.mock('../hooks/useTrashModalManager', () => ({
  useTrashModalManager: () => ({
    isTrashModalOpen: false,
    trashItems: [],
    selectedTrashIds: [],
    isTrashLoading: false,
    isTrashProcessing: false,
    setSelectedTrashIds: vi.fn(),
    handleCloseTrash: vi.fn(),
    handleTrashRestoreConfirm: vi.fn(),
    handleTrashDeleteConfirm: vi.fn(),
    handleTrashEmptyConfirm: vi.fn(),
  }),
}));

vi.mock('./FolderContent/FolderContentToolbar', () => ({
  default: () => <div data-testid="toolbar" />,
}));

vi.mock('./FolderContent/FolderContentTable', () => ({
  default: ({ dataSource, onItemClick, onItemDoubleClick, onItemTouchStart, onItemTouchEnd, onItemTouchCancel }: {
    dataSource: Array<{ path: string; name: string }>;
    onItemClick: (event: ReactMouseEvent<HTMLElement>, record: { path: string; name: string }, index: number) => void;
    onItemDoubleClick: (path: string) => void;
    onItemTouchStart?: (record: { path: string; name: string }, index: number) => void;
    onItemTouchEnd?: () => void;
    onItemTouchCancel?: () => void;
  }) => (
    <div>
      {dataSource.map((item, index) => (
        <button
          key={item.path}
          type="button"
          onClick={(event) => onItemClick(event, item, index)}
          onDoubleClick={() => onItemDoubleClick(item.path)}
          onTouchStart={() => onItemTouchStart?.(item, index)}
          onTouchEnd={() => onItemTouchEnd?.()}
          onTouchCancel={() => onItemTouchCancel?.()}
        >
          {item.name}
        </button>
      ))}
    </div>
  ),
}));

vi.mock('./FolderContent/FolderContentGrid', () => ({
  default: ({ dataSource, onItemClick, onItemDoubleClick, onItemTouchStart, onItemTouchEnd, onItemTouchCancel }: {
    dataSource: Array<{ path: string; name: string }>;
    onItemClick: (event: ReactMouseEvent<HTMLElement>, record: { path: string; name: string }, index: number) => void;
    onItemDoubleClick: (path: string) => void;
    onItemTouchStart?: (record: { path: string; name: string }, index: number) => void;
    onItemTouchEnd?: () => void;
    onItemTouchCancel?: () => void;
  }) => (
    <div>
      {dataSource.map((item, index) => (
        <button
          key={item.path}
          type="button"
          onClick={(event) => onItemClick(event, item, index)}
          onDoubleClick={() => onItemDoubleClick(item.path)}
          onTouchStart={() => onItemTouchStart?.(item, index)}
          onTouchEnd={() => onItemTouchEnd?.()}
          onTouchCancel={() => onItemTouchCancel?.()}
        >
          {item.name}
        </button>
      ))}
    </div>
  ),
}));

vi.mock('./DestinationPickerModal', () => ({
  default: () => null,
}));

vi.mock('./FolderContent/RenameModal', () => ({
  default: () => null,
}));

vi.mock('./FolderContent/CreateFolderModal', () => ({
  default: () => null,
}));

vi.mock('./FolderContent/TrashModal', () => ({
  default: () => null,
}));

vi.mock('./FolderContent/UploadOverlay', () => ({
  default: () => null,
}));

vi.mock('./FolderContent/BoxSelectionOverlay', () => ({
  default: () => null,
}));

vi.mock('@/components/common/BottomSheet', () => ({
  default: () => null,
}));

function setTouchSupport(enabled: boolean) {
  Object.defineProperty(navigator, 'maxTouchPoints', {
    configurable: true,
    value: enabled ? 2 : 0,
  });
}

function setMobileLayout(enabled: boolean) {
  h.breakpointState.lg = !enabled;
}

function dispatchTouchEvent(
  element: Element,
  type: 'touchstart' | 'touchend' | 'touchcancel',
  touches: Array<{ clientX: number; clientY: number }>
) {
  const event = new Event(type, { bubbles: true, cancelable: true }) as Event & {
    touches: Array<{ clientX: number; clientY: number }>;
    changedTouches: Array<{ clientX: number; clientY: number }>;
  };
  Object.defineProperty(event, 'touches', { configurable: true, value: touches });
  Object.defineProperty(event, 'changedTouches', { configurable: true, value: touches });
  element.dispatchEvent(event);
}

describe('FolderContent activation behavior', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    h.transferItems.length = 0;
    useTransferCenterStore.getState().reset();
    h.storeState.content = h.contentItems;
    h.storeState.error = null;
    h.storeState.isLoading = false;
    h.locationPathState.pathname = '/browse';
    h.searchModeState.query = '';
    h.searchModeState.errorMessage = null;
    h.searchModeState.isSearching = false;
    h.searchModeState.resultCount = 0;
    h.searchModeState.currentLimit = 80;
    h.searchModeState.hasMore = false;
    h.searchModeState.hasEnoughQuery = false;
    h.searchModeState.canLoadMore = false;
    setMobileLayout(false);
    setTouchSupport(false);
  });

  it('opens folder path on desktop double click', async () => {
    const user = userEvent.setup();
    const { getByRole } = render(<FolderContent />);

    await user.dblClick(getByRole('button', { name: 'docs' }));

    expect(h.mockSetPath).toHaveBeenCalledWith('/docs', h.selectedSpace);
    expect(h.mockHandleBulkDownload).not.toHaveBeenCalledWith(['/docs']);
  });

  it('downloads file on desktop double click', async () => {
    const user = userEvent.setup();
    const { getByRole } = render(<FolderContent />);

    await user.dblClick(getByRole('button', { name: 'report.pdf' }));

    expect(h.mockHandleBulkDownload).toHaveBeenCalledWith(['/report.pdf']);
  });

  it('opens folder path on mobile single tap', async () => {
    setMobileLayout(true);
    const user = userEvent.setup();
    const { getByRole } = render(<FolderContent />);

    await user.click(getByRole('button', { name: 'docs' }));

    expect(h.mockSetPath).toHaveBeenCalledWith('/docs', h.selectedSpace);
  });

  it('selects file on mobile single tap without downloading', async () => {
    setMobileLayout(true);
    const user = userEvent.setup();
    const { getByRole } = render(<FolderContent />);

    await user.click(getByRole('button', { name: 'report.pdf' }));

    expect(h.mockSetSelection).toHaveBeenCalledWith(new Set(['/report.pdf']), 1);
    expect(h.mockHandleBulkDownload).not.toHaveBeenCalled();
  });

  it('does not auto-download on single click in touch-capable desktop layout', async () => {
    setTouchSupport(true);
    const user = userEvent.setup();
    const { getByRole } = render(<FolderContent />);

    await user.click(getByRole('button', { name: 'report.pdf' }));

    expect(h.mockHandleItemClick).toHaveBeenCalledTimes(1);
    expect(h.mockHandleBulkDownload).not.toHaveBeenCalled();
  });

  it('downloads file on touch double tap in touch-capable desktop layout', () => {
    setTouchSupport(true);
    const { getByRole } = render(<FolderContent />);
    const item = getByRole('button', { name: 'report.pdf' });

    dispatchTouchEvent(item, 'touchstart', [{ clientX: 20, clientY: 30 }]);
    dispatchTouchEvent(item, 'touchend', []);
    item.click();

    dispatchTouchEvent(item, 'touchstart', [{ clientX: 20, clientY: 30 }]);
    dispatchTouchEvent(item, 'touchend', []);
    item.click();

    expect(h.mockHandleBulkDownload).toHaveBeenCalledTimes(1);
    expect(h.mockHandleBulkDownload).toHaveBeenCalledWith(['/report.pdf']);
  });

  it('opens folder path on touch double tap in touch-capable desktop layout', () => {
    setTouchSupport(true);
    const { getByRole } = render(<FolderContent />);
    const item = getByRole('button', { name: 'docs' });

    dispatchTouchEvent(item, 'touchstart', [{ clientX: 18, clientY: 26 }]);
    dispatchTouchEvent(item, 'touchend', []);
    item.click();

    dispatchTouchEvent(item, 'touchstart', [{ clientX: 18, clientY: 26 }]);
    dispatchTouchEvent(item, 'touchend', []);
    item.click();

    expect(h.mockSetPath).toHaveBeenCalledTimes(1);
    expect(h.mockSetPath).toHaveBeenCalledWith('/docs', h.selectedSpace);
  });

  it('does not duplicate activation when native double click follows touch fallback', () => {
    setTouchSupport(true);
    const { getByRole } = render(<FolderContent />);
    const item = getByRole('button', { name: 'report.pdf' });

    dispatchTouchEvent(item, 'touchstart', [{ clientX: 20, clientY: 30 }]);
    dispatchTouchEvent(item, 'touchend', []);
    item.click();

    dispatchTouchEvent(item, 'touchstart', [{ clientX: 20, clientY: 30 }]);
    dispatchTouchEvent(item, 'touchend', []);
    item.click();

    item.dispatchEvent(new MouseEvent('dblclick', { bubbles: true }));

    expect(h.mockHandleBulkDownload).toHaveBeenCalledTimes(1);
  });

  it('keeps long-press selection precedence in mobile layout', () => {
    setMobileLayout(true);
    vi.useFakeTimers();
    try {
      const { getByRole } = render(<FolderContent />);
      const item = getByRole('button', { name: 'report.pdf' });

      dispatchTouchEvent(item, 'touchstart', [{ clientX: 20, clientY: 30 }]);
      vi.advanceTimersByTime(420);
      dispatchTouchEvent(item, 'touchend', []);
      item.click();

      expect(h.mockSetSelection).toHaveBeenCalledWith(new Set(['/report.pdf']));
      expect(h.mockHandleBulkDownload).not.toHaveBeenCalled();
    } finally {
      vi.useRealTimers();
    }
  });

  it('keeps search mode activation routed to search result handler', async () => {
    h.locationPathState.pathname = '/search';
    const user = userEvent.setup();
    const { getByRole } = render(<FolderContent />);

    await user.click(getByRole('button', { name: 'report.pdf' }));

    expect(h.mockOpenSearchResult).toHaveBeenCalledWith('/report.pdf');
    expect(h.mockHandleBulkDownload).not.toHaveBeenCalled();
    expect(h.mockSetPath).not.toHaveBeenCalled();
  });

  it('shows search summary and load-more action in search mode', async () => {
    h.locationPathState.pathname = '/search';
    h.searchModeState.query = 'report';
    h.searchModeState.hasEnoughQuery = true;
    h.searchModeState.resultCount = 2;
    h.searchModeState.currentLimit = 80;
    h.searchModeState.hasMore = true;
    h.searchModeState.canLoadMore = true;
    const user = userEvent.setup();

    const { getByText, getByRole } = render(<FolderContent />);

    expect(getByText('folderContent.searchSummary')).toBeTruthy();
    await user.click(getByRole('button', { name: 'folderContent.searchLoadMore' }));

    expect(h.mockLoadMoreSearchResults).toHaveBeenCalledTimes(1);
  });

  it('renders floating transfer center for active transfers', () => {
    useTransferCenterStore.getState().upsertTransfer({
      id: 'archive-1',
      kind: 'archive',
      name: 'docs.zip',
      status: 'running',
    });

    const { getByTestId, getByText } = render(<FolderContent />);

    expect(getByTestId('transfer-center-trigger')).toBeTruthy();
    expect(getByTestId('transfer-center-panel')).toBeTruthy();
    expect(getByText('docs.zip')).toBeTruthy();
  });

  it('renders aligned guidance for full browse permission failures', () => {
    const expectedMessage = 'browseApi.permissionDeniedReason directorySetup.validation.permissionDeniedHint';
    h.storeState.content = [];
    h.storeState.error = new Error(expectedMessage);

    const { getByText, getByRole } = render(<FolderContent />);

    expect(getByText(expectedMessage)).toBeTruthy();
    expect(getByRole('button', { name: 'folderContent.retry' })).toBeTruthy();
    expect(document.body.textContent).not.toContain('Permission denied');
  });

  it('renders aligned guidance for inline browse permission failures', () => {
    const expectedMessage = 'browseApi.permissionDeniedReason directorySetup.validation.permissionDeniedHint';
    h.storeState.error = new Error(expectedMessage);

    const { getByText, getByRole } = render(<FolderContent />);

    expect(getByText('folderContent.latestFolderLoadFailed')).toBeTruthy();
    expect(getByText(expectedMessage)).toBeTruthy();
    expect(getByRole('button', { name: 'folderContent.retryShort' })).toBeTruthy();
    expect(document.body.textContent).not.toContain('Permission denied');
  });
});
