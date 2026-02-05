
import React, { useEffect, useState } from 'react';
import { Table, Empty, Breadcrumb, Space as AntSpace, Menu } from 'antd';
import type { MenuProps } from 'antd';
import { FolderFilled, FileOutlined, DownloadOutlined, DeleteOutlined, EditOutlined } from '@ant-design/icons';
import { useBrowseApi } from '../hooks/useBrowseApi';
import type { FileNode } from '../types';
import type { ColumnsType } from 'antd/es/table';
import type { Space } from '@/features/space/types';

interface FolderContentProps {
  selectedPath: string;
  selectedSpace?: Space;
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

const FolderContent: React.FC<FolderContentProps> = ({ selectedPath, selectedSpace, onPathChange }) => {
  const [content, setContent] = useState<FileNode[]>([]);
  const { isLoading, fetchDirectoryContents } = useBrowseApi();

  // 컨텍스트 메뉴 상태 관리
  const [contextMenu, setContextMenu] = useState<{
    visible: boolean;
    x: number;
    y: number;
    record?: FileNode;
  }>({ visible: false, x: 0, y: 0 });

  useEffect(() => {
    if (selectedPath) {
      const loadContent = async () => {
        const contents = await fetchDirectoryContents(selectedPath);
        setContent(contents);
      };
      loadContent();
    }
  }, [selectedPath, fetchDirectoryContents]);

  // 컨텍스트 메뉴 닫기
  useEffect(() => {
    const handleClick = () => {
      setContextMenu({ visible: false, x: 0, y: 0 });
    };

    if (contextMenu.visible) {
      document.addEventListener('click', handleClick);
      return () => document.removeEventListener('click', handleClick);
    }
  }, [contextMenu.visible]);

  // Space 상대 경로로 Breadcrumb 생성
  const breadcrumbItems = (() => {
    if (!selectedPath) return [];

    // Space가 선택된 경우 상대 경로로 표시
    if (selectedSpace && selectedPath.startsWith(selectedSpace.space_path)) {
      const relativePath = selectedPath.slice(selectedSpace.space_path.length);
      const segments = relativePath.split('/').filter(Boolean);

      const items: Array<{title: React.ReactNode; key: string}> = [
        {
          title: <a onClick={() => onPathChange(selectedSpace.space_path)}>{selectedSpace.space_name}</a>,
          key: selectedSpace.space_path
        }
      ];

      segments.forEach((curr, idx) => {
        const path = selectedSpace.space_path + '/' + segments.slice(0, idx + 1).join('/');
        items.push({
          title: <a onClick={() => onPathChange(path)}>{curr}</a>,
          key: path
        });
      });

      return items;
    }

    // Space가 없는 경우 절대 경로로 표시 (기존 로직)
    return selectedPath.split('/').filter(Boolean).reduce((acc: Array<{title: React.ReactNode; key: string}>, curr, idx, array) => {
      const path = '/' + array.slice(0, idx + 1).join('/');
      acc.push({
        title: <a onClick={() => onPathChange(path)}>{curr}</a>,
        key: path
      });
      return acc;
    }, [{ title: <a onClick={() => onPathChange('/')}>Root</a>, key: '/' }]);
  })();

  const columns: ColumnsType<FileNode> = [
    {
      title: '이름',
      dataIndex: 'name',
      key: 'name',
      render: (text: string, record: FileNode) => (
        <AntSpace>
          {record.isDir ? <FolderFilled style={{ color: '#ffca28' }} /> : <FileOutlined />}
          {record.isDir ? (
            <a onClick={() => onPathChange(record.path)}>{text}</a>
          ) : (
            <a href={`/api/browse/download?path=${encodeURIComponent(record.path)}`} download>{text}</a>
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
      render: (size: number, record: FileNode) => record.isDir ? '-' : formatSize(size),
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

  // 우클릭 핸들러
  const handleContextMenu = (e: React.MouseEvent, record: FileNode) => {
    e.preventDefault();
    setContextMenu({
      visible: true,
      x: e.clientX,
      y: e.clientY,
      record,
    });
  };

  // 메뉴 항목 생성
  const menuItems: MenuProps['items'] = contextMenu.record ? [
    ...(!contextMenu.record.isDir ? [{
      key: 'download',
      icon: <DownloadOutlined />,
      label: '다운로드',
      onClick: () => {
        if (contextMenu.record) {
          window.location.href = `/api/browse/download?path=${encodeURIComponent(contextMenu.record.path)}`;
        }
      },
    }] : []),
    {
      key: 'rename',
      icon: <EditOutlined />,
      label: '이름 변경',
      onClick: () => {
        // TODO: 이름 변경 기능 구현
        console.log('이름 변경:', contextMenu.record);
      },
    },
    {
      type: 'divider',
    },
    {
      key: 'delete',
      icon: <DeleteOutlined />,
      label: '삭제',
      danger: true,
      onClick: () => {
        // TODO: 삭제 기능 구현
        console.log('삭제:', contextMenu.record);
      },
    },
  ] : [];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
      <Breadcrumb items={breadcrumbItems} />

      <Table
        dataSource={content}
        columns={columns}
        loading={isLoading}
        rowKey="path"
        pagination={false}
        onRow={(record: FileNode) => ({
          onDoubleClick: () => record.isDir && onPathChange(record.path),
          onContextMenu: (e) => handleContextMenu(e, record),
        })}
        locale={{ emptyText: '이 폴더는 비어 있습니다.' }}
      />

      {contextMenu.visible && (
        <Menu
          items={menuItems}
          style={{
            position: 'fixed',
            left: contextMenu.x,
            top: contextMenu.y,
            zIndex: 1000,
          }}
        />
      )}
    </div>
  );
};

export default FolderContent;
