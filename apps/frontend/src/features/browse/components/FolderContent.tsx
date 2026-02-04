
import React, { useEffect, useState } from 'react';
import { Table, Empty, Breadcrumb, Space as AntSpace, Button } from 'antd';
import { FolderFilled, FileOutlined, LeftOutlined } from '@ant-design/icons';
import { useBrowseApi } from '../hooks/useBrowseApi';
import type { FileNode } from '../types';
import type { ColumnsType } from 'antd/es/table';

interface FolderContentProps {
  selectedPath: string;
  onPathChange: (path: string) => void;
}

const formatSize = (bytes: number) => {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
};

const formatDate = (dateString: string) => {
  if (!dateString) return '-';
  const date = new Date(dateString);
  return date.toLocaleString();
};

const FolderContent: React.FC<FolderContentProps> = ({ selectedPath, onPathChange }) => {
  const [content, setContent] = useState<FileNode[]>([]);
  const { isLoading, fetchDirectoryContents } = useBrowseApi();

  useEffect(() => {
    if (selectedPath) {
      const loadContent = async () => {
        const contents = await fetchDirectoryContents(selectedPath);
        setContent(contents);
      };
      loadContent();
    }
  }, [selectedPath, fetchDirectoryContents]);

  const breadcrumbItems = selectedPath.split('/').filter(Boolean).reduce((acc: Array<{title: React.ReactNode; key: string}>, curr, idx, array) => {
    const path = '/' + array.slice(0, idx + 1).join('/');
    acc.push({
      title: <a onClick={() => onPathChange(path)}>{curr}</a>,
      key: path
    });
    return acc;
  }, [{ title: <a onClick={() => onPathChange('/')}>Root</a>, key: '/' }]);

  const columns: ColumnsType<FileNode> = [
    {
      title: '이름',
      dataIndex: 'name',
      key: 'name',
      render: (text, record) => (
        <AntSpace>
          {record.isDir ? <FolderFilled style={{ color: '#ffca28' }} /> : <FileOutlined />}
          <a onClick={() => record.isDir && onPathChange(record.path)}>{text}</a>
        </AntSpace>
      ),
      sorter: (a, b) => a.name.localeCompare(b.name),
    },
    {
      title: '수정일',
      dataIndex: 'modTime',
      key: 'modTime',
      width: 200,
      render: (date) => formatDate(date),
      sorter: (a, b) => new Date(a.modTime).getTime() - new Date(b.modTime).getTime(),
    },
    {
      title: '크기',
      dataIndex: 'size',
      key: 'size',
      width: 120,
      render: (size, record) => record.isDir ? '-' : formatSize(size),
      sorter: (a, b) => a.size - b.size,
    },
  ];

  if (!selectedPath) {
    return (
      <div style={{ height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <Empty description="왼쪽 트리나 스페이스에서 폴더를 선택하세요." />
      </div>
    );
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Breadcrumb items={breadcrumbItems} />
        {selectedPath !== '/' && (
          <Button 
            icon={<LeftOutlined />} 
            size="small" 
            onClick={() => {
              const parent = selectedPath.split('/').slice(0, -1).join('/') || '/';
              onPathChange(parent);
            }}
          >
            상위로
          </Button>
        )}
      </div>
      
      <Table
        dataSource={content}
        columns={columns}
        loading={isLoading}
        rowKey="path"
        pagination={false}
        onRow={(record) => ({
          onDoubleClick: () => record.isDir && onPathChange(record.path),
        })}
        locale={{ emptyText: '이 폴더는 비어 있습니다.' }}
      />
    </div>
  );
};

export default FolderContent;
