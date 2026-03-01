import React from 'react';
import { Table, Dropdown, Button } from 'antd';
import type { MenuProps } from 'antd';
import { FolderFilled, MoreOutlined, EditOutlined, DeleteOutlined, CopyOutlined, DownloadOutlined } from '@ant-design/icons';
import type { FileNode } from '../../types';
import { formatDate, formatSize } from '../../constants';
import { FileTypeIcon } from '../FileTypeIcon';
import { useTranslation } from 'react-i18next';

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
  onItemDownload?: (record: FileNode) => void;
  onItemCopy?: (record: FileNode) => void;
  onItemMove?: (record: FileNode) => void;
  onItemRename?: (record: FileNode) => void;
  onItemDelete?: (record: FileNode) => void;
  showActions?: boolean;
  rowKeyResolver?: (record: FileNode) => string;
  renderName?: (record: FileNode) => React.ReactNode;
  renderMeta?: (record: FileNode) => React.ReactNode;
  emptyText?: React.ReactNode;
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
  showActions = true,
  rowKeyResolver,
  renderName,
  renderMeta,
  emptyText,
}) => {
  const { t } = useTranslation();
  const resolvedEmptyText = emptyText ?? t('folderContent.folderEmpty');
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
            label: record.isDir ? t('browseMenu.folderDownloadZip') : t('browseMenu.download'),
          },
          ...(canWriteFiles
            ? [
                {
                  key: 'copy',
                  icon: <CopyOutlined />,
                  label: t('browseMenu.copy'),
                },
                {
                  key: 'move',
                  icon: (
                    <span className="material-symbols-rounded move-action-icon" style={{ fontVariationSettings: '"FILL" 1, "wght" 500, "GRAD" 0, "opsz" 20' }}>
                      drive_file_move
                    </span>
                  ),
                  label: t('browseMenu.move'),
                },
                {
                  key: 'rename',
                  icon: <EditOutlined />,
                  label: t('browseMenu.rename'),
                },
                { type: 'divider' as const },
                {
                  key: 'delete',
                  icon: <DeleteOutlined />,
                  label: t('browseMenu.moveToTrash'),
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
                  {renderName ? renderName(record) : record.name}
                </div>
                <div style={{ fontSize: 12, opacity: 0.72 }}>
                  {renderMeta
                    ? renderMeta(record)
                    : `${record.isDir ? '-' : formatSize(record.size)} | ${formatDate(record.modTime)}`}
                </div>
              </div>
            </div>
            {showActions && (
              <Dropdown
                trigger={['click']}
                placement="bottomRight"
                menu={{
                  items: menuItems,
                  onClick: (info: Parameters<NonNullable<MenuProps['onClick']>>[0]) => {
                    const { key, domEvent } = info;
                    domEvent.stopPropagation();
                    if (key === 'download') onItemDownload?.(record);
                    if (key === 'copy') onItemCopy?.(record);
                    if (key === 'move') onItemMove?.(record);
                    if (key === 'rename') onItemRename?.(record);
                    if (key === 'delete') onItemDelete?.(record);
                  },
                }}
              >
                <Button
                  type="text"
                  size="small"
                  icon={<MoreOutlined />}
                  onClick={(e) => e.stopPropagation()}
                  aria-label={t('browseMenu.more')}
                  title={t('browseMenu.more')}
                />
              </Dropdown>
            )}
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
      rowKey={rowKeyResolver ?? 'path'}
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
      locale={{ emptyText: resolvedEmptyText }}
    />
  );
};

export default FolderContentTable;
