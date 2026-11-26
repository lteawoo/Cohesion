import { render } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { App } from 'antd';
import { MemoryRouter } from 'react-router';
import { afterEach, expect, it, vi } from 'vitest';
import '@/i18n';
import { apiFetch } from '@/api/client';
import { buildSingleItemMenu, type ContextMenuCallbacks } from '@/features/browse/constants';
import DownloadLinkManager from './DownloadLinkManager';
import SharedDownload from './SharedDownload';

vi.mock('@/api/client', () => ({ apiFetch: vi.fn() }));

const file = { name: 'report.pdf', path: 'team/report.pdf', size: 21, expires: 1900000000 };
const link = { ...file, id: 42, token: 'public-test-token' };
const api = vi.mocked(apiFetch);

afterEach(() => { api.mockReset(); vi.unstubAllGlobals(); });

it('offers sharing for individual files when the space grants the management action', () => {
  const callbacks: ContextMenuCallbacks = {
    onDownload: vi.fn(), onCopy: vi.fn(), onMove: vi.fn(), onRename: vi.fn(), onDelete: vi.fn(),
    onBulkDownload: vi.fn(), onBulkDelete: vi.fn(), onCreateFolder: vi.fn(),
  };
  const record = { ...file, isDir: false, modTime: '' };
  function choices(isDir: boolean, canManage: boolean) {
    return buildSingleItemMenu({ ...record, isDir }, { ...callbacks, onShare: canManage ? vi.fn() : undefined }, key => key)
      ?.some(item => !!item && 'key' in item && item.key === 'share');
  }
  expect(choices(false, true)).toBe(true);
  expect(choices(true, true)).toBe(false);
  expect(choices(false, false)).toBe(false);
});

it('creates a download link for the selected file', async () => {
  const user = userEvent.setup();
  api.mockResolvedValueOnce(Response.json([])).mockResolvedValueOnce(Response.json(link, { status: 201 }));
  const view = render(<App><DownloadLinkManager spaceId={7} path={file.path} onClose={vi.fn()} /></App>);
  const create = view.getByRole('button', { name: '다운로드 링크 만들기' });
  await vi.waitFor(() => expect((create as HTMLButtonElement).disabled).toBe(false));
  await user.click(create);
  const address = await view.findByLabelText('다운로드 링크 주소') as HTMLInputElement;
  expect(new URL(address.value).pathname).toBe('/share');
  expect(new URL(address.value).hash).toBe('#public-test-token');
  expect(api.mock.calls[0][0]).toContain('path=team%2Freport.pdf');
  expect(JSON.parse(String(api.mock.calls[1][1]?.body))).toEqual({ path: file.path, days: 7 });
}, 10_000);

it('revokes an existing link from the space-wide list', async () => {
  const user = userEvent.setup();
  api.mockResolvedValueOnce(Response.json([link])).mockResolvedValueOnce(new Response(null, { status: 204 }));
  const all = render(<App><DownloadLinkManager spaceId={7} onClose={vi.fn()} /></App>);
  await user.click(await all.findByRole('button', { name: '링크 닫기' }));
  const confirm = (all.getAllByRole('button', { name: '링크 닫기' }) as HTMLElement[]).find(button => button.closest('.ant-popconfirm'));
  expect(confirm).toBeTruthy();
  await user.click(confirm!);
  expect(await all.findByText('이 스페이스에 활성 다운로드 링크가 없습니다.')).toBeTruthy();
  expect(api.mock.calls[1][0]).toContain('id=42');
  expect(api.mock.calls[1][1]?.method).toBe('DELETE');
});

it('explains how to recover when the server cannot store the file copy', async () => {
  const user = userEvent.setup();
  api.mockResolvedValueOnce(Response.json([])).mockResolvedValueOnce(new Response(null, { status: 507 }));
  const view = render(<App><DownloadLinkManager spaceId={7} path={file.path} onClose={vi.fn()} /></App>);
  const create = await view.findByRole('button', { name: '다운로드 링크 만들기' });
  await vi.waitFor(() => expect((create as HTMLButtonElement).disabled).toBe(false));
  await user.click(create);
  expect(await view.findByText(/사본을 저장할 공간이 부족합니다/)).toBeTruthy();
});

it('reads metadata without credentials and posts the token in the native download form', async () => {
  const fetch = vi.fn().mockResolvedValue(Response.json(file));
  vi.stubGlobal('fetch', fetch);
  const view = render(<MemoryRouter initialEntries={['/share#public-test-token']}><SharedDownload /></MemoryRouter>);
  expect(await view.findByRole('heading', { name: file.name })).toBeTruthy();
  expect(fetch).toHaveBeenCalledWith('/api/shared-file', expect.objectContaining({
    headers: { 'X-Download-Link': 'public-test-token' }, credentials: 'omit', referrerPolicy: 'no-referrer',
  }));
  const form = view.getByRole('button', { name: '다운로드' }).closest('form')!;
  expect(form.method).toBe('post');
  expect(new URL(form.action).pathname).toBe('/api/shared-file');
  expect(new URL(form.action).search).toBe('');
  expect(new FormData(form).get('token')).toBe('public-test-token');
});

it('keeps an unavailable link on the public page without a download action', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(null, { status: 404 })));
  const view = render(<MemoryRouter initialEntries={['/share#closed']}><SharedDownload /></MemoryRouter>);
  expect(await view.findByText('사용할 수 없는 다운로드 링크입니다.')).toBeTruthy();
  expect(view.queryByRole('button', { name: '다운로드' })).toBeNull();
});

it('shares the selected paths together and shows their total before creation', async () => {
  const user = userEvent.setup();
  const files = [{ path: file.path, size: 21 }, { path: 'team/notes.txt', size: 9 }];
  api.mockResolvedValueOnce(Response.json({ ...link, path: '', itemCount: 2 }));
  const view = render(<App><DownloadLinkManager spaceId={7} files={files} onClose={vi.fn()} /></App>);
  expect(view.getByText('선택한 파일 2개 · 30 B')).toBeTruthy();
  await user.click(view.getByRole('button', { name: '다운로드 링크 만들기' }));
  expect(await view.findByLabelText('다운로드 링크 주소')).toBeTruthy();
  expect(JSON.parse(String(api.mock.calls[0][1]?.body))).toEqual({ paths: files.map(item => item.path), days: 7 });
});

it('offers a native POST per item and a separate complete ZIP', async () => {
  const items = [{ id: 1, name: '보고서.pdf', size: 12 }, { id: 2, name: 'notes.txt', size: 9 }];
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(Response.json({ ...file, items })));
  const view = render(<MemoryRouter initialEntries={['/share#bundle-token']}><SharedDownload /></MemoryRouter>);
  expect(await view.findByRole('heading', { name: '공유 파일 2개' })).toBeTruthy();
  for (const item of items) {
    const form = view.getByRole('button', { name: item.name + ' 다운로드' }).closest('form')!;
    expect(form.method).toBe('post');
    expect(new FormData(form).get('item')).toBe(String(item.id));
    expect(new FormData(form).get('token')).toBe('bundle-token');
  }
  const zip = view.getByRole('button', { name: '전체 ZIP 다운로드' }).closest('form')!;
  expect(new FormData(zip).has('item')).toBe(false);
  expect(new FormData(zip).get('token')).toBe('bundle-token');
});
