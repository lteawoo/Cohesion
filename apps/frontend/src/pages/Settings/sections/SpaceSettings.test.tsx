import { render } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ChangeEvent, ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import SpaceSettings from './SpaceSettings';
import { apiFetch } from '@/api/client';

const h = vi.hoisted(() => {
  const permissionsState = {
    permissions: [] as string[],
  };
  const storeState = {
    fetchSpaces: vi.fn<() => Promise<void>>(),
    renameSpace: vi.fn<(id: number, name: string) => Promise<void>>(),
    deleteSpace: vi.fn<(id: number) => Promise<void>>(),
  };
  const messageApi = {
    success: vi.fn(),
    error: vi.fn(),
  };
  const modalApi = {
    confirm: vi.fn(),
  };
  const t = (key: string, options?: Record<string, unknown>) => {
    if (options?.spaceName) {
      return `${key}:${String(options.spaceName)}`;
    }
    return key;
  };

  return {
    permissionsState,
    storeState,
    messageApi,
    modalApi,
    t,
  };
});

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: h.t,
  }),
}));

vi.mock('@ant-design/icons', () => ({
  ReloadOutlined: () => null,
}));

vi.mock('antd', () => {
  const App = Object.assign(
    ({ children }: { children: ReactNode }) => <div>{children}</div>,
    {
      useApp: () => ({
        message: h.messageApi,
        modal: h.modalApi,
      }),
    }
  );

  return {
    App,
    Button: ({
      children,
      onClick,
      disabled,
      type,
    }: {
      children: ReactNode;
      onClick?: () => void;
      disabled?: boolean;
      type?: 'button' | 'submit' | 'reset';
    }) => (
      <button type={type ?? 'button'} onClick={onClick} disabled={disabled}>
        {children}
      </button>
    ),
    Card: ({ children, title, extra }: { children: ReactNode; title?: ReactNode; extra?: ReactNode }) => (
      <section>
        {title ? <h2>{title}</h2> : null}
        {extra}
        {children}
      </section>
    ),
    Input: ({
      value,
      onChange,
      onPressEnter,
      placeholder,
      disabled,
    }: {
      value?: string;
      onChange?: (event: ChangeEvent<HTMLInputElement>) => void;
      onPressEnter?: () => void;
      placeholder?: string;
      disabled?: boolean;
    }) => (
      <input
        value={value}
        onChange={onChange}
        onKeyDown={(event) => {
          if (event.key === 'Enter') {
            onPressEnter?.();
          }
        }}
        placeholder={placeholder}
        disabled={disabled}
      />
    ),
    InputNumber: ({
      value,
      onChange,
      placeholder,
      disabled,
    }: {
      value?: number | null;
      onChange?: (value: number | null) => void;
      placeholder?: string;
      disabled?: boolean;
    }) => (
      <input
        type="number"
        value={value ?? ''}
        onChange={(event) => onChange?.(event.target.value === '' ? null : Number(event.target.value))}
        placeholder={placeholder}
        disabled={disabled}
      />
    ),
    Progress: ({ percent }: { percent: number }) => <progress value={percent} max={100} />,
    Space: ({ children }: { children: ReactNode }) => <div>{children}</div>,
    Table: ({
      columns,
      dataSource,
      rowKey,
    }: {
      columns: Array<{ key?: string; render?: (_: unknown, item: Record<string, unknown>) => ReactNode }>;
      dataSource: Array<Record<string, unknown>> | Record<string, unknown>;
      rowKey: string | ((item: Record<string, unknown>) => string | number);
    }) => {
      const rows = Array.isArray(dataSource) ? dataSource : [];

      return (
        <table>
          <tbody>
            {rows.map((item) => (
              <tr key={typeof rowKey === 'function' ? rowKey(item) : String(item[rowKey])}>
                {columns.map((column, index) => (
                  <td key={column.key ?? index}>{column.render?.(undefined, item)}</td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      );
    },
    Tag: ({ children }: { children: ReactNode }) => <span>{children}</span>,
    Typography: {
      Text: ({ children }: { children: ReactNode }) => <span>{children}</span>,
      Title: ({ children }: { children: ReactNode }) => <h4>{children}</h4>,
    },
  };
});

vi.mock('@/api/client', () => ({
  apiFetch: vi.fn(),
}));

vi.mock('@/features/auth/useAuth', () => ({
  useAuth: () => ({
    user: { permissions: h.permissionsState.permissions },
  }),
}));

vi.mock('@/stores/spaceStore', () => ({
  useSpaceStore: (selector: (state: typeof h.storeState) => unknown) => selector(h.storeState),
}));

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: {
      'Content-Type': 'application/json',
    },
  });
}

function renderSection() {
  return render(
    <div>
      <SpaceSettings />
    </div>
  );
}

const usageItem = {
  spaceId: 1,
  spaceName: 'Alpha',
  usedBytes: 1024 * 1024,
  quotaBytes: 2 * 1024 * 1024,
  overQuota: false,
  scannedAt: '2026-03-06T12:00:00Z',
};

describe('SpaceSettings', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    h.permissionsState.permissions = ['space.read', 'space.write'];
    h.storeState.fetchSpaces.mockResolvedValue();
    h.storeState.renameSpace.mockResolvedValue();
    h.storeState.deleteSpace.mockResolvedValue();
  });

  it('renders an editable table for writable users', async () => {
    vi.mocked(apiFetch).mockImplementation(async () => jsonResponse([usageItem]));

    const view = renderSection();

    const nameInput = await view.findByPlaceholderText('spaceSettings.spaceNamePlaceholder') as HTMLInputElement;
    expect(nameInput.value).toBe('Alpha');
    expect(view.getByText('spaceSettings.sectionSubtitle')).toBeTruthy();
    expect(view.getByRole('button', { name: 'spaceSettings.refresh' })).toBeTruthy();
    expect(view.getByRole('button', { name: 'spaceSettings.saveAction' })).toBeTruthy();
    expect(view.getByRole('button', { name: 'spaceSettings.deleteSpaceAction' })).toBeTruthy();
  });

  it('saves changed name and quota from the same row', async () => {
    vi.mocked(apiFetch).mockImplementation(async (input: string | URL | Request) => {
      if (input === '/api/spaces/1/quota') {
        return jsonResponse({ id: 1, quotaBytes: 3 * 1024 * 1024 });
      }
      return jsonResponse([usageItem]);
    });

    const user = userEvent.setup();
    const view = renderSection();

    const nameInput = await view.findByPlaceholderText('spaceSettings.spaceNamePlaceholder');
    const quotaInput = view.getByDisplayValue('2') as HTMLInputElement;

    await user.clear(nameInput);
    await user.type(nameInput, 'Alpha Renamed');
    await user.clear(quotaInput);
    await user.type(quotaInput, '3');
    await user.click(view.getByRole('button', { name: 'spaceSettings.saveAction' }));

    await vi.waitFor(() => {
      expect(h.storeState.renameSpace).toHaveBeenCalledWith(1, 'Alpha Renamed');
      expect(apiFetch).toHaveBeenCalledWith('/api/spaces/1/quota', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ quotaBytes: 3 * 1024 * 1024 }),
      });
    });
    await vi.waitFor(() => {
      expect(h.storeState.fetchSpaces).toHaveBeenCalledTimes(2);
    });
    expect(h.messageApi.success).toHaveBeenCalledWith('spaceSettings.saveSpaceSuccess');
  });

  it('opens a confirmation modal and deletes the selected space from settings', async () => {
    vi.mocked(apiFetch).mockImplementation(async () => jsonResponse([usageItem]));

    const user = userEvent.setup();
    const view = renderSection();

    await view.findByRole('button', { name: 'spaceSettings.deleteSpaceAction' });
    await user.click(view.getByRole('button', { name: 'spaceSettings.deleteSpaceAction' }));

    expect(h.modalApi.confirm).toHaveBeenCalledTimes(1);
    const confirmConfig = h.modalApi.confirm.mock.calls[0]?.[0] as { content: string; onOk?: () => Promise<void> };
    expect(confirmConfig.content).toBe('spaceSettings.deleteSpaceDescription:Alpha');

    await confirmConfig.onOk?.();

    expect(h.storeState.deleteSpace).toHaveBeenCalledWith(1);
    expect(h.messageApi.success).toHaveBeenCalledWith('spaceSettings.deleteSpaceSuccess');
    expect(apiFetch).toHaveBeenCalledTimes(2);
  });

  it('renders read-only details for users without space.write', async () => {
    h.permissionsState.permissions = ['space.read'];
    vi.mocked(apiFetch).mockImplementation(async () => jsonResponse([usageItem]));

    const view = renderSection();

    expect(await view.findByText('Alpha')).toBeTruthy();
    expect(view.queryByPlaceholderText('spaceSettings.spaceNamePlaceholder')).toBeNull();
    expect(view.queryByRole('button', { name: 'spaceSettings.saveAction' })).toBeNull();
    expect(view.queryByRole('button', { name: 'spaceSettings.deleteSpaceAction' })).toBeNull();
  });
});
