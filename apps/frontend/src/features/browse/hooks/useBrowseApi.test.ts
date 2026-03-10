import { act, renderHook } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useBrowseApi } from './useBrowseApi';

const h = vi.hoisted(() => ({
  apiFetch: vi.fn(),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

vi.mock('@/api/client', () => ({
  apiFetch: h.apiFetch,
}));

function okJsonResponse(payload: unknown): Response {
  return {
    ok: true,
    json: vi.fn().mockResolvedValue(payload),
  } as unknown as Response;
}

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: {
      'Content-Type': 'application/json',
    },
  });
}

describe('useBrowseApi endpoint boundaries', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    h.apiFetch.mockResolvedValue(okJsonResponse([]));
  });

  it('uses system browse endpoints for base directories and system path listing', async () => {
    const { result } = renderHook(() => useBrowseApi());

    await act(async () => {
      await result.current.fetchBaseDirectories();
      await result.current.fetchDirectoryContents('/dev');
    });

    expect(h.apiFetch).toHaveBeenNthCalledWith(1, '/api/browse/base-directories');
    expect(h.apiFetch).toHaveBeenNthCalledWith(2, '/api/browse?path=%2Fdev');
  });

  it('uses space browse endpoint for space-scoped directory listing', async () => {
    const { result } = renderHook(() => useBrowseApi());

    await act(async () => {
      await result.current.fetchSpaceDirectoryContents(17, '/docs');
    });

    expect(h.apiFetch).toHaveBeenCalledWith('/api/spaces/17/browse?path=%2Fdocs');
    expect(h.apiFetch).not.toHaveBeenCalledWith('/api/browse?path=%2Fdocs');
  });

  it('normalizes permission denied for system browse directory listing', async () => {
    h.apiFetch.mockResolvedValueOnce(jsonResponse({ message: 'Permission denied' }, 403));
    const { result } = renderHook(() => useBrowseApi());
    const expectedMessage = 'browseApi.permissionDeniedReason directorySetup.validation.permissionDeniedHint';

    await act(async () => {
      await expect(result.current.fetchDirectoryContents('/private/tmp')).rejects.toThrow(expectedMessage);
    });

    expect(result.current.error?.message).toBe(expectedMessage);
  });

  it('normalizes permission denied for space-scoped directory listing', async () => {
    h.apiFetch.mockResolvedValueOnce(jsonResponse({ message: 'Permission denied' }, 403));
    const { result } = renderHook(() => useBrowseApi());
    const expectedMessage = 'browseApi.permissionDeniedReason directorySetup.validation.permissionDeniedHint';

    await act(async () => {
      await expect(result.current.fetchSpaceDirectoryContents(7, '/secret')).rejects.toThrow(expectedMessage);
    });

    expect(result.current.error?.message).toBe(expectedMessage);
  });

  it('keeps unrelated 403 browse failures unchanged', async () => {
    h.apiFetch.mockResolvedValueOnce(jsonResponse({ message: 'Forbidden by policy' }, 403));
    const { result } = renderHook(() => useBrowseApi());

    await act(async () => {
      await expect(result.current.fetchDirectoryContents('/policy')).rejects.toThrow('Forbidden by policy');
    });

    expect(result.current.error?.message).toBe('Forbidden by policy');
  });
});
