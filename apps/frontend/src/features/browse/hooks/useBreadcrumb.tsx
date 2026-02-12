import { useMemo } from 'react';
import type { BreadcrumbProps } from 'antd';
import type { Space } from '@/features/space/types';

export type BreadcrumbItem = NonNullable<BreadcrumbProps['items']>[number];

interface UseBreadcrumbParams {
  selectedPath: string;
  selectedSpace?: Space;
  onNavigate: (path: string, space?: Space) => void;
}

export function useBreadcrumb({ selectedPath, selectedSpace, onNavigate }: UseBreadcrumbParams) {
  const breadcrumbItems = useMemo<BreadcrumbItem[]>(() => {
    if (!selectedPath) return [];

    // Space가 선택된 경우 상대 경로로 표시
    if (selectedSpace && selectedPath.startsWith(selectedSpace.space_path)) {
      const relativePath = selectedPath.slice(selectedSpace.space_path.length);
      const segments = relativePath.split('/').filter(Boolean);

      const items: BreadcrumbItem[] = [
        {
          title: <a onClick={() => onNavigate(selectedSpace.space_path, selectedSpace)}>{selectedSpace.space_name}</a>,
          key: selectedSpace.space_path,
        },
      ];

      segments.forEach((curr, idx) => {
        const path = selectedSpace.space_path + '/' + segments.slice(0, idx + 1).join('/');
        items.push({
          title: <a onClick={() => onNavigate(path, selectedSpace)}>{curr}</a>,
          key: path,
        });
      });

      return items;
    }

    // Space가 없는 경우 절대 경로로 표시 (기존 로직)
    return selectedPath.split('/').filter(Boolean).reduce<BreadcrumbItem[]>(
      (acc, curr, idx, array) => {
        const path = '/' + array.slice(0, idx + 1).join('/');
        acc.push({
          title: <a onClick={() => onNavigate(path)}>{curr}</a>,
          key: path,
        });
        return acc;
      },
      [{ title: <a onClick={() => onNavigate('/')}>Root</a>, key: '/' }]
    );
  }, [selectedPath, selectedSpace, onNavigate]);

  return { breadcrumbItems };
}
