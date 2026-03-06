import { render } from '@testing-library/react';
import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import ServerSettings from './ServerSettings';

const mockUseAuth = vi.fn();

const h = vi.hoisted(() => {
  const getConfig = vi.fn<() => Promise<{ server: {
    port: string;
    webdavEnabled: boolean;
    ftpEnabled: boolean;
    ftpPort: number;
    sftpEnabled: boolean;
    sftpPort: number;
  };
  auditLogRetentionDays: number;
  }>>();
  const updateConfig = vi.fn<(config: {
    server: {
      port: string;
      webdavEnabled: boolean;
      ftpEnabled: boolean;
      ftpPort: number;
      sftpEnabled: boolean;
      sftpPort: number;
    };
    auditLogRetentionDays: number;
  }) => Promise<void>>();
  const restartServer = vi.fn<() => Promise<string>>();
  const waitForReconnect = vi.fn<() => Promise<boolean>>();
  const messageApi = {
    success: vi.fn(),
    error: vi.fn(),
    loading: vi.fn(),
  };
  const modalApi = {
    confirm: vi.fn(),
  };

  return {
    getConfig,
    updateConfig,
    restartServer,
    waitForReconnect,
    messageApi,
    modalApi,
  };
});

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, options?: Record<string, unknown>) => {
      if (options?.error) {
        return `${key}:${String(options.error)}`;
      }
      if (options?.port) {
        return `${key}:${String(options.port)}`;
      }
      return key;
    },
  }),
}));

vi.mock('@/api/config', () => ({
  getConfig: h.getConfig,
  updateConfig: h.updateConfig,
  restartServer: h.restartServer,
  waitForReconnect: h.waitForReconnect,
}));

vi.mock('@/features/auth/useAuth', () => ({
  useAuth: () => mockUseAuth(),
}));

vi.mock('../components/SettingSectionHeader', () => ({
  default: ({ title, subtitle }: { title: string; subtitle: string }) => (
    <div>
      <h2>{title}</h2>
      <p>{subtitle}</p>
    </div>
  ),
}));

vi.mock('../components/SettingRow', () => ({
  default: ({ left, right }: { left: ReactNode; right: ReactNode }) => (
    <div>
      {left}
      {right}
    </div>
  ),
}));

vi.mock('@ant-design/icons', () => ({
  ReloadOutlined: () => null,
  SaveOutlined: () => null,
}));

vi.mock('antd', () => ({
  Alert: ({ title }: { title: ReactNode; type?: string; showIcon?: boolean; className?: string }) => <div>{title}</div>,
  App: {
    useApp: () => ({
      message: h.messageApi,
      modal: h.modalApi,
    }),
  },
  Button: ({
    children,
    onClick,
    disabled,
    loading,
  }: ButtonHTMLAttributes<HTMLButtonElement> & {
    icon?: ReactNode;
    size?: string;
    type?: 'primary' | 'default';
    loading?: boolean;
  }) => (
    <button type="button" onClick={onClick} disabled={disabled || loading}>
      {children}
    </button>
  ),
  Card: ({ children, title }: { children: ReactNode; title?: ReactNode; size?: string }) => (
    <section>
      {title}
      {children}
    </section>
  ),
  Divider: () => <hr />,
  InputNumber: ({
    value,
    onChange,
    disabled,
  }: InputHTMLAttributes<HTMLInputElement> & {
    value?: number | null;
    onChange?: (value: number | null) => void;
    size?: string;
    min?: number;
    max?: number;
    className?: string;
    disabled?: boolean;
  }) => (
    <input
      type="number"
      value={value ?? ''}
      disabled={disabled}
      onChange={(event) => onChange?.(event.target.value === '' ? null : Number(event.target.value))}
    />
  ),
  Space: ({ children }: { children: ReactNode; vertical?: boolean; size?: string; className?: string }) => <div>{children}</div>,
  Switch: ({
    checked,
    onChange,
    disabled,
  }: {
    checked?: boolean;
    onChange?: (checked: boolean) => void;
    disabled?: boolean;
  }) => <input type="checkbox" checked={checked} disabled={disabled} onChange={(event) => onChange?.(event.target.checked)} />,
  Typography: {
    Text: ({ children }: { children: ReactNode; strong?: boolean }) => <span>{children}</span>,
  },
}));

const baseConfig = {
  server: {
    port: '3000',
    webdavEnabled: true,
    ftpEnabled: false,
    ftpPort: 2121,
    sftpEnabled: false,
    sftpPort: 2022,
  },
  auditLogRetentionDays: 0,
};

