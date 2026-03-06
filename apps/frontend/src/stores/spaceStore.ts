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
  createSpace: (name: string, path: string) => Promise<void>;
  renameSpace: (id: number, name: string) => Promise<void>;
  deleteSpace: (id: number) => Promise<void>;
}

function normalizeUnknownError(error: unknown, fallbackMessage: string): Error {
  if (error instanceof Error) {
    return error;
  }
  return new Error(fallbackMessage);
}

function reconcileSelectedSpace(spaces: Space[], selectedSpace: Space | undefined): Space | undefined {
  if (!selectedSpace) {
    return undefined;
  }
  return spaces.find((space) => space.id === selectedSpace.id);
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
      set((state) => ({
        spaces: data,
        selectedSpace: reconcileSelectedSpace(data, state.selectedSpace),
        isLoading: false,
      }));
    } catch (e) {
      set({ error: normalizeUnknownError(e, i18n.t('storeErrors.loadSpaceListFailed')), isLoading: false });
    }
  },

  setSelectedSpace: (space: Space | undefined) => {
    set({ selectedSpace: space });
  },

  createSpace: async (name: string, path: string) => {
    set({ isLoading: true, error: null });
    try {
      const response = await apiFetch('/api/spaces', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          space_name: name,
          space_path: path,
        }),
      });

      if (!response.ok) {
        throw await toApiError(response, i18n.t('storeErrors.createSpaceFailed'));
      }

      await get().fetchSpaces();
    } catch (e) {
      const error = normalizeUnknownError(e, i18n.t('storeErrors.createSpaceFailed'));
      set({ error, isLoading: false });
      throw error;
    }
  },

  renameSpace: async (id: number, name: string) => {
    set({ isLoading: true, error: null });
    try {
      const response = await apiFetch(`/api/spaces/${id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          space_name: name.trim(),
        }),
      });

      if (!response.ok) {
        throw await toApiError(response, i18n.t('storeErrors.renameSpaceFailed'));
      }

      await get().fetchSpaces();
    } catch (e) {
      const error = normalizeUnknownError(e, i18n.t('storeErrors.renameSpaceFailed'));
      set({ error, isLoading: false });
      throw error;
    }
  },

  deleteSpace: async (id: number) => {
    set({ isLoading: true, error: null });
    try {
      const response = await apiFetch(`/api/spaces/${id}`, {
        method: 'DELETE',
      });

      if (!response.ok) {
        throw await toApiError(response, i18n.t('storeErrors.deleteSpaceFailed'));
      }

      await get().fetchSpaces();
    } catch (e) {
      const error = normalizeUnknownError(e, i18n.t('storeErrors.deleteSpaceFailed'));
      set({ error, isLoading: false });
      throw error;
    }
  },
}));
