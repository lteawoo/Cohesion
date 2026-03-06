import { act, render } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import MainLayout from './index';
import type { SearchFileResult } from '@/features/search/types';

const h = vi.hoisted(() => {
  const navigate = vi.fn();
  const searchFiles = vi.fn<(query: string, limit: number, options?: { signal?: AbortSignal }) => Promise<SearchFileResult[]>>();
  const fetchSpaces = vi.fn<() => Promise<void>>();
  const setPath = vi.fn();
  const reconcileSelectedSpace = vi.fn();
  const locationState = {
    pathname: '/',
  };
  const breakpointState = {
    lg: true,
  };
  const spaceState = {
    spaces: [] as Array<{ id: number; name: string }>,
    fetchSpaces,
  };
  const browseState = {
    selectedSpace: null as { id: number; name?: string } | null,
    setPath,
    reconcileSelectedSpace,
  };

  return {
    navigate,
    searchFiles,
    fetchSpaces,
    setPath,
    reconcileSelectedSpace,
    locationState,
    breakpointState,
    spaceState,
    browseState,
  };
});

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

vi.mock('react-router', () => ({
  Outlet: () => <div data-testid="outlet" />,
  useLocation: () => h.locationState,
  useNavigate: () => h.navigate,
}));

vi.mock('@/stores/spaceStore', () => ({
  useSpaceStore: (selector: (state: typeof h.spaceState) => unknown) => selector(h.spaceState),
}));

vi.mock('@/stores/browseStore', () => ({
  useBrowseStore: (selector: (state: typeof h.browseState) => unknown) => selector(h.browseState),
}));

vi.mock('@/features/search/api/searchApi', () => ({
  searchFiles: h.searchFiles,
}));

vi.mock('@/features/search/utils/highlightQueryMatch', () => ({
  highlightQueryMatch: (value: string) => value,
}));

vi.mock('./MainSider', () => ({
  default: ({ onPathSelect }: { onPathSelect?: (path: string) => void }) => (
    <button type="button" onClick={() => onPathSelect?.('/from-sider')}>
      sidenav
    </button>
  ),
}));

vi.mock('./ServerStatus', () => ({
  default: () => <div>server-status</div>,
}));

vi.mock('@/components/ContextMenu', () => ({
  default: () => <div data-testid="context-menu" />,
}));

vi.mock('@/components/common/HeaderBrand', () => ({
  default: ({ text, onClick }: { text: string; onClick?: () => void }) => (
    <button type="button" onClick={onClick}>
      {text}
    </button>
  ),
}));

vi.mock('@/components/common/HeaderGroup', () => ({
  default: ({ children }: { children: ReactNode; align?: string }) => <div>{children}</div>,
}));

vi.mock('@/features/browse/components/FileTypeIcon', () => ({
  FileTypeIcon: () => <span>file-icon</span>,
}));

vi.mock('@ant-design/icons', () => ({
  SettingOutlined: () => null,
  MenuOutlined: () => null,
  SearchOutlined: () => null,
  CloseOutlined: () => null,
  FolderFilled: () => <span>folder-icon</span>,
}));

vi.mock('antd', () => {
  const Input = ({
    value,
    onChange,
    onPressEnter,
    placeholder,
    disabled,
  }: InputHTMLAttributes<HTMLInputElement> & {
    allowClear?: boolean;
    autoFocus?: boolean;
    className?: string;
    prefix?: ReactNode;
    onPressEnter?: () => void;
  }) => (
    <input
      value={typeof value === 'string' ? value : ''}
      onChange={(event) => onChange?.(event)}
      onKeyDown={(event) => {
        if (event.key === 'Enter') {
          onPressEnter?.();
        }
      }}
      placeholder={placeholder}
      disabled={disabled}
    />
  );

  return {
    Button: ({
      children,
      onClick,
      ...props
    }: ButtonHTMLAttributes<HTMLButtonElement> & {
      icon?: ReactNode;
      type?: 'text' | 'default' | 'primary';
      title?: string;
    }) => (
      <button type="button" onClick={onClick} {...props}>
        {children}
      </button>
    ),
    Drawer: ({ children, open }: { children: ReactNode; open?: boolean }) => (open ? <div>{children}</div> : null),
    Grid: {
      useBreakpoint: () => h.breakpointState,
    },
    Input,
    Layout: Object.assign(
      ({ children }: { children: ReactNode }) => <div>{children}</div>,
      {
        Header: ({ children }: { children: ReactNode; className?: string; style?: Record<string, unknown> }) => <header>{children}</header>,
        Content: ({ children }: { children: ReactNode; className?: string }) => <main>{children}</main>,
      }
    ),
    Spin: ({ size }: { size?: string }) => <span>{size ?? 'spin'}</span>,
    theme: {
      useToken: () => ({
        token: {
          colorBgContainer: '#ffffff',
          colorText: '#111111',
        },
      }),
    },
  };
});