function changeInputValue(input: HTMLInputElement, value: string) {
  const descriptor = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value');
  descriptor?.set?.call(input, value);
  input.dispatchEvent(new Event('input', { bubbles: true }));
}

describe('ServerSettings', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseAuth.mockReturnValue({
      user: {
        permissions: ['server.config.read', 'server.config.write'],
      },
    });
    h.getConfig.mockResolvedValue(baseConfig);
    h.updateConfig.mockResolvedValue();
    h.restartServer.mockResolvedValue('3000');
    h.waitForReconnect.mockResolvedValue(true);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('surfaces an error when the server config fails to load', async () => {
    h.getConfig.mockRejectedValue(new Error('load failed'));

    render(<ServerSettings />);

    await vi.waitFor(() => {
      expect(h.messageApi.error).toHaveBeenCalledWith('serverSettings.loadFailed');
    });
  });

  it('shows validation feedback and blocks actions for an invalid config', async () => {
    h.getConfig.mockResolvedValue({
      ...baseConfig,
      server: {
        ...baseConfig.server,
        port: '0',
      },
    });

    const view = render(<ServerSettings />);

    expect(await view.findByText('serverSettings.validationWebPort')).toBeTruthy();
    expect(view.getByRole('button', { name: 'serverSettings.save' }).hasAttribute('disabled')).toBe(true);
    expect(view.getByRole('button', { name: 'serverSettings.restartButton' }).hasAttribute('disabled')).toBe(true);
  });

  it('shows validation feedback for a negative audit retention day setting', async () => {
    h.getConfig.mockResolvedValue({
      ...baseConfig,
      auditLogRetentionDays: -1,
    });

    const view = render(<ServerSettings />);

    expect(await view.findByText('serverSettings.validationAuditRetentionDays')).toBeTruthy();
    expect(view.getByRole('button', { name: 'serverSettings.save' }).hasAttribute('disabled')).toBe(true);
  });

  it('saves edited config and shows success feedback', async () => {
    const view = render(<ServerSettings />);

    const portInput = await view.findByDisplayValue('3000');
    changeInputValue(portInput as HTMLInputElement, '3300');
    (view.getByRole('button', { name: 'serverSettings.save' }) as HTMLButtonElement).click();

    await vi.waitFor(() => {
      expect(h.updateConfig).toHaveBeenCalledWith({
        ...baseConfig,
        server: {
          ...baseConfig.server,
          port: '3300',
        },
      });
    });
    expect(h.messageApi.success).toHaveBeenCalledWith('serverSettings.saveSuccess');
  });

  it('runs the restart confirmation flow and surfaces reconnect failure', async () => {
    h.waitForReconnect.mockResolvedValue(false);
    const view = render(<ServerSettings />);
    const restartButton = await view.findByRole('button', { name: 'serverSettings.restartButton' });
    const setTimeoutSpy = vi.spyOn(globalThis, 'setTimeout').mockImplementation((handler: TimerHandler) => {
      if (typeof handler === 'function') {
        handler();
      }
      return 0 as ReturnType<typeof setTimeout>;
    });

    (restartButton as HTMLButtonElement).click();

    expect(h.modalApi.confirm).toHaveBeenCalledTimes(1);
    const confirmConfig = h.modalApi.confirm.mock.calls[0]?.[0] as { onOk?: () => Promise<void> };
    await confirmConfig.onOk?.();

    expect(h.restartServer).toHaveBeenCalledTimes(1);
    expect(h.waitForReconnect).toHaveBeenCalledTimes(1);
    expect(h.messageApi.loading).toHaveBeenCalledWith({
      content: 'serverSettings.restarting',
      key: 'restart',
      duration: 0,
    });
    expect(h.messageApi.error).toHaveBeenCalledWith({
      content: 'serverSettings.restartFailedOrTimeout',
      key: 'restart',
    });
    setTimeoutSpy.mockRestore();
  });

  it('renders server settings as read-only without server.config.write', async () => {
    mockUseAuth.mockReturnValue({
      user: {
        permissions: ['server.config.read'],
      },
    });

    const view = render(<ServerSettings />);

    expect(await view.findByText('serverSettings.readOnlyHint')).toBeTruthy();
    expect(view.getByRole('button', { name: 'serverSettings.save' }).hasAttribute('disabled')).toBe(true);
    expect(view.getByRole('button', { name: 'serverSettings.restartButton' }).hasAttribute('disabled')).toBe(true);
    expect((view.getByDisplayValue('3000') as HTMLInputElement).disabled).toBe(true);
  });
});
