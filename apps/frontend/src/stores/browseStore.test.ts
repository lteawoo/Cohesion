import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { FileNode } from '@/features/browse/types';
import type { Space } from '@/features/space/types';
import { apiFetch } from '@/api/client';
import { useSpaceStore } from './spaceStore';
import { useBrowseStore } from './browseStore';

vi.mock('@/api/client', () => ({
  apiFetch: vi.fn(),
}));

const sampleContent: FileNode[] = [
  {
    name: 'report.txt',
    path: 'docs/report.txt',
    isDir: false,
    modTime: '2026-03-06T10:00:00.000Z',
    size: 128,
  },
];

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: {
      'Content-Type': 'application/json',
    },
  });
}

describe('useBrowseStore.reconcileSelectedSpace', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useBrowseStore.setState({
      selectedPath: 'docs',
      selectedSpace: { id: 1, space_name: 'Alpha' },
      content: sampleContent,
      isLoading: false,
      error: null,
      treeRefreshVersion: 0,
      treeInvalidationTargets: [],
      trashOpenRequest: null,
    });
  });

  it('replaces selectedSpace with the latest space object by id', () => {
    const refreshedSpaces: Space[] = [{ id: 1, space_name: 'Alpha Renamed' }];

    useBrowseStore.getState().reconcileSelectedSpace(refreshedSpaces);

    expect(useBrowseStore.getState().selectedSpace?.space_name).toBe('Alpha Renamed');
    expect(useBrowseStore.getState().selectedPath).toBe('docs');
    expect(useBrowseStore.getState().content).toEqual(sampleContent);
  });

  it('clears browse state when the selected space disappears from the shared list', () => {
    useBrowseStore.getState().reconcileSelectedSpace([]);

    expect(useBrowseStore.getState().selectedSpace).toBeUndefined();
    expect(useBrowseStore.getState().selectedPath).toBe('');
    expect(useBrowseStore.getState().content).toEqual([]);
  });
});

describe('useBrowseStore browse error normalization', () => {
  const expectedBrowsePermissionGuidance = '현재 서버가 이 위치를 읽을 수 없습니다. 해당 위치 접근 권한을 부여한 뒤 다시 시도하세요.';

  beforeEach(() => {
    vi.clearAllMocks();
    useSpaceStore.setState({
      spaces: [{ id: 7, space_name: 'Workspace' }],
      selectedSpace: undefined,
      isLoading: false,
      error: null,
    });
    useBrowseStore.setState({
      selectedPath: '',
      selectedSpace: undefined,
      content: [],
      isLoading: false,
      error: null,
      treeRefreshVersion: 0,
      treeInvalidationTargets: [],
      trashOpenRequest: null,
    });
  });

  it('normalizes system browse permission failures for content loading', async () => {
    vi.mocked(apiFetch).mockResolvedValueOnce(jsonResponse({ message: 'Permission denied' }, 403));

    await useBrowseStore.getState().fetchSystemContents('/private/tmp');

    expect(useBrowseStore.getState().error?.message).toBe(expectedBrowsePermissionGuidance);
  });

  it('normalizes space browse permission failures for content loading', async () => {
    vi.mocked(apiFetch).mockResolvedValueOnce(jsonResponse({ message: 'Permission denied' }, 403));

    await useBrowseStore.getState().fetchSpaceContents(7, 'secret');

    expect(useBrowseStore.getState().error?.message).toBe(expectedBrowsePermissionGuidance);
  });

  it('keeps unrelated browse failures unchanged', async () => {
    vi.mocked(apiFetch).mockResolvedValueOnce(jsonResponse({ message: 'Forbidden by policy' }, 403));

    await useBrowseStore.getState().fetchSystemContents('/policy');

    expect(useBrowseStore.getState().error?.message).toBe('Forbidden by policy');
  });
});
