import { render } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ChangeEvent, ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import AccountSettings from './AccountSettings';
import {
  listAccountPermissions,
  listAccounts,
  listSpaces,
  updateAccountPermissions,
} from '@/api/accounts';
import { listRoles } from '@/api/roles';

const h = vi.hoisted(() => {
  const messageApi = {
    success: vi.fn(),
    error: vi.fn(),
  };
  const t = (key: string, options?: Record<string, unknown>) => {
    if (options?.username) {
      return `${key}:${String(options.username)}`;
    }
    return key;
  };

  return {
    messageApi,
    t,
  };
});

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: h.t,
  }),
}));

vi.mock('@ant-design/icons', () => ({
  DeleteOutlined: () => null,
  EditOutlined: () => null,
  PlusOutlined: () => null,
  ReloadOutlined: () => null,
  UserOutlined: () => null,
}));

vi.mock('antd', () => {
  const App = Object.assign(
    ({ children }: { children: ReactNode }) => <div>{children}</div>,
    {
      useApp: () => ({
        message: h.messageApi,
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
    Card: ({ children }: { children: ReactNode }) => <section>{children}</section>,
    Grid: {
      useBreakpoint: () => ({ lg: true }),
    },
    Input: ({
      value,
      onChange,
    }: {
      value?: string;
      onChange?: (event: ChangeEvent<HTMLInputElement>) => void;
    }) => <input value={value} onChange={onChange} />,
    Modal: ({
      open,
      title,
      children,
      onOk,
      okText,
      onCancel,
      cancelText,
    }: {
      open: boolean;
      title?: ReactNode;
      children: ReactNode;
      onOk?: () => void;
      okText?: ReactNode;
      onCancel?: () => void;
      cancelText?: ReactNode;
    }) => open ? (
      <div>
        {title ? <h2>{title}</h2> : null}
        {children}
        <button type="button" onClick={onOk}>{okText ?? 'ok'}</button>
        <button type="button" onClick={onCancel}>{cancelText ?? 'cancel'}</button>
      </div>
    ) : null,
    Popconfirm: ({ children }: { children: ReactNode }) => <>{children}</>,
    Select: ({
      value,
      onChange,
      options,
    }: {
      value?: string | number;
      onChange?: (value: string | number) => void;
      options?: Array<{ value: string | number; label: ReactNode }>;
    }) => (
      <select
        value={value ?? ''}
        onChange={(event) => onChange?.(event.target.value)}
      >
        {options?.map((item) => (
          <option key={String(item.value)} value={String(item.value)}>
            {item.label}
          </option>
        ))}
      </select>
    ),
    Space: ({ children }: { children: ReactNode }) => <div>{children}</div>,
    Table: ({
      columns,
      dataSource,
      rowKey,
    }: {
      columns: Array<{ key?: string; dataIndex?: string; render?: (_: unknown, item: Record<string, unknown>) => ReactNode }>;
      dataSource: Array<Record<string, unknown>>;
      rowKey: string | ((item: Record<string, unknown>) => string | number);
    }) => (
      <table>
        <tbody>
          {dataSource.map((item) => (
            <tr key={typeof rowKey === 'function' ? rowKey(item) : String(item[rowKey])}>
              {columns.map((column, index) => (
                <td key={column.key ?? index}>
                  {column.render?.(column.dataIndex ? item[column.dataIndex] : undefined, item)
                    ?? (column.dataIndex ? item[column.dataIndex] as ReactNode : null)}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    ),
    Tag: ({ children }: { children: ReactNode }) => <span>{children}</span>,
    Typography: {
      Text: ({ children }: { children: ReactNode }) => <span>{children}</span>,
    },
  };
});

vi.mock('@/api/accounts', () => ({
  listAccounts: vi.fn(),
  createAccount: vi.fn(),
  deleteAccount: vi.fn(),
  updateAccount: vi.fn(),
  listAccountPermissions: vi.fn(),
  updateAccountPermissions: vi.fn(),
  listSpaces: vi.fn(),
}));

vi.mock('@/api/roles', () => ({
  listRoles: vi.fn(),
}));

vi.mock('../components/SettingSectionHeader', () => ({
  default: ({ title, subtitle }: { title: ReactNode; subtitle: ReactNode }) => (
    <header>
      <h1>{title}</h1>
      <p>{subtitle}</p>
    </header>
  ),
}));

const accountUser = {
  id: 7,
  username: 'member',
  nickname: 'Member',
  role: 'user',
  createdAt: '2026-03-07T00:00:00Z',
  updatedAt: '2026-03-07T00:00:00Z',
};

const spaceItem = {
  id: 3,
  space_name: 'Alpha',
};

describe('AccountSettings', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(listAccounts).mockResolvedValue([accountUser]);
    vi.mocked(listRoles).mockResolvedValue([
      { name: 'admin', description: 'admin', isSystem: true, permissions: [] },
      { name: 'user', description: 'user', isSystem: true, permissions: [] },
    ]);
    vi.mocked(listSpaces).mockResolvedValue([spaceItem]);
    vi.mocked(listAccountPermissions).mockResolvedValue([
      { userId: accountUser.id, spaceId: spaceItem.id, permission: 'manage' },
    ]);
    vi.mocked(updateAccountPermissions).mockResolvedValue();
  });

  it('preserves manage permission when account space permissions are loaded and saved', async () => {
    const user = userEvent.setup();
    const view = render(<AccountSettings />);

    await view.findByText('member');
    await user.click(view.getByRole('button', { name: 'accountSettings.spacePermissionsButton' }));

    const select = await view.findByRole('combobox');
    expect((select as HTMLSelectElement).value).toBe('manage');

    await user.click(view.getByRole('button', { name: 'accountSettings.save' }));

    expect(updateAccountPermissions).toHaveBeenCalledWith(accountUser.id, [
      { userId: accountUser.id, spaceId: spaceItem.id, permission: 'manage' },
    ]);
    expect(h.messageApi.success).toHaveBeenCalledWith('accountSettings.saveSpacePermissionsSuccess');
  });
});
