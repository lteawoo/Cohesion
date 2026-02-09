
import React, { useEffect, useState, useRef, useMemo } from 'react';
import { Table, Empty, Breadcrumb, Space as AntSpace, Modal, Input, message, Button, Card, Row, Col, Select } from 'antd';
import type { MenuProps } from 'antd';
import { useContextMenuStore } from '@/stores/contextMenuStore';
import { useBrowseStore } from '@/stores/browseStore';
import { FolderFilled, FolderOutlined, FileOutlined, DownloadOutlined, DeleteOutlined, EditOutlined, InboxOutlined, UnorderedListOutlined, AppstoreOutlined, UploadOutlined, CopyOutlined, ScissorOutlined } from '@ant-design/icons';
import type { FileNode } from '../types';
import type { ColumnsType } from 'antd/es/table';
import DestinationPickerModal from './DestinationPickerModal';

interface FolderContentProps {}

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

const FolderContent: React.FC<FolderContentProps> = () => {
  const selectedPath = useBrowseStore((state) => state.selectedPath);
  const selectedSpace = useBrowseStore((state) => state.selectedSpace);
  const content = useBrowseStore((state) => state.content);
  const isLoading = useBrowseStore((state) => state.isLoading);
  const fetchDirectoryContents = useBrowseStore((state) => state.fetchDirectoryContents);
  const setPath = useBrowseStore((state) => state.setPath);

  console.log('[FolderContent] Render - selectedPath:', selectedPath);

  const fileInputRef = useRef<HTMLInputElement>(null);
  const openContextMenu = useContextMenuStore((state) => state.openContextMenu);

  // 뷰 모드 상태 (테이블/그리드)
  const [viewMode, setViewMode] = useState<'table' | 'grid'>('grid');

  // 정렬 상태 관리
  const [sortConfig, setSortConfig] = useState<{
    sortBy: 'name' | 'modTime' | 'size';
    sortOrder: 'ascend' | 'descend';
  }>({
    sortBy: 'name',
    sortOrder: 'ascend',
  });

  // 다중 선택 상태 관리
  const [selectedItems, setSelectedItems] = useState<Set<string>>(new Set());
  const [lastSelectedIndex, setLastSelectedIndex] = useState<number>(-1);

  // 드래그 상태 관리
  const [isDragging, setIsDragging] = useState(false);
  const [dragOverFolder, setDragOverFolder] = useState<string | null>(null); // 드롭 가능한 폴더 하이라이트

  // 이름 변경 모달 상태
  const [renameModal, setRenameModal] = useState<{
    visible: boolean;
    record?: FileNode;
    newName: string;
  }>({ visible: false, newName: '' });

  // 새 폴더 만들기 모달 상태
  const [createFolderModal, setCreateFolderModal] = useState<{
    visible: boolean;
    folderName: string;
  }>({ visible: false, folderName: '' });

  // 이동/복사 모달 상태
  const [destinationModal, setDestinationModal] = useState<{
    visible: boolean;
    mode: 'move' | 'copy';
  }>({ visible: false, mode: 'move' });

  useEffect(() => {
    console.log('[FolderContent] useEffect triggered - selectedPath:', selectedPath);
    if (selectedPath) {
      console.log('[FolderContent] Fetching contents for:', selectedPath);
      fetchDirectoryContents(selectedPath);
    }
    // 경로 변경 시 선택 해제
    setSelectedItems(new Set());
    setLastSelectedIndex(-1);
  }, [selectedPath, fetchDirectoryContents]);

  // 정렬된 콘텐츠 (폴더 우선 + sortConfig)
  const sortedContent = useMemo(() => {
    if (!Array.isArray(content)) {
      return [];
    }

    const sorted = [...content].sort((a, b) => {
      // 1. 폴더 우선 정렬
      if (a.isDir !== b.isDir) {
        return a.isDir ? -1 : 1;
      }

      // 2. sortBy에 따른 정렬
      let result = 0;
      if (sortConfig.sortBy === 'name') {
        result = a.name.localeCompare(b.name);
      } else if (sortConfig.sortBy === 'modTime') {
        result = new Date(a.modTime).getTime() - new Date(b.modTime).getTime();
      } else if (sortConfig.sortBy === 'size') {
        result = a.size - b.size;
      }

      // 3. sortOrder 적용
      return sortConfig.sortOrder === 'ascend' ? result : -result;
    });

    return sorted;
  }, [content, sortConfig]);

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
      await fetchDirectoryContents(selectedPath);
    } catch (error) {
      message.error(error instanceof Error ? error.message : '이름 변경 실패');
    }
  };

  // 새 폴더 만들기 처리
  const handleCreateFolder = async () => {
    if (!createFolderModal.folderName.trim()) {
      message.error('폴더 이름을 입력하세요');
      return;
    }

    try {
      const response = await fetch('/api/browse/create-folder', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          parentPath: selectedPath,
          folderName: createFolderModal.folderName.trim(),
        }),
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.message || 'Failed to create folder');
      }

      message.success('폴더가 생성되었습니다');
      setCreateFolderModal({ visible: false, folderName: '' });

      // 디렉토리 목록 새로고침
      await fetchDirectoryContents(selectedPath);
    } catch (error) {
      message.error(error instanceof Error ? error.message : '폴더 생성 실패');
    }
  };

  // 다중 다운로드 처리
  const handleBulkDownload = async () => {
    if (selectedItems.size === 0) return;

    try {
      const response = await fetch('/api/browse/download-multiple', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ paths: Array.from(selectedItems) }),
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.message || 'Failed to download');
      }

      // Blob 다운로드 처리
      const blob = await response.blob();
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `download-${Date.now()}.zip`;
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(url);
      document.body.removeChild(a);

      message.success('다운로드가 시작되었습니다');
      setSelectedItems(new Set());
      setLastSelectedIndex(-1);
    } catch (error) {
      message.error(error instanceof Error ? error.message : '다운로드 실패');
    }
  };

  // 다중 삭제 처리
  const handleBulkDelete = async () => {
    if (selectedItems.size === 0) return;

    Modal.confirm({
      title: '삭제 확인',
      content: `선택한 ${selectedItems.size}개 항목을 삭제하시겠습니까?`,
      okText: '삭제',
      okType: 'danger',
      cancelText: '취소',
      onOk: async () => {
        try {
          const response = await fetch('/api/browse/delete-multiple', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ paths: Array.from(selectedItems) }),
          });

          if (!response.ok) {
            const error = await response.json();
            throw new Error(error.message || 'Failed to delete');
          }

          const result = await response.json();
          const succeededCount = result.succeeded?.length || 0;
          const failedCount = result.failed?.length || 0;

          if (failedCount > 0) {
            message.warning(`${succeededCount}개 삭제 완료, ${failedCount}개 실패`);
          } else {
            message.success(`${succeededCount}개 항목이 삭제되었습니다`);
          }

          setSelectedItems(new Set());
          setLastSelectedIndex(-1);

          // 목록 새로고침
          await fetchDirectoryContents(selectedPath);
        } catch (error) {
          message.error(error instanceof Error ? error.message : '삭제 실패');
        }
      },
    });
  };

  // 이동 처리
  const handleMove = async (destination: string) => {
    if (selectedItems.size === 0) return;

    try {
      const response = await fetch('/api/browse/move', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          sources: Array.from(selectedItems),
          destination,
        }),
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.message || 'Failed to move');
      }

      const result = await response.json();
      const succeededCount = result.succeeded?.length || 0;
      const failedCount = result.failed?.length || 0;

      if (failedCount > 0) {
        message.warning(`${succeededCount}개 이동 완료, ${failedCount}개 실패`);
      } else {
        message.success(`${succeededCount}개 항목이 이동되었습니다`);
      }

      setSelectedItems(new Set());
      setLastSelectedIndex(-1);
      setDestinationModal({ visible: false, mode: 'move' });

      // 목록 새로고침
      await fetchDirectoryContents(selectedPath);
    } catch (error) {
      message.error(error instanceof Error ? error.message : '이동 실패');
    }
  };

  // 복사 처리
  const handleCopy = async (destination: string) => {
    if (selectedItems.size === 0) return;

    try {
      const response = await fetch('/api/browse/copy', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          sources: Array.from(selectedItems),
          destination,
        }),
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.message || 'Failed to copy');
      }

      const result = await response.json();
      const succeededCount = result.succeeded?.length || 0;
      const failedCount = result.failed?.length || 0;

      if (failedCount > 0) {
        message.warning(`${succeededCount}개 복사 완료, ${failedCount}개 실패`);
      } else {
        message.success(`${succeededCount}개 항목이 복사되었습니다`);
      }

      setSelectedItems(new Set());
      setLastSelectedIndex(-1);
      setDestinationModal({ visible: false, mode: 'copy' });

      // 목록 새로고침
      await fetchDirectoryContents(selectedPath);
    } catch (error) {
      message.error(error instanceof Error ? error.message : '복사 실패');
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
          await fetchDirectoryContents(selectedPath);
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
    await fetchDirectoryContents(selectedPath);
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

  // 아이템 드래그 시작 핸들러
  const handleItemDragStart = (e: React.DragEvent, record: FileNode) => {
    // 드래그 시작 시 선택 처리
    if (!selectedItems.has(record.path)) {
      // 선택되지 않은 항목을 드래그하면 해당 항목만 선택
      setSelectedItems(new Set([record.path]));
    }

    // dataTransfer에 경로 목록 저장
    const dragData = {
      type: 'cohesion-internal',
      paths: selectedItems.has(record.path)
        ? Array.from(selectedItems)
        : [record.path]
    };
    e.dataTransfer.setData('application/json', JSON.stringify(dragData));
    e.dataTransfer.effectAllowed = 'move';
  };

  // 아이템 드래그 종료 핸들러
  const handleItemDragEnd = (_e: React.DragEvent) => {
    setDragOverFolder(null);
  };

  // 폴더 위 드래그 오버 핸들러
  const handleFolderDragOver = (e: React.DragEvent, folder: FileNode) => {
    if (!folder.isDir) return;

    e.preventDefault();
    e.stopPropagation();

    // 외부 파일인지 내부 이동인지 확인
    const hasFiles = e.dataTransfer.types.includes('Files');
    if (hasFiles) {
      // 외부 파일은 폴더 드롭 불가
      return;
    }

    e.dataTransfer.dropEffect = 'move';
    setDragOverFolder(folder.path);
  };

  // 폴더 위 드래그 떠남 핸들러
  const handleFolderDragLeave = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setDragOverFolder(null);
  };

  // 폴더 위 드롭 핸들러
  const handleFolderDrop = async (e: React.DragEvent, folder: FileNode) => {
    if (!folder.isDir) return;

    e.preventDefault();
    e.stopPropagation();
    setDragOverFolder(null);

    try {
      const dataText = e.dataTransfer.getData('application/json');
      if (dataText) {
        const dragData = JSON.parse(dataText);
        if (dragData.type === 'cohesion-internal') {
          const sourcePaths = dragData.paths as string[];

          // 자기 자신으로 이동 방지
          if (sourcePaths.includes(folder.path)) {
            message.warning('자기 자신으로 이동할 수 없습니다');
            return;
          }

          // 해당 폴더로 이동
          await handleMove(folder.path);
        }
      }
    } catch (error) {
      console.error('Drop error:', error);
    }
  };

  // 드래그 이벤트 핸들러 (외부 파일 업로드용)
  const handleDragEnter = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();

    // 외부 파일만 isDragging 상태 활성화
    const hasFiles = e.dataTransfer.types.includes('Files');
    if (hasFiles) {
      setIsDragging(true);
    }
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

    // 1. 외부 파일 업로드 체크
    const files = Array.from(e.dataTransfer.files);
    if (files.length > 0) {
      // 첫 번째 파일만 업로드
      await handleFileUpload(files[0]);
      return;
    }

    // 2. 내부 파일/폴더 이동 (빈 영역에 드롭 시 현재 폴더에 이동)
    try {
      const dataText = e.dataTransfer.getData('application/json');
      if (dataText) {
        const dragData = JSON.parse(dataText);
        if (dragData.type === 'cohesion-internal') {
          // 같은 폴더에 드롭하면 무시
          const sourcePaths = dragData.paths as string[];
          const allInSameFolder = sourcePaths.every(p => {
            const parentPath = p.substring(0, p.lastIndexOf('/'));
            return parentPath === selectedPath;
          });
          if (!allInSameFolder) {
            // 현재 폴더로 이동
            await handleMove(selectedPath);
          }
        }
      }
    } catch (error) {
      // JSON 파싱 실패 시 무시
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

  // 아이템 선택 핸들러 (Table/Grid 모두 사용)
  const handleItemClick = (e: React.MouseEvent, record: FileNode, index: number) => {
    // Ctrl/Cmd + 클릭: 토글
    if (e.ctrlKey || e.metaKey) {
      e.preventDefault();
      const newSelected = new Set(selectedItems);
      if (newSelected.has(record.path)) {
        newSelected.delete(record.path);
      } else {
        newSelected.add(record.path);
      }
      setSelectedItems(newSelected);
      setLastSelectedIndex(index);
    }
    // Shift + 클릭: 범위 선택
    else if (e.shiftKey && lastSelectedIndex >= 0) {
      e.preventDefault();
      const start = Math.min(lastSelectedIndex, index);
      const end = Math.max(lastSelectedIndex, index);
      const newSelected = new Set(selectedItems);
      for (let i = start; i <= end; i++) {
        if (sortedContent[i]) {
          newSelected.add(sortedContent[i].path);
        }
      }
      setSelectedItems(newSelected);
    }
    // 일반 클릭: 단일 선택
    else {
      setSelectedItems(new Set([record.path]));
      setLastSelectedIndex(index);
    }
  };

  // 빈 영역 클릭 시 선택 해제
  const handleContainerClick = (e: React.MouseEvent) => {
    const target = e.target as HTMLElement;
    // 카드나 테이블 행을 클릭하지 않은 경우
    const isCard = target.closest('.ant-card');
    const isTableRow = target.closest('tr');
    if (!isCard && !isTableRow) {
      setSelectedItems(new Set());
      setLastSelectedIndex(-1);
    }
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
          title: <a onClick={() => setPath(selectedSpace.space_path)}>{selectedSpace.space_name}</a>,
          key: selectedSpace.space_path
        }
      ];

      segments.forEach((curr, idx) => {
        const path = selectedSpace.space_path + '/' + segments.slice(0, idx + 1).join('/');
        items.push({
          title: <a onClick={() => setPath(path)}>{curr}</a>,
          key: path
        });
      });

      return items;
    }

    // Space가 없는 경우 절대 경로로 표시 (기존 로직)
    return selectedPath.split('/').filter(Boolean).reduce((acc: Array<{title: React.ReactNode; key: string}>, curr, idx, array) => {
      const path = '/' + array.slice(0, idx + 1).join('/');
      acc.push({
        title: <a onClick={() => setPath(path)}>{curr}</a>,
        key: path
      });
      return acc;
    }, [{ title: <a onClick={() => setPath('/')}>Root</a>, key: '/' }]);
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
            <a onClick={() => setPath(record.path)}>{text}</a>
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

    // 우클릭한 항목이 선택된 항목에 포함되어 있고, 다중 선택 상태인 경우
    const isSelectedItem = selectedItems.has(record.path);
    const isMultiSelect = selectedItems.size > 1;

    if (isSelectedItem && isMultiSelect) {
      // 다중 선택 메뉴
      const menuItems: MenuProps['items'] = [
        {
          key: 'download',
          icon: <DownloadOutlined />,
          label: `다운로드 (${selectedItems.size}개)`,
          onClick: handleBulkDownload,
        },
        {
          key: 'copy',
          icon: <CopyOutlined />,
          label: `복사 (${selectedItems.size}개)`,
          onClick: () => setDestinationModal({ visible: true, mode: 'copy' }),
        },
        {
          key: 'move',
          icon: <ScissorOutlined />,
          label: `이동 (${selectedItems.size}개)`,
          onClick: () => setDestinationModal({ visible: true, mode: 'move' }),
        },
        {
          type: 'divider',
        },
        {
          key: 'delete',
          icon: <DeleteOutlined />,
          label: `삭제 (${selectedItems.size}개)`,
          danger: true,
          onClick: handleBulkDelete,
        },
      ];

      openContextMenu(e.clientX, e.clientY, menuItems);
    } else if (isSelectedItem && selectedItems.size === 1) {
      // 단일 선택 메뉴 (선택된 항목 우클릭)
      const menuItems: MenuProps['items'] = [
        {
          key: 'download',
          icon: <DownloadOutlined />,
          label: record.isDir ? '폴더 다운로드 (ZIP)' : '다운로드',
          onClick: () => {
            window.location.href = `/api/browse/download?path=${encodeURIComponent(record.path)}`;
          },
        },
        {
          key: 'copy',
          icon: <CopyOutlined />,
          label: '복사',
          onClick: () => setDestinationModal({ visible: true, mode: 'copy' }),
        },
        {
          key: 'move',
          icon: <ScissorOutlined />,
          label: '이동',
          onClick: () => setDestinationModal({ visible: true, mode: 'move' }),
        },
        {
          key: 'rename',
          icon: <EditOutlined />,
          label: '이름 변경',
          onClick: () => {
            setRenameModal({
              visible: true,
              record,
              newName: record.name,
            });
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
            handleDelete(record);
          },
        },
      ];

      openContextMenu(e.clientX, e.clientY, menuItems);
    } else {
      // 선택되지 않은 항목 우클릭 - 해당 항목만 선택하고 단일 메뉴 표시
      setSelectedItems(new Set([record.path]));
      setLastSelectedIndex(sortedContent.findIndex(item => item.path === record.path));

      const menuItems: MenuProps['items'] = [
        {
          key: 'download',
          icon: <DownloadOutlined />,
          label: record.isDir ? '폴더 다운로드 (ZIP)' : '다운로드',
          onClick: () => {
            window.location.href = `/api/browse/download?path=${encodeURIComponent(record.path)}`;
          },
        },
        {
          key: 'copy',
          icon: <CopyOutlined />,
          label: '복사',
          onClick: () => setDestinationModal({ visible: true, mode: 'copy' }),
        },
        {
          key: 'move',
          icon: <ScissorOutlined />,
          label: '이동',
          onClick: () => setDestinationModal({ visible: true, mode: 'move' }),
        },
        {
          key: 'rename',
          icon: <EditOutlined />,
          label: '이름 변경',
          onClick: () => {
            setRenameModal({
              visible: true,
              record,
              newName: record.name,
            });
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
            handleDelete(record);
          },
        },
      ];

      openContextMenu(e.clientX, e.clientY, menuItems);
    }
  };

  // 빈 영역 우클릭 핸들러
  const handleEmptyAreaContextMenu = (e: React.MouseEvent) => {
    e.preventDefault();
    // 빈 영역 클릭 감지: 카드나 테이블 행이 아닌 경우
    const target = e.target as HTMLElement;
    const isCard = target.closest('.ant-card');
    const isTableRow = target.closest('tr');

    // 카드나 테이블 행을 클릭하지 않은 경우 빈 영역 메뉴 표시
    if (!isCard && !isTableRow) {
      const emptyAreaMenuItems: MenuProps['items'] = [
        {
          key: 'create-folder',
          icon: <FolderOutlined />,
          label: '새 폴더 만들기',
          onClick: () => {
            setCreateFolderModal({ visible: true, folderName: '' });
          },
        },
      ];

      openContextMenu(e.clientX, e.clientY, emptyAreaMenuItems);
    }
  };

  return (
    <div
      style={{ display: 'flex', flexDirection: 'column', gap: '16px', position: 'relative', height: '100%' }}
      onDragEnter={handleDragEnter}
      onDragLeave={handleDragLeave}
      onDragOver={handleDragOver}
      onDrop={handleDrop}
      onContextMenu={handleEmptyAreaContextMenu}
      onClick={handleContainerClick}
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
          {viewMode === 'grid' && (
            <Select
              style={{ width: 160 }}
              value={`${sortConfig.sortBy}-${sortConfig.sortOrder}`}
              onChange={(value: string) => {
                const [sortBy, sortOrder] = value.split('-') as ['name' | 'modTime' | 'size', 'ascend' | 'descend'];
                setSortConfig({ sortBy, sortOrder });
              }}
              options={[
                { value: 'name-ascend', label: '이름 ↑' },
                { value: 'name-descend', label: '이름 ↓' },
                { value: 'modTime-ascend', label: '수정일 ↑' },
                { value: 'modTime-descend', label: '수정일 ↓' },
                { value: 'size-ascend', label: '크기 ↑' },
                { value: 'size-descend', label: '크기 ↓' },
              ]}
            />
          )}
          <AntSpace.Compact>
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
          </AntSpace.Compact>
        </AntSpace>
      </div>

      {/* 선택 시 나타나는 툴바 */}
      {selectedItems.size > 0 && (
        <div
          style={{
            padding: '8px 16px',
            backgroundColor: 'rgba(24, 144, 255, 0.1)',
            borderRadius: '4px',
            display: 'flex',
            alignItems: 'center',
            gap: '16px',
          }}
        >
          <span style={{ fontWeight: 'bold', color: '#1890ff' }}>
            ✓ {selectedItems.size}개 선택됨
          </span>
          <AntSpace size="small">
            <Button
              size="small"
              icon={<DownloadOutlined />}
              onClick={handleBulkDownload}
            >
              다운로드
            </Button>
            <Button
              size="small"
              icon={<CopyOutlined />}
              onClick={() => setDestinationModal({ visible: true, mode: 'copy' })}
            >
              복사
            </Button>
            <Button
              size="small"
              icon={<ScissorOutlined />}
              onClick={() => setDestinationModal({ visible: true, mode: 'move' })}
            >
              이동
            </Button>
            {selectedItems.size === 1 && (
              <Button
                size="small"
                icon={<EditOutlined />}
                onClick={() => {
                  const path = Array.from(selectedItems)[0];
                  const record = sortedContent.find(item => item.path === path);
                  if (record) {
                    setRenameModal({
                      visible: true,
                      record,
                      newName: record.name,
                    });
                  }
                }}
              >
                이름 변경
              </Button>
            )}
            <Button
              size="small"
              icon={<DeleteOutlined />}
              danger
              onClick={handleBulkDelete}
            >
              삭제
            </Button>
            <Button
              size="small"
              onClick={() => {
                setSelectedItems(new Set());
                setLastSelectedIndex(-1);
              }}
            >
              선택 해제
            </Button>
          </AntSpace>
        </div>
      )}

      {viewMode === 'table' ? (
        <Table
          dataSource={sortedContent}
          columns={columns}
          loading={isLoading}
          rowKey="path"
          pagination={false}
          rowSelection={{
            type: 'checkbox',
            selectedRowKeys: Array.from(selectedItems),
            onChange: (keys) => setSelectedItems(new Set(keys as string[])),
          }}
          onChange={(_, __, sorter: any) => {
            if (sorter && !Array.isArray(sorter)) {
              const field = sorter.field as 'name' | 'modTime' | 'size';
              const order = sorter.order as 'ascend' | 'descend' | undefined;
              if (field && order) {
                setSortConfig({ sortBy: field, sortOrder: order });
              }
            }
          }}
          onRow={(record: FileNode, index?: number) => ({
            onClick: (e: React.MouseEvent<HTMLElement>) => handleItemClick(e, record, index ?? 0),
            onDoubleClick: () => record.isDir && setPath(record.path),
            onContextMenu: (e: React.MouseEvent<HTMLElement>) => handleContextMenu(e, record),
            draggable: true,
            onDragStart: (e: React.DragEvent<HTMLElement>) => handleItemDragStart(e, record),
            onDragEnd: (e: React.DragEvent<HTMLElement>) => handleItemDragEnd(e),
            onDragOver: (e: React.DragEvent<HTMLElement>) => record.isDir && handleFolderDragOver(e, record),
            onDragLeave: (e: React.DragEvent<HTMLElement>) => record.isDir && handleFolderDragLeave(e),
            onDrop: (e: React.DragEvent<HTMLElement>) => record.isDir && handleFolderDrop(e, record),
            style: {
              backgroundColor: dragOverFolder === record.path ? 'rgba(24, 144, 255, 0.1)' : undefined,
              userSelect: 'none',
            } as React.CSSProperties,
          })}
          locale={{ emptyText: '이 폴더는 비어 있습니다.' }}
        />
      ) : (
        <Row gutter={[16, 16]}>
          {(sortedContent?.length ?? 0) === 0 && !isLoading ? (
            <Col span={24}>
              <Empty description="이 폴더는 비어 있습니다." />
            </Col>
          ) : (
            sortedContent?.map((item, index) => {
              const isSelected = selectedItems.has(item.path);
              return (
              <Col key={item.path} xs={12} sm={8} md={6} lg={4} xl={3}>
                <Card
                  hoverable
                  draggable
                  onClick={(e) => handleItemClick(e, item, index)}
                  onDoubleClick={() => item.isDir && setPath(item.path)}
                  onContextMenu={(e) => handleContextMenu(e, item)}
                  onDragStart={(e) => handleItemDragStart(e, item)}
                  onDragEnd={(e) => handleItemDragEnd(e)}
                  onDragOver={(e) => item.isDir && handleFolderDragOver(e, item)}
                  onDragLeave={(e) => item.isDir && handleFolderDragLeave(e)}
                  onDrop={(e) => item.isDir && handleFolderDrop(e, item)}
                  style={{
                    textAlign: 'center',
                    cursor: 'pointer',
                    border: isSelected ? '2px solid #1890ff' : dragOverFolder === item.path ? '2px dashed #1890ff' : undefined,
                    backgroundColor: isSelected ? 'rgba(24, 144, 255, 0.1)' : dragOverFolder === item.path ? 'rgba(24, 144, 255, 0.05)' : undefined,
                    userSelect: 'none',
                  }}
                  styles={{ body: { padding: '16px 8px' } }}
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
              );
            })
          )}
        </Row>
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
          onChange={(e: React.ChangeEvent<HTMLInputElement>) => setRenameModal({ ...renameModal, newName: e.target.value })}
          onPressEnter={handleRename}
        />
      </Modal>

      <Modal
        title="새 폴더 만들기"
        open={createFolderModal.visible}
        onOk={handleCreateFolder}
        onCancel={() => setCreateFolderModal({ visible: false, folderName: '' })}
        okText="생성"
        cancelText="취소"
      >
        <Input
          placeholder="폴더 이름"
          value={createFolderModal.folderName}
          onChange={(e: React.ChangeEvent<HTMLInputElement>) => setCreateFolderModal({ ...createFolderModal, folderName: e.target.value })}
          onPressEnter={handleCreateFolder}
          autoFocus
        />
      </Modal>

      <DestinationPickerModal
        visible={destinationModal.visible}
        mode={destinationModal.mode}
        sourceCount={selectedItems.size}
        sources={Array.from(selectedItems)}
        currentPath={selectedPath}
        selectedSpace={selectedSpace}
        onConfirm={destinationModal.mode === 'move' ? handleMove : handleCopy}
        onCancel={() => setDestinationModal({ visible: false, mode: 'move' })}
      />

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
