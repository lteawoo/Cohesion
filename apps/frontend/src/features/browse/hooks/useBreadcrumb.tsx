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
    if (!selectedSpace) return [];

    const segments = selectedPath.split('/').filter(Boolean);
    const items: BreadcrumbItem[] = [
      {
        title: <a onClick={() => onNavigate('', selectedSpace)}>{selectedSpace.space_name}</a>,
        key: `space-${selectedSpace.id}`,
      },
    ];

    segments.forEach((curr, idx) => {
      const path = segments.slice(0, idx + 1).join('/');
      items.push({
        title: <a onClick={() => onNavigate(path, selectedSpace)}>{curr}</a>,
        key: `${selectedSpace.id}:${path}`,
      });
    });

    return items;
  }, [selectedPath, selectedSpace, onNavigate]);

  return { breadcrumbItems };
}