function changeInputValue(input: HTMLInputElement, value: string) {
  const descriptor = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value');
  descriptor?.set?.call(input, value);
  input.dispatchEvent(new Event('input', { bubbles: true }));
}

describe('MainLayout header search', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    h.locationState.pathname = '/';
    h.breakpointState.lg = true;
    h.spaceState.spaces = [];
    h.browseState.selectedSpace = null;
    h.fetchSpaces.mockResolvedValue();
    h.searchFiles.mockResolvedValue([]);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('disables header search when there are no connected spaces', () => {
    const view = render(<MainLayout />);

    expect(view.getByPlaceholderText('mainLayout.searchPlaceholder').hasAttribute('disabled')).toBe(true);
    expect(h.fetchSpaces).toHaveBeenCalledTimes(1);
  });

  it('debounces suggestions and renders returned search results', async () => {
    vi.useFakeTimers();
    h.spaceState.spaces = [{ id: 1, name: 'Alpha' }];
    h.searchFiles.mockResolvedValue([
      {
        spaceId: 1,
        spaceName: 'Alpha',
        name: 'alpha.txt',
        path: '/alpha.txt',
        parentPath: '/',
        isDir: false,
        size: 10,
        modTime: '2026-03-06T12:00:00Z',
      },
    ]);

    const view = render(<MainLayout />);
    const input = view.getByPlaceholderText('mainLayout.searchPlaceholder');

    changeInputValue(input as HTMLInputElement, 'al');
    await act(async () => {
      await vi.advanceTimersByTimeAsync(420);
      await Promise.resolve();
    });
    expect(h.searchFiles).toHaveBeenCalledWith('al', 8, expect.objectContaining({
      signal: expect.any(AbortSignal),
    }));
    expect(view.getByText('alpha.txt')).toBeTruthy();
    expect(view.getByText('Alpha')).toBeTruthy();
  });

  it('navigates to the full search page when the query is submitted', async () => {
    h.spaceState.spaces = [{ id: 1, name: 'Alpha' }];
    const user = userEvent.setup();
    const view = render(<MainLayout />);

    await user.type(view.getByPlaceholderText('mainLayout.searchPlaceholder'), 'al');
    await user.keyboard('{Enter}');

    expect(h.navigate).toHaveBeenCalledWith('/search?q=al');
  });

  it('selects a suggestion and routes back to the browse page', async () => {
    vi.useFakeTimers();
    h.spaceState.spaces = [{ id: 1, name: 'Alpha' }];
    h.searchFiles.mockResolvedValue([
      {
        spaceId: 1,
        spaceName: 'Alpha',
        name: 'readme.txt',
        path: '/docs/readme.txt',
        parentPath: '/docs',
        isDir: false,
        size: 10,
        modTime: '2026-03-06T12:00:00Z',
      },
    ]);

    const view = render(<MainLayout />);
    const input = view.getByPlaceholderText('mainLayout.searchPlaceholder');

    changeInputValue(input as HTMLInputElement, 're');
    await act(async () => {
      await vi.advanceTimersByTimeAsync(420);
      await Promise.resolve();
    });
    expect(h.searchFiles).toHaveBeenCalledWith('re', 8, expect.any(Object));
    (view.getByRole('button', { name: /readme\.txt/i }) as HTMLButtonElement).click();

    expect(h.setPath).toHaveBeenCalledWith('/docs', { id: 1, name: 'Alpha' });
    expect(h.navigate).toHaveBeenCalledWith('/', {
      state: {
        fromSearchQuery: 're',
      },
    });
  });
});
