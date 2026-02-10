import { useMemo } from 'react';
import type { FileNode, SortConfig } from '../types';

/**
 * 파일/폴더 목록을 정렬하는 훅
 * - 폴더 우선 정렬
 * - sortConfig에 따른 2차 정렬 (이름/수정일/크기)
 * - 오름차순/내림차순 지원
 */
export function useSortedContent(
  content: FileNode[] | null,
  sortConfig: SortConfig
): FileNode[] {
  return useMemo(() => {
    if (!Array.isArray(content)) {
      return [];
    }

    const sorted = [...content].sort((a, b) => {
      // 1. 폴더 우선 정렬
      if (a.isDir !== b.isDir) {
        return a.isDir ? -1 : 1;
      }

      // 2. sortBy에 따른 정렬
      let result = 0;
      if (sortConfig.sortBy === 'name') {
        result = a.name.localeCompare(b.name);
      } else if (sortConfig.sortBy === 'modTime') {
        result = new Date(a.modTime).getTime() - new Date(b.modTime).getTime();
      } else if (sortConfig.sortBy === 'size') {
        result = a.size - b.size;
      }

      // 3. sortOrder 적용
      return sortConfig.sortOrder === 'ascend' ? result : -result;
    });

    return sorted;
  }, [content, sortConfig]);
}
