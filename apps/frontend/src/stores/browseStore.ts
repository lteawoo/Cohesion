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
  treeRefreshVersion: number;

  setPath: (path: string, space?: Space) => void;
  fetchSystemContents: (path: string) => Promise<void>;
  fetchSpaceContents: (spaceId: number, relativePath: string) => Promise<void>;
  invalidateTree: () => void;
  clearContent: () => void;
}

export const useBrowseStore = create<BrowseStore>((set) => ({
  selectedPath: '',
  selectedSpace: undefined,
  content: [],
  isLoading: false,
  error: null,
  treeRefreshVersion: 0,

  setPath: (path: string, space?: Space) => {
    set((state) => ({
      selectedPath: path,
      selectedSpace: space !== undefined ? space : state.selectedSpace,
    }));
  },

  // Space 등록 모달 전용 — Space 외부 시스템 탐색에서만 사용
  fetchSystemContents: async (path: string) => {
    set({ isLoading: true, error: null });
    try {
      const url = `/api/browse?path=${encodeURIComponent(path)}&system=true`;
      const response = await fetch(url);
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
      const data: FileNode[] = await response.json();
      set({ content: data, isLoading: false });
    } catch (e) {
      set({ error: e as Error, isLoading: false });
    }
  },

  fetchSpaceContents: async (spaceId: number, relativePath: string) => {
    set({ isLoading: true, error: null });
    try {
      const url = `/api/spaces/${spaceId}/browse?path=${encodeURIComponent(relativePath)}`;
      const response = await fetch(url);
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
      const data: FileNode[] = await response.json();

      // Space 정보는 ID로 확정 (매칭 불필요!)
      const space = useSpaceStore.getState().spaces.find(s => s.id === spaceId);

      set({
        content: data,
        isLoading: false,
        selectedSpace: space  // 덮어쓰기 없음
      });
    } catch (e) {
      set({ error: e as Error, isLoading: false });
    }
  },

  invalidateTree: () => {
    set((state) => ({ treeRefreshVersion: state.treeRefreshVersion + 1 }));
  },

  clearContent: () => {
    set({ content: [], selectedPath: '', selectedSpace: undefined });
  },
}));
