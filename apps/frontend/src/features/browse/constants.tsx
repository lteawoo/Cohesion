import { Space as AntSpace } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import type { MenuProps } from 'antd';
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
export function buildTableColumns(onNavigate: (path: string) => void): ColumnsType<FileNode> {
  return [
    {
      title: '이름',
      dataIndex: 'name',
      key: 'name',
      render: (text: string, record: FileNode) => (
        <AntSpace>
          {record.isDir ? <FolderFilled style={{ color: '#ffca28' }} /> : <FileOutlined />}
          {record.isDir ? (
            <a onClick={() => onNavigate(record.path)}>{text}</a>
          ) : (
            <span>{text}</span>
          )}
        </AntSpace>
      ),
      sorter: (a, b) => a.name.localeCompare(b.name),
    },
    {
      title: '수정일',
      dataIndex: 'modTime',
      key: 'modTime',
      width: 200,
      render: (date: string) => formatDate(date),
      sorter: (a, b) => new Date(a.modTime).getTime() - new Date(b.modTime).getTime(),
    },
    {
      title: '크기',
      dataIndex: 'size',
      key: 'size',
      width: 120,
      render: (size: number, record: FileNode) => (record.isDir ? '-' : formatSize(size)),
      sorter: (a, b) => a.size - b.size,
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
}

export function buildSingleItemMenu(
  record: FileNode,
  callbacks: ContextMenuCallbacks
): MenuProps['items'] {
  return [
    {
      key: 'download',
      icon: <DownloadOutlined />,
      label: record.isDir ? '폴더 다운로드 (ZIP)' : '다운로드',
      onClick: () => callbacks.onDownload(record.path),
    },
    {
      key: 'copy',
      icon: <CopyOutlined />,
      label: '복사',
      onClick: callbacks.onCopy,
    },
    {
      key: 'move',
      icon: <ScissorOutlined />,
      label: '이동',
      onClick: callbacks.onMove,
    },
    {
      key: 'rename',
      icon: <EditOutlined />,
      label: '이름 변경',
      onClick: () => callbacks.onRename(record),
    },
    { type: 'divider' },
    {
      key: 'delete',
      icon: <DeleteOutlined />,
      label: '삭제',
      danger: true,
      onClick: () => callbacks.onDelete(record),
    },
  ];
}

export function buildMultiItemMenu(
  count: number,
  callbacks: Pick<ContextMenuCallbacks, 'onBulkDownload' | 'onCopy' | 'onMove' | 'onBulkDelete'>
): MenuProps['items'] {
  return [
    {
      key: 'download',
      icon: <DownloadOutlined />,
      label: `다운로드 (${count}개)`,
      onClick: callbacks.onBulkDownload,
    },
    {
      key: 'copy',
      icon: <CopyOutlined />,
      label: `복사 (${count}개)`,
      onClick: callbacks.onCopy,
    },
    {
      key: 'move',
      icon: <ScissorOutlined />,
      label: `이동 (${count}개)`,
      onClick: callbacks.onMove,
    },
    { type: 'divider' },
    {
      key: 'delete',
      icon: <DeleteOutlined />,
      label: `삭제 (${count}개)`,
      danger: true,
      onClick: callbacks.onBulkDelete,
    },
  ];
}

export function buildEmptyAreaMenu(onCreateFolder: () => void): MenuProps['items'] {
  return [
    {
      key: 'create-folder',
      icon: <FolderOutlined />,
      label: '새 폴더 만들기',
      onClick: onCreateFolder,
    },
  ];
}

// Sort options
export const SORT_OPTIONS = [
  { value: 'name-ascend', label: '이름 ↑' },
  { value: 'name-descend', label: '이름 ↓' },
  { value: 'modTime-ascend', label: '수정일 ↑' },
  { value: 'modTime-descend', label: '수정일 ↓' },
  { value: 'size-ascend', label: '크기 ↑' },
  { value: 'size-descend', label: '크기 ↓' },
];
