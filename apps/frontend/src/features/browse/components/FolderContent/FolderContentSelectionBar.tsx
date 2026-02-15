import React from 'react';
import { Space as AntSpace, Button } from 'antd';
import { DownloadOutlined, CopyOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';

interface FolderContentSelectionBarProps {
  selectedCount: number;
  showRename: boolean;
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
        backgroundColor: 'rgba(24, 144, 255, 0.1)',
        borderRadius: '4px',
        display: 'flex',
        alignItems: 'center',
        flexWrap: 'wrap',
        gap: '16px',
      }}
    >
      <span style={{ fontWeight: 'bold', color: '#1890ff' }}>✓ {selectedCount}개 선택됨</span>
      <AntSpace size="small" wrap>
        <Button size="small" icon={<DownloadOutlined />} onClick={onDownload}>
          다운로드
        </Button>
        <Button size="small" icon={<CopyOutlined />} onClick={onCopy}>
          복사
        </Button>
        <Button size="small" icon={moveActionIcon} onClick={onMove}>
          이동
        </Button>
        {showRename && onRename && (
          <Button size="small" icon={<EditOutlined />} onClick={onRename}>
            이름 변경
          </Button>
        )}
        <Button size="small" icon={<DeleteOutlined />} danger onClick={onDelete}>
          삭제
        </Button>
        <Button size="small" onClick={onClear}>
          선택 해제
        </Button>
      </AntSpace>
    </div>
  );
};

export default FolderContentSelectionBar;
