import { Space as AntSpace } from 'antd';
import type { TableColumnsType, MenuProps } from 'antd';
import {
  FolderFilled,
  FileOutlined,
  DownloadOutlined,
  DeleteOutlined,
  EditOutlined,
  CopyOutlined,
  ScissorOutlined,
  FolderOutlined,
} from '@ant-design/icons';
import type { FileNode } from './types';

type TranslationOptions = Record<string, unknown>;
type TranslateFn = (key: string, options?: TranslationOptions) => string;

// Utility functions
export const formatSize = (bytes: number): string => {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
};

export const formatDate = (dateString: string): string => {
  if (!dateString) return '-';
  const date = new Date(dateString);
  return date.toLocaleString();
};

// Table columns builder
export function buildTableColumns(
  onNavigate: (path: string) => void,
  sortConfig: { sortBy: string; sortOrder: 'ascend' | 'descend' },
  t?: TranslateFn
): TableColumnsType<FileNode> {
  const translate = t ?? ((key: string) => {
    if (key === 'browseTable.name') return 'Name';
    if (key === 'browseTable.modifiedAt') return 'Modified';
    if (key === 'browseTable.size') return 'Size';
    return key;
  });

  return [
    {
      title: translate('browseTable.name'),
      dataIndex: 'name',
      key: 'name',
      sorter: true, // Enable sorting UI
      sortOrder: sortConfig.sortBy === 'name' ? sortConfig.sortOrder : null,
      render: (text: string, record: FileNode) => (
        <AntSpace>
          {record.isDir ? <FolderFilled style={{ color: 'var(--app-folder-icon-color, #415a77)' }} /> : <FileOutlined />}
          {record.isDir ? (
            <a onClick={() => onNavigate(record.path)}>{text}</a>
          ) : (
            <span>{text}</span>
          )}
        </AntSpace>
      ),
    },
    {
      title: translate('browseTable.modifiedAt'),
      dataIndex: 'modTime',
      key: 'modTime',
      width: 200,
      sorter: true,
      sortOrder: sortConfig.sortBy === 'modTime' ? sortConfig.sortOrder : null,
      render: (date: string) => formatDate(date),
    },
    {
      title: translate('browseTable.size'),
      dataIndex: 'size',
      key: 'size',
      width: 120,
      sorter: true,
      sortOrder: sortConfig.sortBy === 'size' ? sortConfig.sortOrder : null,
      render: (size: number, record: FileNode) => (record.isDir ? '-' : formatSize(size)),
    },
  ];
}

// Context menu builders
export interface ContextMenuCallbacks {
  onDownload: (path: string) => void;
  onCopy: () => void;
  onMove: () => void;
  onRename: (record: FileNode) => void;
  onDelete: (record: FileNode) => void;
  onBulkDownload: () => void;
  onBulkDelete: () => void;
  onCreateFolder: () => void;
}

export function buildSingleItemMenu(
  record: FileNode,
  callbacks: ContextMenuCallbacks,
  t: TranslateFn,
  options?: { canWriteFiles?: boolean }
): MenuProps['items'] {
  const canWriteFiles = options?.canWriteFiles ?? true;
  return [
    {
      key: 'download',
      icon: <DownloadOutlined />,
      label: record.isDir ? t('browseMenu.folderDownloadZip') : t('browseMenu.download'),
      onClick: () => callbacks.onDownload(record.path),
    },
    ...(canWriteFiles
      ? [
          {
            key: 'copy',
            icon: <CopyOutlined />,
            label: t('browseMenu.copy'),
            onClick: callbacks.onCopy,
          },
          {
            key: 'move',
            icon: <ScissorOutlined />,
            label: t('browseMenu.move'),
            onClick: callbacks.onMove,
          },
          {
            key: 'rename',
            icon: <EditOutlined />,
            label: t('browseMenu.rename'),
            onClick: () => callbacks.onRename(record),
          },
          { type: 'divider' as const },
          {
            key: 'delete',
            icon: <DeleteOutlined />,
            label: t('browseMenu.moveToTrash'),
            danger: true,
            onClick: () => callbacks.onDelete(record),
          },
        ]
      : []),
  ];
}

export function buildMultiItemMenu(
  count: number,
  callbacks: Pick<ContextMenuCallbacks, 'onBulkDownload' | 'onCopy' | 'onMove' | 'onBulkDelete'>,
  t: TranslateFn,
  options?: { canWriteFiles?: boolean }
): MenuProps['items'] {
  const canWriteFiles = options?.canWriteFiles ?? true;
  return [
    {
      key: 'selection-summary',
      label: t('browseMenu.selectedCount', { count }),
      disabled: true,
    },
    { type: 'divider' as const },
    {
      key: 'download',
      icon: <DownloadOutlined />,
      label: t('browseMenu.download'),
      onClick: callbacks.onBulkDownload,
    },
    ...(canWriteFiles
      ? [
          {
            key: 'copy',
            icon: <CopyOutlined />,
            label: t('browseMenu.copy'),
            onClick: callbacks.onCopy,
          },
          {
            key: 'move',
            icon: <ScissorOutlined />,
            label: t('browseMenu.move'),
            onClick: callbacks.onMove,
          },
          { type: 'divider' as const },
          {
            key: 'delete',
            icon: <DeleteOutlined />,
            label: t('browseMenu.moveToTrash'),
            danger: true,
            onClick: callbacks.onBulkDelete,
          },
        ]
      : []),
  ];
}

export function buildEmptyAreaMenu(
  onCreateFolder: () => void,
  t: TranslateFn,
  options?: { canWriteFiles?: boolean }
): MenuProps['items'] {
  const canWriteFiles = options?.canWriteFiles ?? true;
  if (!canWriteFiles) {
    return [];
  }
  return [
    {
      key: 'create-folder',
      icon: <FolderOutlined />,
      label: t('browseMenu.createFolder'),
      onClick: onCreateFolder,
    },
  ];
}

// Sort options
export function buildSortOptions(t: TranslateFn) {
  return [
    { value: 'name-ascend', label: t('browseMenu.sortNameAsc') },
    { value: 'name-descend', label: t('browseMenu.sortNameDesc') },
    { value: 'modTime-ascend', label: t('browseMenu.sortModTimeAsc') },
    { value: 'modTime-descend', label: t('browseMenu.sortModTimeDesc') },
    { value: 'size-ascend', label: t('browseMenu.sortSizeAsc') },
    { value: 'size-descend', label: t('browseMenu.sortSizeDesc') },
  ];
}
