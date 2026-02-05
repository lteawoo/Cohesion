
import React, { useEffect, useState, useRef } from 'react';
import { Table, Empty, Breadcrumb, Space as AntSpace, Menu, Modal, Input, message, Upload, Button, Card, Row, Col } from 'antd';
import type { MenuProps, UploadProps } from 'antd';
import { FolderFilled, FileOutlined, DownloadOutlined, DeleteOutlined, EditOutlined, InboxOutlined, UnorderedListOutlined, AppstoreOutlined, UploadOutlined } from '@ant-design/icons';
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
  const fileInputRef = useRef<HTMLInputElement>(null);

  // 뷰 모드 상태 (테이블/그리드)
  const [viewMode, setViewMode] = useState<'table' | 'grid'>('table');

  // 드래그 상태 관리
  const [isDragging, setIsDragging] = useState(false);

  // 컨텍스트 메뉴 상태 관리
  const [contextMenu, setContextMenu] = useState<{
    visible: boolean;
    x: number;
    y: number;
    record?: FileNode;
  }>({ visible: false, x: 0, y: 0 });

  // 이름 변경 모달 상태
  const [renameModal, setRenameModal] = useState<{
    visible: boolean;
    record?: FileNode;
    newName: string;
  }>({ visible: false, newName: '' });

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

  // 이름 변경 처리
  const handleRename = async () => {
    if (!renameModal.record || !renameModal.newName.trim()) {
      message.error('새 이름을 입력하세요');
      return;
    }

    try {
      const response = await fetch('/api/browse/rename', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          oldPath: renameModal.record.path,
          newName: renameModal.newName.trim(),
        }),
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.message || 'Failed to rename');
      }

      message.success('이름이 변경되었습니다');
      setRenameModal({ visible: false, newName: '' });

      // 목록 새로고침
      const contents = await fetchDirectoryContents(selectedPath);
      setContent(contents);
    } catch (error) {
      message.error(error instanceof Error ? error.message : '이름 변경 실패');
    }
  };

  // 삭제 처리
  const handleDelete = async (record: FileNode) => {
    Modal.confirm({
      title: '삭제 확인',
      content: `"${record.name}"을(를) 삭제하시겠습니까?${record.isDir ? ' (폴더 내 모든 파일도 삭제됩니다)' : ''}`,
      okText: '삭제',
      okType: 'danger',
      cancelText: '취소',
      onOk: async () => {
        try {
          const response = await fetch('/api/browse/delete', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: record.path }),
          });

          if (!response.ok) {
            const error = await response.json();
            throw new Error(error.message || 'Failed to delete');
          }

          message.success('삭제되었습니다');

          // 목록 새로고침
          const contents = await fetchDirectoryContents(selectedPath);
          setContent(contents);
        } catch (error) {
          message.error(error instanceof Error ? error.message : '삭제 실패');
        }
      },
    });
  };

  // 파일 업로드 실행 함수
  const performUpload = async (file: File, overwrite: boolean = false): Promise<void> => {
    const formData = new FormData();
    formData.append('file', file);
    formData.append('targetPath', selectedPath);
    if (overwrite) {
      formData.append('overwrite', 'true');
    }

    const response = await fetch('/api/browse/upload', {
      method: 'POST',
      body: formData,
    });

    if (!response.ok) {
      const error = await response.json();
      throw { status: response.status, message: error.message || 'Failed to upload' };
    }

    const result = await response.json();
    message.success(`"${result.filename}" 업로드 완료`);

    // 목록 새로고침
    const contents = await fetchDirectoryContents(selectedPath);
    setContent(contents);
  };

  // 파일 업로드 처리 (중복 확인 포함)
  const handleFileUpload = async (file: File) => {
    try {
      await performUpload(file, false);
    } catch (error: any) {
      // 파일 중복 에러 (409)
      if (error.status === 409) {
        Modal.confirm({
          title: '파일 덮어쓰기',
          content: `"${file.name}" 파일이 이미 존재합니다. 덮어쓰시겠습니까?`,
          okText: '덮어쓰기',
          okType: 'danger',
          cancelText: '취소',
          onOk: async () => {
            try {
              await performUpload(file, true);
            } catch (retryError: any) {
              message.error(retryError.message || '업로드 실패');
            }
          },
        });
      } else {
        message.error(error.message || '업로드 실패');
      }
    }
  };

  // 드래그 이벤트 핸들러
  const handleDragEnter = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(true);
  };

  const handleDragLeave = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    // 자식 요소로 이동할 때는 무시
    if (e.currentTarget === e.target) {
      setIsDragging(false);
    }
  };

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
  };

  const handleDrop = async (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(false);

    const files = Array.from(e.dataTransfer.files);
    if (files.length > 0) {
      // 첫 번째 파일만 업로드
      await handleFileUpload(files[0]);
    }
  };

  // 파일 선택 핸들러
  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files;
    if (files && files.length > 0) {
      handleFileUpload(files[0]);
      // input 초기화
      e.target.value = '';
    }
  };

  // 업로드 버튼 클릭 핸들러
  const handleUploadClick = () => {
    fileInputRef.current?.click();
  };

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
        if (contextMenu.record) {
          setRenameModal({
            visible: true,
            record: contextMenu.record,
            newName: contextMenu.record.name,
          });
        }
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
        if (contextMenu.record) {
          handleDelete(contextMenu.record);
        }
      },
    },
  ] : [];

  return (
    <div
      style={{ display: 'flex', flexDirection: 'column', gap: '16px', position: 'relative', height: '100%' }}
      onDragEnter={handleDragEnter}
      onDragLeave={handleDragLeave}
      onDragOver={handleDragOver}
      onDrop={handleDrop}
    >
      <input
        ref={fileInputRef}
        type="file"
        style={{ display: 'none' }}
        onChange={handleFileSelect}
      />

      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Breadcrumb items={breadcrumbItems} />
        <AntSpace>
          <Button
            icon={<UploadOutlined />}
            onClick={handleUploadClick}
          >
            업로드
          </Button>
          <Button.Group>
            <Button
              icon={<UnorderedListOutlined />}
              onClick={() => setViewMode('table')}
              type={viewMode === 'table' ? 'primary' : 'default'}
            />
            <Button
              icon={<AppstoreOutlined />}
              onClick={() => setViewMode('grid')}
              type={viewMode === 'grid' ? 'primary' : 'default'}
            />
          </Button.Group>
        </AntSpace>
      </div>

      {viewMode === 'table' ? (
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
      ) : (
        <Row gutter={[16, 16]}>
          {content.length === 0 && !isLoading ? (
            <Col span={24}>
              <Empty description="이 폴더는 비어 있습니다." />
            </Col>
          ) : (
            content.map((item) => (
              <Col key={item.path} xs={12} sm={8} md={6} lg={4} xl={3}>
                <Card
                  hoverable
                  onDoubleClick={() => item.isDir && onPathChange(item.path)}
                  onContextMenu={(e) => handleContextMenu(e, item)}
                  style={{ textAlign: 'center', cursor: 'pointer' }}
                  bodyStyle={{ padding: '16px 8px' }}
                >
                  <div style={{ fontSize: '48px', marginBottom: '8px' }}>
                    {item.isDir ? (
                      <FolderFilled style={{ color: '#ffca28' }} />
                    ) : (
                      <FileOutlined style={{ color: '#8c8c8c' }} />
                    )}
                  </div>
                  <div
                    style={{
                      fontSize: '12px',
                      wordBreak: 'break-word',
                      marginBottom: '4px',
                    }}
                  >
                    {item.name}
                  </div>
                  {!item.isDir && (
                    <div style={{ fontSize: '11px', color: '#8c8c8c' }}>
                      {formatSize(item.size)}
                    </div>
                  )}
                </Card>
              </Col>
            ))
          )}
        </Row>
      )}

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

      <Modal
        title="이름 변경"
        open={renameModal.visible}
        onOk={handleRename}
        onCancel={() => setRenameModal({ visible: false, newName: '' })}
        okText="변경"
        cancelText="취소"
      >
        <Input
          placeholder="새 이름"
          value={renameModal.newName}
          onChange={(e) => setRenameModal({ ...renameModal, newName: e.target.value })}
          onPressEnter={handleRename}
        />
      </Modal>

      {isDragging && (
        <div
          style={{
            position: 'absolute',
            top: 0,
            left: 0,
            right: 0,
            bottom: 0,
            backgroundColor: 'rgba(24, 144, 255, 0.1)',
            border: '2px dashed #1890ff',
            borderRadius: '8px',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 999,
            pointerEvents: 'none',
          }}
        >
          <div style={{ textAlign: 'center' }}>
            <InboxOutlined style={{ fontSize: '64px', color: '#1890ff', marginBottom: '16px' }} />
            <div style={{ fontSize: '20px', fontWeight: 'bold', color: '#1890ff' }}>
              파일을 놓아 업로드
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default FolderContent;
