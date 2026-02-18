import { useMemo } from 'react';
import type { BreadcrumbProps } from 'antd';
import { FolderFilled } from '@ant-design/icons';
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
        title: (
          <a style={{ color: 'inherit', display: 'inline-flex', alignItems: 'center', gap: 6 }} onClick={() => onNavigate('', selectedSpace)}>
            <FolderFilled aria-hidden style={{ fontSize: 12 }} />
            <span>{selectedSpace.space_name}</span>
          </a>
        ),
        key: `space-${selectedSpace.id}`,
      },
    ];

    segments.forEach((curr, idx) => {
      const path = segments.slice(0, idx + 1).join('/');
      items.push({
        title: (
          <a style={{ color: 'inherit', display: 'inline-flex', alignItems: 'center', gap: 6 }} onClick={() => onNavigate(path, selectedSpace)}>
            <FolderFilled aria-hidden style={{ fontSize: 12 }} />
            <span>{curr}</span>
          </a>
        ),
        key: `${selectedSpace.id}:${path}`,
      });
    });

    return items;
  }, [selectedPath, selectedSpace, onNavigate]);

  return { breadcrumbItems };
}
