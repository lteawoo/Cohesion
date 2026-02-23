import React from 'react';
import { Space as AntSpace, Button, Select, theme } from 'antd';
import {
  UploadOutlined,
  UnorderedListOutlined,
  AppstoreOutlined,
  LeftOutlined,
  RightOutlined,
} from '@ant-design/icons';
import type { ViewMode, SortConfig } from '../../types';
import { buildSortOptions } from '../../constants';
import { useTranslation } from 'react-i18next';

interface FolderContentToolbarProps {
  viewMode: ViewMode;
  sortConfig: SortConfig;
  canUpload: boolean;
  canGoBack?: boolean;
  canGoForward?: boolean;
  compact?: boolean;
  onGoBack?: () => void;
  onGoForward?: () => void;
  onUpload: () => void;
  onViewModeChange: (mode: ViewMode) => void;
  onSortChange: (config: SortConfig) => void;
}

const FolderContentToolbar: React.FC<FolderContentToolbarProps> = ({
  viewMode,
  sortConfig,
  canUpload,
  canGoBack = false,
  canGoForward = false,
  compact = false,
  onGoBack,
  onGoForward,
  onUpload,
  onViewModeChange,
  onSortChange,
}) => {
  const { t } = useTranslation();
  const sortOptions = buildSortOptions((key, options) => String(t(key, options)));
  const { token } = theme.useToken();
  const getViewModeStyle = (active: boolean) => (
    active
      ? {
          background: token.colorPrimary,
          borderColor: token.colorPrimary,
          color: token.colorTextLightSolid,
        }
      : {
          background: token.colorBgContainer,
          borderColor: token.colorBorder,
          color: token.colorText,
        }
  );

  return (
    <div
      style={{
        display: 'flex',
        justifyContent: 'flex-start',
        alignItems: 'center',
        height: compact ? '100%' : undefined,
        overflowX: compact ? 'hidden' : undefined,
        overflowY: compact ? 'hidden' : undefined,
        flexWrap: compact ? 'nowrap' : 'wrap',
        gap: compact ? '8px' : '8px 16px',
      }}
    >
      <AntSpace.Compact>
        <Button
          icon={<LeftOutlined />}
          onClick={onGoBack}
          disabled={!canGoBack}
          aria-label={t('folderContentToolbar.prevFolder')}
          title={t('folderContentToolbar.prevFolder')}
        />
        <Button
          icon={<RightOutlined />}
          onClick={onGoForward}
          disabled={!canGoForward}
          aria-label={t('folderContentToolbar.nextFolder')}
          title={t('folderContentToolbar.nextFolder')}
        />
      </AntSpace.Compact>
      <div style={{ marginLeft: 'auto', minWidth: 0 }}>
        <AntSpace wrap={!compact}>
          {canUpload && (
            <Button
              icon={<UploadOutlined />}
              onClick={onUpload}
              aria-label={t('folderContentToolbar.upload')}
              title={t('folderContentToolbar.upload')}
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
            options={sortOptions}
          />
          <AntSpace.Compact>
            <Button
              icon={<UnorderedListOutlined />}
              onClick={() => onViewModeChange('table')}
              type="default"
              style={getViewModeStyle(viewMode === 'table')}
              aria-label={t('folderContentToolbar.tableView')}
              title={t('folderContentToolbar.tableView')}
            />
            <Button
              icon={<AppstoreOutlined />}
              onClick={() => onViewModeChange('grid')}
              type="default"
              style={getViewModeStyle(viewMode === 'grid')}
              aria-label={t('folderContentToolbar.gridView')}
              title={t('folderContentToolbar.gridView')}
            />
          </AntSpace.Compact>
        </AntSpace>
      </div>
    </div>
  );
};

export default FolderContentToolbar;
