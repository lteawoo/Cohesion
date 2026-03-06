import { App } from 'antd';
import { render } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import AuditLogsSettings from './AuditLogsSettings';
import { cleanupAuditLogs, exportAuditLogsCsv, getAuditLog, listAuditLogs } from '@/api/audit';
import type { AuditLogItem, AuditLogListResponse } from '@/api/audit';

const h = vi.hoisted(() => ({
  permissions: ['account.read', 'account.write'] as string[],
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

vi.mock('@/features/auth/useAuth', () => ({
  useAuth: () => ({
    user: {
      permissions: h.permissions,
    },
  }),
}));

vi.mock('@/api/audit', () => ({
  listAuditLogs: vi.fn(),
  getAuditLog: vi.fn(),
  exportAuditLogsCsv: vi.fn(),
  cleanupAuditLogs: vi.fn(),
}));

function renderSection() {
  return render(
    <App>
      <AuditLogsSettings />
    </App>
  );
}

function buildLog(overrides: Partial<AuditLogItem> = {}): AuditLogItem {
  return {
    id: 1,
    occurredAt: '2026-03-03T10:00:00.000Z',
    actor: 'admin',
    action: 'file.delete',
    result: 'failure',
    target: 'docs/report.txt',
    requestId: 'req_1',
    spaceId: 3,
    metadata: {},
    ...overrides,
  };
}

function buildListResponse(items: AuditLogItem[], overrides: Partial<AuditLogListResponse> = {}): AuditLogListResponse {
  return {
    items,
    page: 1,
    pageSize: 20,
    total: items.length,
    retentionDays: 30,
    ...overrides,
  };
}

describe('AuditLogsSettings', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    h.permissions = ['account.read', 'account.write'];
    vi.mocked(listAuditLogs).mockResolvedValue(buildListResponse([buildLog()]));
    vi.mocked(getAuditLog).mockResolvedValue(buildLog());
    vi.mocked(exportAuditLogsCsv).mockResolvedValue({
      blob: new Blob(['csv'], { type: 'text/csv' }),
      filename: 'audit-logs.csv',
    });
    vi.mocked(cleanupAuditLogs).mockResolvedValue({
      deletedCount: 1,
      retentionDays: 30,
      cutoff: '2026-02-01T00:00:00Z',
    });
    Object.defineProperty(window.URL, 'createObjectURL', {
      configurable: true,
      value: vi.fn(() => 'blob:mock'),
    });
    Object.defineProperty(window.URL, 'revokeObjectURL', {
      configurable: true,
      value: vi.fn(),
    });
  });

  it('applies filters and sends expected query params', async () => {
    const user = userEvent.setup();
    const view = renderSection();

    await vi.waitFor(() => {
      expect(listAuditLogs).toHaveBeenCalled();
    });
    vi.mocked(listAuditLogs).mockClear();

    await user.type(view.getByPlaceholderText('auditSettings.userPlaceholder'), 'alice');
    await user.type(view.getByPlaceholderText('auditSettings.actionPlaceholder'), 'file.delete');
    await user.type(view.getByPlaceholderText('auditSettings.spacePlaceholder'), '7');

    await user.click(view.getByRole('button', { name: 'auditSettings.search' }));

    await vi.waitFor(() => {
      expect(listAuditLogs).toHaveBeenCalled();
    });

    const latestCall = vi.mocked(listAuditLogs).mock.calls.at(-1);
    expect(latestCall?.[0]).toEqual({
      page: 1,
      pageSize: 20,
      user: 'alice',
      action: 'file.delete',
      spaceId: 7,
    });
  });

  it('loads and renders selected row detail metadata', async () => {
    const rowItem = buildLog({ id: 11, requestId: 'req_11' });
    const detailItem = buildLog({
      id: 11,
      requestId: 'req_11',
      metadata: { reason: 'validation_failed', code: 'ROLE_INVALID' },
    });
    vi.mocked(listAuditLogs).mockResolvedValue(buildListResponse([rowItem]));
    vi.mocked(getAuditLog).mockResolvedValue(detailItem);

    const view = renderSection();

    const rowTarget = await view.findByText('docs/report.txt');
    rowTarget.dispatchEvent(new MouseEvent('click', { bubbles: true }));

    await vi.waitFor(() => {
      expect(getAuditLog).toHaveBeenCalledWith(11);
    });
    expect(await view.findByText(/validation_failed/)).toBeTruthy();
    expect(view.getByText(/ROLE_INVALID/)).toBeTruthy();
  });

  it('exports csv with the currently applied filters', async () => {
    const user = userEvent.setup();
    const view = renderSection();

    await vi.waitFor(() => {
      expect(listAuditLogs).toHaveBeenCalled();
    });
    vi.mocked(listAuditLogs).mockClear();

    await user.type(view.getByPlaceholderText('auditSettings.userPlaceholder'), 'alice');
    await user.type(view.getByPlaceholderText('auditSettings.actionPlaceholder'), 'file.delete');
    await user.click(view.getByRole('button', { name: 'auditSettings.search' }));

    await vi.waitFor(() => {
      expect(listAuditLogs).toHaveBeenCalled();
    });

    await user.click(view.getByRole('button', { name: 'auditSettings.exportAction' }));

    await vi.waitFor(() => {
      expect(exportAuditLogsCsv).toHaveBeenCalledWith({
        user: 'alice',
        action: 'file.delete',
      });
    });
  });

  it('hides cleanup action without account.write permission', async () => {
    h.permissions = ['account.read'];
    const view = renderSection();

    await vi.waitFor(() => {
      expect(listAuditLogs).toHaveBeenCalled();
    });

    expect(view.queryByRole('button', { name: 'auditSettings.cleanupAction' })).toBeNull();
  });

  it('blocks cleanup when retention policy is disabled', async () => {
    h.permissions = ['account.read', 'account.write'];
    vi.mocked(listAuditLogs).mockResolvedValue(buildListResponse([buildLog()], { retentionDays: 0 }));
    const user = userEvent.setup();
    const view = renderSection();

    await vi.waitFor(() => {
      expect(listAuditLogs).toHaveBeenCalled();
    });

    await user.click(view.getByRole('button', { name: 'auditSettings.cleanupAction' }));

    expect(cleanupAuditLogs).not.toHaveBeenCalled();
  });

  it('runs cleanup and refetches the list after confirmation', async () => {
    vi.mocked(listAuditLogs)
      .mockResolvedValueOnce(buildListResponse([buildLog()], { retentionDays: 30 }))
      .mockResolvedValueOnce(buildListResponse([], { retentionDays: 30 }));

    const user = userEvent.setup();
    const view = renderSection();

    await vi.waitFor(() => {
      expect(listAuditLogs).toHaveBeenCalled();
    });
    vi.mocked(listAuditLogs).mockClear();

    await user.click(view.getByRole('button', { name: 'auditSettings.cleanupAction' }));

    const confirmButtons = await view.findAllByRole('button', { name: 'auditSettings.cleanupAction' });
    await user.click(confirmButtons.at(-1)!);

    await vi.waitFor(() => {
      expect(cleanupAuditLogs).toHaveBeenCalledTimes(1);
      expect(listAuditLogs).toHaveBeenCalled();
    });

    expect(vi.mocked(listAuditLogs).mock.calls.at(-1)?.[0]).toMatchObject({
      page: 1,
      pageSize: 20,
    });
  });
});
