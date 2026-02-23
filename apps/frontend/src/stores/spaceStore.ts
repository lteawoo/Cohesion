import { create } from 'zustand';
import type { Space } from '@/features/space/types';
import { apiFetch } from '@/api/client';
import { toApiError } from '@/api/error';
import i18n from '@/i18n';

interface SpaceStore {
  spaces: Space[];
  selectedSpace: Space | undefined;
  isLoading: boolean;
  error: Error | null;

  fetchSpaces: () => Promise<void>;
  setSelectedSpace: (space: Space | undefined) => void;
  createSpace: (name: string, path: string, description?: string) => Promise<void>;
  deleteSpace: (id: number) => Promise<void>;
}

function normalizeUnknownError(error: unknown, fallbackMessage: string): Error {
  if (error instanceof Error) {
    return error;
  }
  return new Error(fallbackMessage);
}

export const useSpaceStore = create<SpaceStore>((set, get) => ({
  spaces: [],
  selectedSpace: undefined,
  isLoading: false,
  error: null,

  fetchSpaces: async () => {
    set({ isLoading: true, error: null });
    try {
      const response = await apiFetch('/api/spaces');
      if (!response.ok) {
        throw await toApiError(response, i18n.t('storeErrors.loadSpaceListFailed'));
      }
      const data: Space[] = await response.json();
      set({ spaces: data, isLoading: false });
    } catch (e) {
      set({ error: normalizeUnknownError(e, i18n.t('storeErrors.loadSpaceListFailed')), isLoading: false });
    }
  },

  setSelectedSpace: (space: Space | undefined) => {
    set({ selectedSpace: space });
  },

  createSpace: async (name: string, path: string, description?: string) => {
    set({ isLoading: true, error: null });
    try {
      const trimmedDescription = description?.trim();
      const response = await apiFetch('/api/spaces', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          space_name: name,
          space_path: path,
          ...(trimmedDescription ? { space_desc: trimmedDescription } : {}),
        }),
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.message || i18n.t('storeErrors.createSpaceFailed'));
      }

      // Space 생성 후 목록 갱신
      await get().fetchSpaces();
    } catch (e) {
      set({ error: e as Error, isLoading: false });
      throw e;
    }
  },

  deleteSpace: async (id: number) => {
    set({ isLoading: true, error: null });
    try {
      const response = await apiFetch(`/api/spaces/${id}`, {
        method: 'DELETE',
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.message || i18n.t('storeErrors.deleteSpaceFailed'));
      }

      // Space 삭제 후 목록 갱신
      await get().fetchSpaces();
    } catch (e) {
      set({ error: e as Error, isLoading: false });
      throw e;
    }
  },
}));
