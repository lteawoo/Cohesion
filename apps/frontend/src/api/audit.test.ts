import { afterEach, describe, expect, it, vi } from 'vitest';
import { getAuditLog, listAuditLogs } from './audit';

describe('audit api', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('builds list query including pagination and filters', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({
      items: [],
      page: 2,
      pageSize: 50,
      total: 0,
    }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }));

    await listAuditLogs({
      page: 2,
      pageSize: 50,
      from: '2026-03-03T00:00:00.000Z',
      to: '2026-03-03T01:00:00.000Z',
      user: 'alice',
      action: 'file.delete',
      spaceId: 7,
      result: 'denied',
    });

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0] ?? [];
    expect(typeof url).toBe('string');
    const requestURL = String(url);
    expect(requestURL).toContain('/api/audit/logs?');
    expect(requestURL).toContain('page=2');
    expect(requestURL).toContain('pageSize=50');
    expect(requestURL).toContain('user=alice');
    expect(requestURL).toContain('action=file.delete');
    expect(requestURL).toContain('spaceId=7');
    expect(requestURL).toContain('result=denied');
    expect(requestURL).toContain('from=2026-03-03T00%3A00%3A00.000Z');
    expect(requestURL).toContain('to=2026-03-03T01%3A00%3A00.000Z');
    expect(init).toMatchObject({ credentials: 'include' });
  });

  it('requests detail by id', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({
      id: 9,
      occurredAt: '2026-03-03T00:00:00.000Z',
      actor: 'admin',
      action: 'file.delete',
      result: 'success',
      target: 'docs/a.txt',
      requestId: 'req_9',
      metadata: {},
    }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }));

    await getAuditLog(9);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0]?.[0]).toBe('/api/audit/logs/9');
  });
});
