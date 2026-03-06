import { render } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ButtonHTMLAttributes, ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import SettingsPage from './index';

const h = vi.hoisted(() => {
  const navigate = vi.fn();
  const authState = {
    permissions: [] as string[],
  };
  const breakpointState = {
    lg: true,
  };

  return {
    navigate,
    authState,
    breakpointState,
  };
});

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

vi.mock('react-router', () => ({
  useNavigate: () => h.navigate,
}));

vi.mock('@/features/auth/useAuth', () => ({
  useAuth: () => ({
    user: {
      permissions: h.authState.permissions,
    },
  }),
}));

function createSectionStub(testId: string) {
  return () => <div data-testid={testId}>{testId}</div>;
}

vi.mock('./sections/ProfileSettings', () => ({ default: createSectionStub('profile-section') }));
vi.mock('./sections/GeneralSettings', () => ({ default: createSectionStub('general-section') }));
vi.mock('./sections/AppearanceSettings', () => ({ default: createSectionStub('appearance-section') }));
vi.mock('./sections/ServerSettings', () => ({ default: createSectionStub('server-section') }));
vi.mock('./sections/SpaceSettings', () => ({ default: createSectionStub('spaces-section') }));
vi.mock('./sections/PermissionSettings', () => ({ default: createSectionStub('permissions-section') }));
vi.mock('./sections/AccountSettings', () => ({ default: createSectionStub('accounts-section') }));
vi.mock('./sections/AuditLogsSettings', () => ({ default: createSectionStub('auditLogs-section') }));
vi.mock('./sections/AboutSettings', () => ({ default: createSectionStub('about-section') }));

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

vi.mock('@/components/common/SidePanelShell', () => ({
  default: ({ title, children, leftAction }: { title: string; children: ReactNode; leftAction?: ReactNode }) => (
    <section>
      <h2>{title}</h2>
      {leftAction}
      {children}
    </section>
  ),
}));

vi.mock('@ant-design/icons', () => ({
  UserOutlined: () => null,
  AppstoreOutlined: () => null,
  BgColorsOutlined: () => null,
  InfoCircleOutlined: () => null,
  GlobalOutlined: () => null,
  ClusterOutlined: () => null,
  SafetyCertificateOutlined: () => null,
  TeamOutlined: () => null,
  AuditOutlined: () => null,
  MenuOutlined: () => null,
  CloseOutlined: () => null,
}));

vi.mock('antd', () => ({
  Button: ({
    children,
    onClick,
    ...props
  }: ButtonHTMLAttributes<HTMLButtonElement> & {
    icon?: ReactNode;
    size?: string;
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
  Layout: Object.assign(
    ({ children }: { children: ReactNode }) => <div>{children}</div>,
    {
      Header: ({ children }: { children: ReactNode; className?: string; style?: Record<string, unknown> }) => <header>{children}</header>,
      Content: ({ children }: { children: ReactNode; className?: string; style?: Record<string, unknown> }) => <main>{children}</main>,
      Sider: ({ children }: { children: ReactNode; className?: string; width?: number; style?: Record<string, unknown> }) => <aside>{children}</aside>,
    }
  ),
  Menu: ({
    items,
    onClick,
  }: {
    items: Array<{ key: string; label: ReactNode }>;
    selectedKeys?: string[];
    onClick?: (info: { key: string }) => void;
    mode?: string;
    className?: string;
  }) => (
    <nav>
      {items.map((item) => (
        <button key={item.key} type="button" onClick={() => onClick?.({ key: item.key })}>
          {item.label}
        </button>
      ))}
    </nav>
  ),
  theme: {
    useToken: () => ({
      token: {
        colorBgContainer: '#ffffff',
        colorBgLayout: '#f5f5f5',
        colorText: '#111111',
      },
    }),
  },
}));

describe('SettingsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    h.authState.permissions = [];
    h.breakpointState.lg = true;
  });

  it('hides permission-gated menu items when the user lacks access', () => {
    const view = render(<SettingsPage />);

    expect(view.getByText('settingsPage.sections.profile')).toBeTruthy();
    expect(view.getByText('settingsPage.sections.general')).toBeTruthy();
    expect(view.getByText('settingsPage.sections.appearance')).toBeTruthy();
    expect(view.getByText('settingsPage.sections.about')).toBeTruthy();
    expect(view.queryByText('settingsPage.sections.server')).toBeNull();
    expect(view.queryByText('settingsPage.sections.spaces')).toBeNull();
    expect(view.queryByText('settingsPage.sections.permissions')).toBeNull();
    expect(view.queryByText('settingsPage.sections.accounts')).toBeNull();
    expect(view.queryByText('settingsPage.sections.auditLogs')).toBeNull();
    expect(view.getByTestId('profile-section')).toBeTruthy();
  });

  it('renders available sections when the corresponding menu item is selected', async () => {
    h.authState.permissions = ['server.config.read', 'space.read', 'account.read'];
    const user = userEvent.setup();
    const view = render(<SettingsPage />);

    expect(view.getByText('settingsPage.sections.server')).toBeTruthy();
    expect(view.getByText('settingsPage.sections.spaces')).toBeTruthy();
    expect(view.getByText('settingsPage.sections.permissions')).toBeTruthy();
    expect(view.getByText('settingsPage.sections.accounts')).toBeTruthy();
    expect(view.getByText('settingsPage.sections.auditLogs')).toBeTruthy();

    await user.click(view.getByText('settingsPage.sections.server'));
    expect(view.getByTestId('server-section')).toBeTruthy();

    await user.click(view.getByText('settingsPage.sections.spaces'));
    expect(view.getByTestId('spaces-section')).toBeTruthy();

    await user.click(view.getByText('settingsPage.sections.auditLogs'));
    expect(view.getByTestId('auditLogs-section')).toBeTruthy();
  });
});
