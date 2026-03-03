import { render } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ButtonHTMLAttributes, ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import MainSider from './MainSider';

const h = vi.hoisted(() => {
  const navigate = vi.fn();
  const deleteSpace = vi.fn();
  const permissionsState = {
    permissions: [] as string[],
  };
  const browseState = {
    selectedPath: '/',
    selectedSpace: null as { id: number } | null,
  };
  const spaceState = {
    deleteSpace,
  };

  return {
    navigate,
    deleteSpace,
    permissionsState,
    browseState,
    spaceState,
  };
});

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

vi.mock('react-router', () => ({
  useLocation: () => ({ pathname: '/' }),
  useNavigate: () => h.navigate,
}));

vi.mock('@/features/auth/useAuth', () => ({
  useAuth: () => ({
    user: { permissions: h.permissionsState.permissions },
  }),
}));

vi.mock('@/stores/spaceStore', () => ({
  useSpaceStore: (selector: (state: typeof h.spaceState) => unknown) => selector(h.spaceState),
}));

vi.mock('@/stores/browseStore', () => ({
  useBrowseStore: (selector: (state: typeof h.browseState) => unknown) => selector(h.browseState),
}));

vi.mock('@/features/space/components/DirectorySetupModal', () => ({
  default: ({ isOpen }: { isOpen: boolean }) => (
    <div data-testid="directory-setup-modal" data-open={isOpen ? 'true' : 'false'} />
  ),
}));

vi.mock('@/features/browse/components/FolderTree', () => ({
  default: () => <div data-testid="folder-tree" />,
}));

vi.mock('@/components/common/SidePanelShell', () => ({
  default: ({ title, leftAction, rightAction, children }: {
    title: string;
    leftAction?: ReactNode;
    rightAction?: ReactNode;
    children: ReactNode;
  }) => (
    <section>
      <h2>{title}</h2>
      {leftAction}
      {rightAction}
      {children}
    </section>
  ),
}));

vi.mock('@ant-design/icons', () => ({
  PlusOutlined: () => null,
  CloseOutlined: () => null,
}));

vi.mock('antd', () => ({
  Button: ({
    children,
    onClick,
    ...props
  }: ButtonHTMLAttributes<HTMLButtonElement>) => (
    <button type="button" onClick={onClick} {...props}>
      {children}
    </button>
  ),
  Layout: {
    Sider: ({ children }: { children: ReactNode }) => (
      <aside data-testid="sider">{children}</aside>
    ),
  },
  App: {
    useApp: () => ({
      message: {
        success: vi.fn(),
        error: vi.fn(),
      },
      modal: {
        confirm: vi.fn(),
      },
    }),
  },
  theme: {
    useToken: () => ({
      token: {
        colorBgContainer: '#ffffff',
      },
    }),
  },
  Tree: {
    DirectoryTree: ({ onSelect }: { onSelect?: (keys: string[]) => void }) => (
      <button type="button" data-testid="trash-entry" onClick={() => onSelect?.(['trash-action'])}>
        trash
      </button>
    ),
  },
}));

describe('MainSider space write policy', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    h.permissionsState.permissions = [];
    h.browseState.selectedPath = '/';
    h.browseState.selectedSpace = null;
  });

  it('shows add-space action and opens modal for users with space.write', async () => {
    h.permissionsState.permissions = ['space.write'];
    const user = userEvent.setup();

    const view = render(<MainSider />);

    const addButton = view.queryByRole('button', { name: 'mainSider.addSpace' });
    if (!addButton) {
      throw new Error('expected add-space button to be rendered');
    }

    expect(view.getByTestId('directory-setup-modal').getAttribute('data-open')).toBe('false');
    await user.click(addButton);
    expect(view.getByTestId('directory-setup-modal').getAttribute('data-open')).toBe('true');
  });

  it('hides add-space action and keeps modal closed without space.write', async () => {
    h.permissionsState.permissions = ['space.read'];
    const user = userEvent.setup();

    const view = render(<MainSider />);

    expect(view.queryByRole('button', { name: 'mainSider.addSpace' })).toBeNull();
    expect(view.getByTestId('directory-setup-modal').getAttribute('data-open')).toBe('false');

    await user.click(view.getByTestId('trash-entry'));
    expect(view.getByTestId('directory-setup-modal').getAttribute('data-open')).toBe('false');
  });
});
