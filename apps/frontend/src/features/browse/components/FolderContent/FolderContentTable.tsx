import React from 'react';
import { Table, Dropdown, Button } from 'antd';
import type { MenuProps } from 'antd';
import { FolderFilled, MoreOutlined, EditOutlined, DeleteOutlined, CopyOutlined, DownloadOutlined } from '@ant-design/icons';
import type { FileNode } from '../../types';
import { formatDate, formatSize } from '../../constants';
import { FileTypeIcon } from '../FileTypeIcon';

interface FolderContentTableProps {
  dataSource: FileNode[];
  loading: boolean;
  selectedItems: Set<string>;
  dragOverFolder: string | null;
  onItemClick: (e: React.MouseEvent<HTMLElement>, record: FileNode, index: number) => void;
  onItemDoubleClick: (path: string) => void;
  onItemTouchStart?: (record: FileNode, index: number) => void;
  onItemTouchEnd?: () => void;
  onItemTouchCancel?: () => void;
  onContextMenu: (e: React.MouseEvent<HTMLElement>, record: FileNode) => void;
  onItemDragStart: (e: React.DragEvent<HTMLElement>, record: FileNode) => void;
  onItemDragEnd: (e: React.DragEvent<HTMLElement>) => void;
  onFolderDragOver: (e: React.DragEvent<HTMLElement>, record: FileNode) => void;
  onFolderDragLeave: (e: React.DragEvent<HTMLElement>) => void;
  onFolderDrop: (e: React.DragEvent<HTMLElement>, record: FileNode) => void;
  disableDrag?: boolean;
  canWriteFiles: boolean;
  onItemDownload: (record: FileNode) => void;
  onItemCopy: (record: FileNode) => void;
  onItemMove: (record: FileNode) => void;
  onItemRename: (record: FileNode) => void;
  onItemDelete: (record: FileNode) => void;
}

const FolderContentTable: React.FC<FolderContentTableProps> = ({
  dataSource,
  loading,
  selectedItems,
  dragOverFolder,
  onItemClick,
  onItemDoubleClick,
  onItemTouchStart,
  onItemTouchEnd,
  onItemTouchCancel,
  onContextMenu,
  onItemDragStart,
  onItemDragEnd,
  onFolderDragOver,
  onFolderDragLeave,
  onFolderDrop,
  disableDrag = false,
  canWriteFiles,
  onItemDownload,
  onItemCopy,
  onItemMove,
  onItemRename,
  onItemDelete,
}) => {
  const columns = [
    {
      key: 'entry',
      dataIndex: 'name',
      width: '100%',
      render: (_: string, record: FileNode) => {
        const menuItems: MenuProps['items'] = [
          {
            key: 'download',
            icon: <DownloadOutlined />,
            label: record.isDir ? '폴더 다운로드 (ZIP)' : '다운로드',
          },
          ...(canWriteFiles
            ? [
                {
                  key: 'copy',
                  icon: <CopyOutlined />,
                  label: '복사',
                },
                {
                  key: 'move',
                  icon: (
                    <span className="material-symbols-rounded move-action-icon" style={{ fontVariationSettings: '"FILL" 1, "wght" 500, "GRAD" 0, "opsz" 20' }}>
                      drive_file_move
                    </span>
                  ),
                  label: '이동',
                },
                {
                  key: 'rename',
                  icon: <EditOutlined />,
                  label: '이름 변경',
                },
                { type: 'divider' as const },
                {
                  key: 'delete',
                  icon: <DeleteOutlined />,
                  label: '삭제',
                  danger: true,
                },
              ]
            : []),
        ];

        return (
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12, width: '100%' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 10, minWidth: 0, flex: 1 }}>
              {record.isDir ? (
                <FolderFilled style={{ color: 'var(--app-folder-icon-color, #415a77)', fontSize: 18 }} />
              ) : (
                <FileTypeIcon filename={record.name} size={18} />
              )}
              <div style={{ minWidth: 0, flex: 1 }}>
                <div title={record.name} style={{ fontWeight: 500, color: 'inherit', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {record.name}
                </div>
                <div style={{ fontSize: 12, opacity: 0.72 }}>
                  {record.isDir ? '-' : formatSize(record.size)} | {formatDate(record.modTime)}
                </div>
              </div>
            </div>
            <Dropdown
              trigger={['click']}
              placement="bottomRight"
              menu={{
                items: menuItems,
                onClick: (info: Parameters<NonNullable<MenuProps['onClick']>>[0]) => {
                  const { key, domEvent } = info;
                  domEvent.stopPropagation();
                  if (key === 'download') onItemDownload(record);
                  if (key === 'copy') onItemCopy(record);
                  if (key === 'move') onItemMove(record);
                  if (key === 'rename') onItemRename(record);
                  if (key === 'delete') onItemDelete(record);
                },
              }}
            >
              <Button
                type="text"
                size="small"
                icon={<MoreOutlined />}
                onClick={(e) => e.stopPropagation()}
                aria-label="더보기"
                title="더보기"
              />
            </Dropdown>
          </div>
        );
      },
    },
  ];

  return (
    <Table<FileNode>
      dataSource={dataSource}
      columns={columns}
      tableLayout="fixed"
      loading={loading}
      rowKey="path"
      pagination={false}
      showHeader={false}
      rowClassName={(record: FileNode) => {
        if (dragOverFolder === record.path) {
          return 'folder-content-row folder-content-row-dragover';
        }
        if (selectedItems.has(record.path)) {
          return 'folder-content-row folder-content-row-selected';
        }
        return 'folder-content-row';
      }}
      onRow={(record: FileNode, index?: number) => ({
        onClick: (e: React.MouseEvent<HTMLElement>) => onItemClick(e, record, index ?? 0),
        onDoubleClick: () => record.isDir && onItemDoubleClick(record.path),
        onTouchStart: () => onItemTouchStart?.(record, index ?? 0),
        onTouchEnd: () => onItemTouchEnd?.(),
        onTouchCancel: () => onItemTouchCancel?.(),
        onContextMenu: (e: React.MouseEvent<HTMLElement>) => onContextMenu(e, record),
        draggable: !disableDrag,
        onDragStart: disableDrag ? undefined : (e: React.DragEvent<HTMLElement>) => onItemDragStart(e, record),
        onDragEnd: disableDrag ? undefined : (e: React.DragEvent<HTMLElement>) => onItemDragEnd(e),
        onDragOver: disableDrag ? undefined : (e: React.DragEvent<HTMLElement>) => record.isDir && onFolderDragOver(e, record),
        onDragLeave: disableDrag ? undefined : (e: React.DragEvent<HTMLElement>) => record.isDir && onFolderDragLeave(e),
        onDrop: disableDrag ? undefined : (e: React.DragEvent<HTMLElement>) => record.isDir && onFolderDrop(e, record),
      })}
      locale={{ emptyText: '이 폴더는 비어 있습니다.' }}
    />
  );
};

export default FolderContentTable;
