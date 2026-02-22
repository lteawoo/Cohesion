import { create } from 'zustand';
import type { FileNode } from '@/features/browse/types';
import type { Space } from '@/features/space/types';
import { useSpaceStore } from './spaceStore';
import { apiFetch } from '@/api/client';
import { toApiError } from '@/api/error';

function normalizeRelativePath(path: string): string {
  return path.replace(/^\/+/, '').replace(/\/+$/, '');
}

export interface TreeInvalidationTarget {
  path: string;
  spaceId?: number;
}

interface TrashOpenRequest {
  spaceId: number;
  nonce: number;
}

interface BrowseStore {
  selectedPath: string;
  selectedSpace: Space | undefined;
  content: FileNode[];
  isLoading: boolean;
  error: Error | null;
  treeRefreshVersion: number;
  treeInvalidationTargets: TreeInvalidationTarget[];
  trashOpenRequest: TrashOpenRequest | null;

  setPath: (path: string, space?: Space) => void;
  fetchSystemContents: (path: string) => Promise<void>;
  fetchSpaceContents: (spaceId: number, relativePath: string) => Promise<void>;
  invalidateTree: (targets?: TreeInvalidationTarget[]) => void;
  requestOpenTrash: (spaceId: number) => void;
  clearTrashOpenRequest: () => void;
  clearContent: () => void;
}

function normalizeUnknownError(error: unknown, fallbackMessage: string): Error {
  if (error instanceof Error) {
    return error;
  }
  return new Error(fallbackMessage);
}

export const useBrowseStore = create<BrowseStore>((set) => ({
  selectedPath: '',
  selectedSpace: undefined,
  content: [],
  isLoading: false,
  error: null,
  treeRefreshVersion: 0,
  treeInvalidationTargets: [],
  trashOpenRequest: null,

  setPath: (path: string, space?: Space) => {
    set((state) => {
      const nextSpace = space !== undefined ? space : state.selectedSpace;
      return {
        selectedPath: nextSpace ? normalizeRelativePath(path) : path,
        selectedSpace: nextSpace,
      };
    });
  },

  // Space 등록 모달 전용 — Space 외부 시스템 탐색에서만 사용
  fetchSystemContents: async (path: string) => {
    set({ isLoading: true, error: null });
    try {
      const url = `/api/browse?path=${encodeURIComponent(path)}&system=true`;
      const response = await apiFetch(url);
      if (!response.ok) {
        throw await toApiError(response, '디렉토리 목록을 불러오지 못했습니다.');
      }
      const data: FileNode[] = await response.json();
      set({ content: data, isLoading: false });
    } catch (e) {
      set({ error: normalizeUnknownError(e, '디렉토리 목록을 불러오지 못했습니다.'), isLoading: false });
    }
  },

  fetchSpaceContents: async (spaceId: number, relativePath: string) => {
    set({ isLoading: true, error: null });
    try {
      const url = `/api/spaces/${spaceId}/browse?path=${encodeURIComponent(relativePath)}`;
      const response = await apiFetch(url);
      if (!response.ok) {
        throw await toApiError(response, 'Space 폴더 목록을 불러오지 못했습니다.');
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
      set({ error: normalizeUnknownError(e, 'Space 폴더 목록을 불러오지 못했습니다.'), isLoading: false });
    }
  },

  invalidateTree: (targets?: TreeInvalidationTarget[]) => {
    const normalizedTargets = (targets ?? [])
      .filter((target) => target.path)
      .map((target) => ({
        path: target.path,
        spaceId: target.spaceId,
      }));

    const dedupedTargets = Array.from(
      new Map(normalizedTargets.map((target) => [`${target.spaceId ?? 'none'}::${target.path}`, target])).values()
    );

    set((state) => ({
      treeRefreshVersion: state.treeRefreshVersion + 1,
      treeInvalidationTargets: dedupedTargets,
    }));
  },

  requestOpenTrash: (spaceId: number) => {
    set((state) => ({
      trashOpenRequest: {
        spaceId,
        nonce: (state.trashOpenRequest?.nonce ?? 0) + 1,
      },
    }));
  },

  clearTrashOpenRequest: () => {
    set({ trashOpenRequest: null });
  },

  clearContent: () => {
    set({ content: [], selectedPath: '', selectedSpace: undefined, trashOpenRequest: null });
  },
}));
