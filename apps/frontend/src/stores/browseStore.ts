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
  fetchSpaceContents: (spaceId: number, relativePath: string) => Promise<void>;
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

      // ✅ Space 매칭 로직 제거 - systemMode 전용 함수
      // Space 정보는 fetchSpaceContents 사용 시에만 설정됨
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

  clearContent: () => {
    set({ content: [], selectedPath: '', selectedSpace: undefined });
  },
}));
