import React from 'react';
import { Space as AntSpace, Button } from 'antd';
import { DownloadOutlined, CopyOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';

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
      <span style={{ fontWeight: 'bold', color: 'var(--ant-color-primary, #415a77)' }}>✓ {selectedCount}개 선택됨</span>
      <AntSpace size="small" wrap>
        <Button size="small" icon={<DownloadOutlined />} onClick={onDownload}>
          다운로드
        </Button>
        {canWrite && (
          <Button size="small" icon={<CopyOutlined />} onClick={onCopy}>
            복사
          </Button>
        )}
        {canWrite && (
          <Button size="small" icon={moveActionIcon} onClick={onMove}>
            이동
          </Button>
        )}
        {canWrite && showRename && onRename && (
          <Button size="small" icon={<EditOutlined />} onClick={onRename}>
            이름 변경
          </Button>
        )}
        {canWrite && (
          <Button size="small" icon={<DeleteOutlined />} danger onClick={onDelete}>
            휴지통 이동
          </Button>
        )}
        <Button size="small" onClick={onClear}>
          선택 해제
        </Button>
      </AntSpace>
    </div>
  );
};

export default FolderContentSelectionBar;
