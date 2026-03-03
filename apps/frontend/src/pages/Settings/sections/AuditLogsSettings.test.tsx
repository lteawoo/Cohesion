import { App } from 'antd';
import { render } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import AuditLogsSettings from './AuditLogsSettings';
import { getAuditLog, listAuditLogs } from '@/api/audit';
import type { AuditLogItem, AuditLogListResponse } from '@/api/audit';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

vi.mock('@/api/audit', () => ({
  listAuditLogs: vi.fn(),
  getAuditLog: vi.fn(),
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
    ...overrides,
  };
}

describe('AuditLogsSettings', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(listAuditLogs).mockResolvedValue(buildListResponse([buildLog()]));
    vi.mocked(getAuditLog).mockResolvedValue(buildLog());
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
});
