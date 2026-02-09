import { create } from 'zustand';
import type { FileNode } from '@/features/browse/types';
import type { Space } from '@/features/space/types';

interface BrowseStore {
  selectedPath: string;
  selectedSpace: Space | undefined;
  content: FileNode[];
  isLoading: boolean;
  error: Error | null;

  setPath: (path: string, space?: Space) => void;
  fetchDirectoryContents: (path: string, systemMode?: boolean) => Promise<void>;
  clearContent: () => void;
}

export const useBrowseStore = create<BrowseStore>((set) => ({
  selectedPath: '',
  selectedSpace: undefined,
  content: [],
  isLoading: false,
  error: null,

  setPath: (path: string, space?: Space) => {
    set({ selectedPath: path, selectedSpace: space });
  },

  fetchDirectoryContents: async (path: string, systemMode = false) => {
    set({ isLoading: true, error: null });
    try {
      const url = systemMode
        ? `/api/browse?path=${encodeURIComponent(path)}&system=true`
        : `/api/browse?path=${encodeURIComponent(path)}`;

      const response = await fetch(url);
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
      const data: FileNode[] = await response.json();
      set({ content: data, isLoading: false, selectedPath: path });
    } catch (e) {
      set({ error: e as Error, isLoading: false });
    }
  },

  clearContent: () => {
    set({ content: [], selectedPath: '', selectedSpace: undefined });
  },
}));
