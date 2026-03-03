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

vi.mock('@/api/error', () => ({
  toApiError: vi.fn(async (_response: Response, fallbackErrorMessage: string) => new Error(fallbackErrorMessage)),
}));

function okJsonResponse(payload: unknown): Response {
  return {
    ok: true,
    json: vi.fn().mockResolvedValue(payload),
  } as unknown as Response;
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
});
