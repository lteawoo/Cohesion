import { useCallback } from 'react';
import { App } from 'antd';
import { useBrowseStore } from '@/stores/browseStore';
import type { Space } from '@/features/space/types';

interface UseFileOperationsReturn {
  handleRename: (oldPath: string, newName: string) => Promise<void>;
  handleCreateFolder: (parentPath: string, folderName: string) => Promise<void>;
  handleDelete: (record: { path: string; name: string; isDir: boolean }) => Promise<void>;
  handleBulkDelete: (paths: string[]) => Promise<void>;
  handleMove: (sources: string[], destination: string, destinationSpace?: Space) => Promise<void>;
  handleCopy: (sources: string[], destination: string, destinationSpace?: Space) => Promise<void>;
  handleBulkDownload: (paths: string[]) => Promise<void>;
  handleFileUpload: (file: File, targetPath: string) => Promise<void>;
}

// 절대 경로를 Space 상대 경로로 변환
function toRelativePath(spacePath: string, absolutePath: string): string {
  return absolutePath.replace(spacePath, '').replace(/^\//, '');
}

export function useFileOperations(selectedPath: string, selectedSpace?: Space): UseFileOperationsReturn {
  const { message, modal } = App.useApp();
  const fetchSpaceContents = useBrowseStore((state) => state.fetchSpaceContents);
  const invalidateTree = useBrowseStore((state) => state.invalidateTree);

  // 현재 경로로 목록 새로고침 (Space 필수)
  const refreshContents = useCallback(async () => {
    if (!selectedSpace) return;
    const relativePath = toRelativePath(selectedSpace.space_path, selectedPath);
    await fetchSpaceContents(selectedSpace.id, relativePath);
  }, [selectedPath, selectedSpace, fetchSpaceContents]);

  // 파일 업로드 실행 함수
  const performUpload = useCallback(
    async (file: File, targetPath: string, overwrite: boolean = false): Promise<void> => {
      if (!selectedSpace) throw new Error('Space가 선택되지 않았습니다');

      const formData = new FormData();
      formData.append('file', file);
      formData.append('path', toRelativePath(selectedSpace.space_path, targetPath));
      if (overwrite) {
        formData.append('overwrite', 'true');
      }

      const response = await fetch(`/api/spaces/${selectedSpace.id}/files/upload`, {
        method: 'POST',
        body: formData,
      });

      if (!response.ok) {
        const error = await response.json();
        throw { status: response.status, message: error.message || 'Failed to upload' };
      }

      const result = await response.json();
      message.success(`"${result.filename}" 업로드 완료`);
      await refreshContents();
    },
    [selectedSpace, refreshContents, message]
  );

  // 파일 업로드 처리 (중복 확인 포함)
  const handleFileUpload = useCallback(
    async (file: File, targetPath: string) => {
      try {
        await performUpload(file, targetPath, false);
      } catch (error: unknown) {
        if (error && typeof error === 'object' && 'status' in error && error.status === 409) {
          modal.confirm({
            title: '파일 덮어쓰기',
            content: `"${file.name}" 파일이 이미 존재합니다. 덮어쓰시겠습니까?`,
            okText: '덮어쓰기',
            okType: 'danger',
            cancelText: '취소',
            onOk: async () => {
              try {
                await performUpload(file, targetPath, true);
              } catch (retryError: unknown) {
                const errorMessage = retryError && typeof retryError === 'object' && 'message' in retryError
                  ? String(retryError.message)
                  : '업로드 실패';
                message.error(errorMessage);
              }
            },
          });
        } else {
          const errorMessage = error && typeof error === 'object' && 'message' in error
            ? String(error.message)
            : '업로드 실패';
          message.error(errorMessage);
        }
      }
    },
    [performUpload, modal, message]
  );

  // 이름 변경 처리
  const handleRename = useCallback(
    async (oldPath: string, newName: string) => {
      if (!newName.trim()) {
        message.error('새 이름을 입력하세요');
        return;
      }
      if (!selectedSpace) {
        message.error('Space가 선택되지 않았습니다');
        return;
      }

      try {
        const response = await fetch(`/api/spaces/${selectedSpace.id}/files/rename`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            path: toRelativePath(selectedSpace.space_path, oldPath),
            newName: newName.trim(),
          }),
        });

        if (!response.ok) {
          const error = await response.json();
          throw new Error(error.message || 'Failed to rename');
        }

        message.success('이름이 변경되었습니다');
        await refreshContents();
        invalidateTree();
      } catch (error) {
        message.error(error instanceof Error ? error.message : '이름 변경 실패');
      }
    },
    [selectedSpace, refreshContents, message, invalidateTree]
  );

  // 새 폴더 만들기 처리
  const handleCreateFolder = useCallback(
    async (parentPath: string, folderName: string) => {
      if (!folderName.trim()) {
        message.error('폴더 이름을 입력하세요');
        return;
      }
      if (!selectedSpace) {
        message.error('Space가 선택되지 않았습니다');
        return;
      }

      try {
        const response = await fetch(`/api/spaces/${selectedSpace.id}/files/create-folder`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            parentPath: toRelativePath(selectedSpace.space_path, parentPath),
            folderName: folderName.trim(),
          }),
        });

        if (!response.ok) {
          const error = await response.json();
          throw new Error(error.message || 'Failed to create folder');
        }

        message.success('폴더가 생성되었습니다');
        await refreshContents();
        invalidateTree();
      } catch (error) {
        message.error(error instanceof Error ? error.message : '폴더 생성 실패');
      }
    },
    [selectedSpace, refreshContents, message, invalidateTree]
  );

  // 다중 다운로드 처리
  const handleBulkDownload = useCallback(
    async (paths: string[]) => {
      if (paths.length === 0) return;
      if (!selectedSpace) {
        message.error('Space가 선택되지 않았습니다');
        return;
      }

      try {
        const relativePaths = paths.map(p => toRelativePath(selectedSpace.space_path, p));
        const response = await fetch(`/api/spaces/${selectedSpace.id}/files/download-multiple`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ paths: relativePaths }),
        });

        if (!response.ok) {
          const error = await response.json();
          throw new Error(error.message || 'Failed to download');
        }

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
      } catch (error) {
        message.error(error instanceof Error ? error.message : '다운로드 실패');
      }
    },
    [selectedSpace, message]
  );

  // 다중 삭제 처리
  const handleBulkDelete = useCallback(
    async (paths: string[]) => {
      if (paths.length === 0) return;

      modal.confirm({
        title: '삭제 확인',
        content: `선택한 ${paths.length}개 항목을 삭제하시겠습니까?`,
        okText: '삭제',
        okType: 'danger',
        cancelText: '취소',
        onOk: async () => {
          if (!selectedSpace) {
            message.error('Space가 선택되지 않았습니다');
            return;
          }
          try {
            const relativePaths = paths.map(p => toRelativePath(selectedSpace.space_path, p));
            const response = await fetch(`/api/spaces/${selectedSpace.id}/files/delete-multiple`, {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ paths: relativePaths }),
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

            await refreshContents();
            invalidateTree();
          } catch (error) {
            message.error(error instanceof Error ? error.message : '삭제 실패');
          }
        },
      });
    },
    [selectedSpace, refreshContents, message, modal, invalidateTree]
  );

  // 이동 처리 (cross-Space 지원)
  const handleMove = useCallback(
    async (sources: string[], destination: string, destinationSpace?: Space) => {
      if (sources.length === 0) return;
      if (!selectedSpace) {
        message.error('Space가 선택되지 않았습니다');
        return;
      }

      const dstSpace = destinationSpace ?? selectedSpace;

      try {
        const relativeSources = sources.map(s => toRelativePath(selectedSpace.space_path, s));
        const relativeDestination = toRelativePath(dstSpace.space_path, destination);

        const response = await fetch(`/api/spaces/${selectedSpace.id}/files/move`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            sources: relativeSources,
            destination: {
              spaceId: dstSpace.id,
              path: relativeDestination,
            },
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

        await refreshContents();
        invalidateTree();
      } catch (error) {
        message.error(error instanceof Error ? error.message : '이동 실패');
      }
    },
    [selectedSpace, refreshContents, message, invalidateTree]
  );

  // 복사 처리 (cross-Space 지원)
  const handleCopy = useCallback(
    async (sources: string[], destination: string, destinationSpace?: Space) => {
      if (sources.length === 0) return;
      if (!selectedSpace) {
        message.error('Space가 선택되지 않았습니다');
        return;
      }

      const dstSpace = destinationSpace ?? selectedSpace;

      try {
        const relativeSources = sources.map(s => toRelativePath(selectedSpace.space_path, s));
        const relativeDestination = toRelativePath(dstSpace.space_path, destination);

        const response = await fetch(`/api/spaces/${selectedSpace.id}/files/copy`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            sources: relativeSources,
            destination: {
              spaceId: dstSpace.id,
              path: relativeDestination,
            },
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

        await refreshContents();
        invalidateTree();
      } catch (error) {
        message.error(error instanceof Error ? error.message : '복사 실패');
      }
    },
    [selectedSpace, refreshContents, message, invalidateTree]
  );

  // 단일 삭제 처리
  const handleDelete = useCallback(
    async (record: { path: string; name: string; isDir: boolean }) => {
      modal.confirm({
        title: '삭제 확인',
        content: `"${record.name}"을(를) 삭제하시겠습니까?${
          record.isDir ? ' (폴더 내 모든 파일도 삭제됩니다)' : ''
        }`,
        okText: '삭제',
        okType: 'danger',
        cancelText: '취소',
        onOk: async () => {
          if (!selectedSpace) {
            message.error('Space가 선택되지 않았습니다');
            return;
          }
          try {
            const response = await fetch(`/api/spaces/${selectedSpace.id}/files/delete`, {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ path: toRelativePath(selectedSpace.space_path, record.path) }),
            });

            if (!response.ok) {
              const error = await response.json();
              throw new Error(error.message || 'Failed to delete');
            }

            message.success('삭제되었습니다');
            await refreshContents();
            invalidateTree();
          } catch (error) {
            message.error(error instanceof Error ? error.message : '삭제 실패');
          }
        },
      });
    },
    [selectedSpace, refreshContents, message, modal, invalidateTree]
  );

  return {
    handleRename,
    handleCreateFolder,
    handleDelete,
    handleBulkDelete,
    handleMove,
    handleCopy,
    handleBulkDownload,
    handleFileUpload,
  };
}
