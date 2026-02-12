import { create } from 'zustand';
import type { FileNode } from '@/features/browse/types';
import type { Space } from '@/features/space/types';
import { useSpaceStore } from './spaceStore';

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

      // 현재 경로에 해당하는 Space 자동 탐색 (가장 긴 매칭을 선택)
      const spaces = useSpaceStore.getState().spaces;
      const matchingSpaces = spaces?.filter(s => path.startsWith(s.space_path)) || [];
      const matchingSpace = matchingSpaces.length > 0
        ? matchingSpaces.reduce((longest, current) =>
            current.space_path.length > longest.space_path.length ? current : longest
          )
        : undefined;

      set({ content: data, isLoading: false, selectedPath: path, selectedSpace: matchingSpace });
    } catch (e) {
      set({ error: e as Error, isLoading: false });
    }
  },

  clearContent: () => {
    set({ content: [], selectedPath: '', selectedSpace: undefined });
  },
}));
