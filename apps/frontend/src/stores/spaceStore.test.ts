import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useSpaceStore } from './spaceStore';
import { apiFetch } from '@/api/client';
import type { Space } from '@/features/space/types';

vi.mock('@/api/client', () => ({
  apiFetch: vi.fn(),
}));

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: {
      'Content-Type': 'application/json',
    },
  });
}

describe('useSpaceStore', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useSpaceStore.setState({
      spaces: [],
      selectedSpace: undefined,
      isLoading: false,
      error: null,
    });
  });

  it('renames a space and refreshes the shared space list', async () => {
    const selectedSpace: Space = { id: 1, space_name: 'Alpha' };
    useSpaceStore.setState({
      spaces: [selectedSpace],
      selectedSpace,
    });

    vi.mocked(apiFetch)
      .mockResolvedValueOnce(jsonResponse({ id: 1, space_name: 'Alpha Renamed' }))
      .mockResolvedValueOnce(jsonResponse([{ id: 1, space_name: 'Alpha Renamed' }]));

    await useSpaceStore.getState().renameSpace(1, ' Alpha Renamed ');

    expect(apiFetch).toHaveBeenNthCalledWith(1, '/api/spaces/1', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ space_name: 'Alpha Renamed' }),
    });
    expect(apiFetch).toHaveBeenNthCalledWith(2, '/api/spaces');
    expect(useSpaceStore.getState().spaces).toEqual([{ id: 1, space_name: 'Alpha Renamed' }]);
    expect(useSpaceStore.getState().selectedSpace?.space_name).toBe('Alpha Renamed');
  });

  it('validates a candidate space root through the dedicated endpoint', async () => {
    vi.mocked(apiFetch).mockResolvedValueOnce(jsonResponse({
      valid: false,
      code: 'permission_denied',
      message: 'Selected folder is not readable by the server',
    }));

    const result = await useSpaceStore.getState().validateSpaceRoot('/Users/tester/Downloads');

    expect(apiFetch).toHaveBeenCalledWith('/api/spaces/validate-root', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ space_path: '/Users/tester/Downloads' }),
    });
    expect(result).toEqual({
      valid: false,
      code: 'permission_denied',
      message: 'Selected folder is not readable by the server',
    });
  });

  it('keeps non-browse 403 handling unchanged for space creation', async () => {
    vi.mocked(apiFetch).mockResolvedValueOnce(jsonResponse({ error: 'Permission denied' }, 403));

    await expect(useSpaceStore.getState().createSpace('Alpha', '/Users/tester/Downloads')).rejects.toThrow('Permission denied');

    expect(useSpaceStore.getState().error?.message).toBe('Permission denied');
  });
});
