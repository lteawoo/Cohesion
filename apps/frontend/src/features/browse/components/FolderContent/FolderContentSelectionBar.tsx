import React from 'react';
import { Space as AntSpace, Button } from 'antd';
import { DownloadOutlined, CopyOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

interface FolderContentSelectionBarProps {
  selectedCount: number;
  showRename: boolean;
  canWrite: boolean;
  onDownload: () => void;
  onCopy: () => void;
  onMove: () => void;
  onRename?: () => void;
  onDelete: () => void;
  onClear: () => void;
}

const FolderContentSelectionBar: React.FC<FolderContentSelectionBarProps> = ({
  selectedCount,
  showRename,
  canWrite,
  onDownload,
  onCopy,
  onMove,
  onRename,
  onDelete,
  onClear,
}) => {
  const { t } = useTranslation();
  if (selectedCount === 0) return null;
  const moveActionIcon = (
    <span
      className="material-symbols-rounded"
      style={{
        fontSize: 18,
        lineHeight: 1,
        fontVariationSettings: '"FILL" 1, "wght" 500, "GRAD" 0, "opsz" 20',
      }}
      aria-hidden="true"
    >
      drive_file_move
    </span>
  );

  return (
    <div
      style={{
        padding: '8px 16px',
        backgroundColor: 'var(--browse-selection-bg)',
        border: '1px solid var(--browse-selection-border-color)',
        borderRadius: '4px',
        display: 'flex',
        alignItems: 'center',
        flexWrap: 'wrap',
        gap: '16px',
      }}
    >
      <span style={{ fontWeight: 'bold', color: 'var(--ant-color-primary, #415a77)' }}>
        ✓ {t('folderContent.selectedCount', { count: selectedCount })}
      </span>
      <AntSpace size="small" wrap>
        <Button size="small" icon={<DownloadOutlined />} onClick={onDownload}>
          {t('folderContent.download')}
        </Button>
        {canWrite && (
          <Button size="small" icon={<CopyOutlined />} onClick={onCopy}>
            {t('folderContent.copy')}
          </Button>
        )}
        {canWrite && (
          <Button size="small" icon={moveActionIcon} onClick={onMove}>
            {t('folderContent.move')}
          </Button>
        )}
        {canWrite && showRename && onRename && (
          <Button size="small" icon={<EditOutlined />} onClick={onRename}>
            {t('folderContent.rename')}
          </Button>
        )}
        {canWrite && (
          <Button size="small" icon={<DeleteOutlined />} danger onClick={onDelete}>
            {t('folderContent.moveToTrash')}
          </Button>
        )}
        <Button size="small" onClick={onClear}>
          {t('folderContent.clearSelection')}
        </Button>
      </AntSpace>
    </div>
  );
};

export default FolderContentSelectionBar;
