import React from 'react';
import { Space as AntSpace, Button, Select } from 'antd';
import { UploadOutlined, UnorderedListOutlined, AppstoreOutlined } from '@ant-design/icons';
import type { ViewMode, SortConfig } from '../../types';
import { SORT_OPTIONS } from '../../constants';

interface FolderContentToolbarProps {
  viewMode: ViewMode;
  sortConfig: SortConfig;
  canUpload: boolean;
  compact?: boolean;
  onUpload: () => void;
  onViewModeChange: (mode: ViewMode) => void;
  onSortChange: (config: SortConfig) => void;
}

const FolderContentToolbar: React.FC<FolderContentToolbarProps> = ({
  viewMode,
  sortConfig,
  canUpload,
  compact = false,
  onUpload,
  onViewModeChange,
  onSortChange,
}) => {
  return (
    <div
      style={{
        display: 'flex',
        justifyContent: 'flex-end',
        alignItems: 'center',
        height: compact ? '100%' : undefined,
        overflowX: compact ? 'hidden' : undefined,
        overflowY: compact ? 'hidden' : undefined,
        flexWrap: compact ? 'nowrap' : 'wrap',
        gap: compact ? '8px' : '8px 16px',
      }}
    >
      <AntSpace wrap={!compact}>
        {canUpload && (
          <Button
            icon={<UploadOutlined />}
            onClick={onUpload}
            aria-label="업로드"
            title="업로드"
          />
        )}
        <Select
          popupMatchSelectWidth={false}
          style={{ width: 'fit-content' }}
          value={`${sortConfig.sortBy}-${sortConfig.sortOrder}`}
          onChange={(value: string) => {
            const [sortBy, sortOrder] = value.split('-') as [
              'name' | 'modTime' | 'size',
              'ascend' | 'descend'
            ];
            onSortChange({ sortBy, sortOrder });
          }}
          options={SORT_OPTIONS}
        />
        <AntSpace.Compact>
          <Button
            icon={<UnorderedListOutlined />}
            onClick={() => onViewModeChange('table')}
            type={viewMode === 'table' ? 'primary' : 'default'}
          />
          <Button
            icon={<AppstoreOutlined />}
            onClick={() => onViewModeChange('grid')}
            type={viewMode === 'grid' ? 'primary' : 'default'}
          />
        </AntSpace.Compact>
      </AntSpace>
    </div>
  );
};

export default FolderContentToolbar;
