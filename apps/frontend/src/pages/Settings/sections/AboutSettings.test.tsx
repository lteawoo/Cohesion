import { render } from '@testing-library/react';
import type { ButtonHTMLAttributes, ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import AboutSettings from './AboutSettings';

const h = vi.hoisted(() => {
  const messageApi = {
    info: vi.fn(),
    error: vi.fn(),
  };
  const useUpdateCheck = vi.fn();
  const useSystemVersion = vi.fn();
  const useSelfUpdate = vi.fn();

  return {
    messageApi,
    useUpdateCheck,
    useSystemVersion,
    useSelfUpdate,
  };
});

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

vi.mock('@/features/status/hooks/useUpdateCheck', () => ({
  useUpdateCheck: () => h.useUpdateCheck(),
}));

vi.mock('@/features/status/hooks/useSystemVersion', () => ({
  useSystemVersion: () => h.useSystemVersion(),
}));

vi.mock('@/features/status/hooks/useSelfUpdate', () => ({
  useSelfUpdate: () => h.useSelfUpdate(),
}));

vi.mock('../components/SettingSectionHeader', () => ({
  default: ({ title, subtitle }: { title: string; subtitle: string }) => (
    <div>
      <h2>{title}</h2>
      <p>{subtitle}</p>
    </div>
  ),
}));

vi.mock('antd', () => ({
  App: {
    useApp: () => ({
      message: h.messageApi,
    }),
  },
  Button: ({
    children,
    onClick,
    disabled,
    loading,
  }: ButtonHTMLAttributes<HTMLButtonElement> & { loading?: boolean }) => (
    <button type="button" onClick={onClick} disabled={disabled || loading}>
      {children}
    </button>
  ),
  Card: ({ children, title }: { children: ReactNode; title?: ReactNode }) => (
    <section>
      {title}
      {children}
    </section>
  ),
  Descriptions: ({ items }: { items: Array<{ key: string; label: ReactNode; children: ReactNode }> }) => (
    <dl>
      {items.map((item) => (
        <div key={item.key}>
          <dt>{item.label}</dt>
          <dd>{item.children}</dd>
        </div>
      ))}
    </dl>
  ),
  Space: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  Typography: {
    Text: ({ children }: { children: ReactNode; type?: string; code?: boolean }) => <span>{children}</span>,
  },
}));

describe('AboutSettings', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    h.useUpdateCheck.mockReturnValue({
      updateInfo: {
        currentVersion: 'v0.5.17',
        latestVersion: 'v0.5.18',
        updateAvailable: true,
        releaseUrl: 'https://github.com/lteawoo/Cohesion/releases/latest',
        checkedAt: '2026-03-07T00:00:00Z',
      },
    });
    h.useSystemVersion.mockReturnValue({
      versionInfo: {
        version: 'v0.5.17',
        commit: 'abc123',
        buildDate: '2026-03-07T00:00:00Z',
        os: 'linux',
        installChannel: 'direct',
      },
    });
    h.useSelfUpdate.mockReturnValue({
      status: null,
      isStarting: false,
      isUpdating: false,
      startUpdate: vi.fn().mockResolvedValue(undefined),
    });
  });

  it('shows Homebrew guidance for Homebrew installs', () => {
    h.useSystemVersion.mockReturnValue({
      versionInfo: {
        version: 'v0.5.17',
        commit: 'abc123',
        buildDate: '2026-03-07T00:00:00Z',
        os: 'linux',
        installChannel: 'homebrew',
      },
    });

    const view = render(<AboutSettings />);

    expect(view.getByText('aboutSettings.homebrewUpdateHint')).toBeTruthy();
    expect(view.getByText('aboutSettings.homebrewUpdateDetail')).toBeTruthy();
    expect(view.getByText('brew upgrade cohesion')).toBeTruthy();
    expect(view.queryByRole('button', { name: 'aboutSettings.updateNow' })).toBeNull();
  });

  it('shows direct-install guidance instead of the update button on macOS', () => {
    h.useSystemVersion.mockReturnValue({
      versionInfo: {
        version: 'v0.5.17',
        commit: 'abc123',
        buildDate: '2026-03-07T00:00:00Z',
        os: 'darwin',
        installChannel: 'direct',
      },
    });

    const view = render(<AboutSettings />);

    expect(view.getByText('aboutSettings.macOsDirectUpdateHint')).toBeTruthy();
    expect(view.getByText('aboutSettings.macOsDirectUpdateDetail')).toBeTruthy();
    expect(view.getByText('brew install lteawoo/cohesion/cohesion')).toBeTruthy();
    expect(view.queryByRole('button', { name: 'aboutSettings.updateNow' })).toBeNull();
  });

  it('shows systemd guidance for Linux service installs', () => {
    h.useSystemVersion.mockReturnValue({
      versionInfo: {
        version: 'v0.5.17',
        commit: 'abc123',
        buildDate: '2026-03-07T00:00:00Z',
        os: 'linux',
        installChannel: 'systemd',
      },
    });

    const view = render(<AboutSettings />);

    expect(view.getByText('aboutSettings.systemdUpdateHint')).toBeTruthy();
    expect(view.getByText('aboutSettings.systemdUpdateDetail')).toBeTruthy();
    expect(view.getByText('sudo ./install.sh --user "$(id -un)"')).toBeTruthy();
    expect(view.queryByRole('button', { name: 'aboutSettings.updateNow' })).toBeNull();
  });

  it('starts self-update on Linux', async () => {
    const startUpdate = vi.fn().mockResolvedValue(undefined);
    h.useSelfUpdate.mockReturnValue({
      status: null,
      isStarting: false,
      isUpdating: false,
      startUpdate,
    });

    const view = render(<AboutSettings />);

    view.getByRole('button', { name: 'aboutSettings.updateNow' }).click();

    await vi.waitFor(() => {
      expect(startUpdate).toHaveBeenCalledWith(false);
      expect(h.messageApi.info).toHaveBeenCalledWith('aboutSettings.updateStartHint');
    });
  });
});
