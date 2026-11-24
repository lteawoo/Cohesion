import { afterEach, expect, it, vi } from 'vitest';
import { sendSubmission } from './api';
import { matchesSubmission, readSubmission, rememberSubmission, startSubmission } from './pendingSubmission';

afterEach(() => { vi.unstubAllGlobals(); sessionStorage.clear(); });

it('resumes at the acknowledged offset and publishes with the original submission key', async () => {
  const fetch = vi.fn()
    .mockResolvedValueOnce(Response.json({ status: 'active', offset: 2 }))
    .mockResolvedValueOnce(Response.json({ status: 'active', offset: 5 }))
    .mockResolvedValueOnce(Response.json({ status: 'uploaded' }));
  vi.stubGlobal('fetch', fetch);
  const progress = vi.fn();
  await sendSubmission('link', 'original', new File(['hello'], 'note.txt'), progress);
  const calls: [string, RequestInit][] = fetch.mock.calls;
  const requests = calls.map(([, init]) => init);
  expect(requests.map(req => req.method)).toEqual(['POST', 'PATCH', 'PUT']);
  expect(requests.every(req => new Headers(req.headers).get('X-Submission-Key') === 'original' && req.credentials === 'omit')).toBe(true);
  expect(new Headers(requests[1].headers).get('Upload-Offset')).toBe('2');
  expect((requests[1].body as Blob).size).toBe(3);
  expect(progress.mock.calls).toEqual([[2], [5]]);
});

it('recovers an existing completion receipt without sending the file again', async () => {
  const fetch = vi.fn().mockResolvedValue(Response.json({ status: 'uploaded' }));
  vi.stubGlobal('fetch', fetch);
  await sendSubmission('link', 'original', new File(['done'], 'done.txt'), vi.fn());
  expect(fetch).toHaveBeenCalledTimes(1);
});

it('stores a per-link key and checks the original selection before resuming', () => {
  const file = new File(['abc'], 'data', { lastModified: 10 });
  const saved = startSubmission(file);
  rememberSubmission('first', saved);
  expect(readSubmission('first')).toEqual(saved);
  expect(readSubmission('second')).toBeNull();
  expect(matchesSubmission(file, saved)).toBe(true);
  expect(matchesSubmission(new File(['abc'], 'data', { lastModified: 11 }), saved)).toBe(false);
  rememberSubmission('first', null);
  expect(readSubmission('first')).toBeNull();
});
