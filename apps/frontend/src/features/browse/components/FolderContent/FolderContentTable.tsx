import React from 'react';
import { Table } from 'antd';
import type { TableColumnsType } from 'antd';
import type { FileNode, SortConfig } from '../../types';

interface FolderContentTableProps {
  dataSource: FileNode[];
  columns: TableColumnsType<FileNode>;
  loading: boolean;
  selectedItems: Set<string>;
  dragOverFolder: string | null;
  onSelectionChange: (items: Set<string>) => void;
  onItemClick: (e: React.MouseEvent<HTMLElement>, record: FileNode, index: number) => void;
  onItemDoubleClick: (path: string) => void;
  onContextMenu: (e: React.MouseEvent<HTMLElement>, record: FileNode) => void;
  onItemDragStart: (e: React.DragEvent<HTMLElement>, record: FileNode) => void;
  onItemDragEnd: (e: React.DragEvent<HTMLElement>) => void;
  onFolderDragOver: (e: React.DragEvent<HTMLElement>, record: FileNode) => void;
  onFolderDragLeave: (e: React.DragEvent<HTMLElement>) => void;
  onFolderDrop: (e: React.DragEvent<HTMLElement>, record: FileNode) => void;
  sortConfig: SortConfig;
  onSortChange: (config: SortConfig) => void;
}

const FolderContentTable: React.FC<FolderContentTableProps> = ({
  dataSource,
  columns,
  loading,
  selectedItems,
  dragOverFolder,
  onSelectionChange,
  onItemClick,
  onItemDoubleClick,
  onContextMenu,
  onItemDragStart,
  onItemDragEnd,
  onFolderDragOver,
  onFolderDragLeave,
  onFolderDrop,
  onSortChange,
}) => {
  return (
    <Table
      dataSource={dataSource}
      columns={columns}
      loading={loading}
      rowKey="path"
      pagination={false}
      rowSelection={{
        type: 'checkbox',
        selectedRowKeys: Array.from(selectedItems),
        onChange: (keys) => onSelectionChange(new Set(keys as string[])),
      }}
      onChange={(_, __, sorter) => {
        if (sorter && !Array.isArray(sorter)) {
          const field = sorter.field as 'name' | 'modTime' | 'size';
          const order = sorter.order as 'ascend' | 'descend' | undefined;
          if (field && order) {
            onSortChange({ sortBy: field, sortOrder: order });
          }
        }
      }}
      onRow={(record: FileNode, index?: number) => ({
        onClick: (e: React.MouseEvent<HTMLElement>) => onItemClick(e, record, index ?? 0),
        onDoubleClick: () => record.isDir && onItemDoubleClick(record.path),
        onContextMenu: (e: React.MouseEvent<HTMLElement>) => onContextMenu(e, record),
        draggable: true,
        onDragStart: (e: React.DragEvent<HTMLElement>) => onItemDragStart(e, record),
        onDragEnd: (e: React.DragEvent<HTMLElement>) => onItemDragEnd(e),
        onDragOver: (e: React.DragEvent<HTMLElement>) => record.isDir && onFolderDragOver(e, record),
        onDragLeave: (e: React.DragEvent<HTMLElement>) => record.isDir && onFolderDragLeave(e),
        onDrop: (e: React.DragEvent<HTMLElement>) => record.isDir && onFolderDrop(e, record),
        style: {
          backgroundColor: dragOverFolder === record.path ? 'rgba(24, 144, 255, 0.1)' : undefined,
          userSelect: 'none',
        } as React.CSSProperties,
      })}
      locale={{ emptyText: '이 폴더는 비어 있습니다.' }}
    />
  );
};

export default FolderContentTable;
