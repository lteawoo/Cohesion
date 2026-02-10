import { useCallback } from 'react';
import { message, Modal } from 'antd';
import { useBrowseStore } from '@/stores/browseStore';

interface UseFileOperationsReturn {
  handleRename: (oldPath: string, newName: string) => Promise<void>;
  handleCreateFolder: (parentPath: string, folderName: string) => Promise<void>;
  handleDelete: (record: { path: string; name: string; isDir: boolean }) => Promise<void>;
  handleBulkDelete: (paths: string[]) => Promise<void>;
  handleMove: (sources: string[], destination: string) => Promise<void>;
  handleCopy: (sources: string[], destination: string) => Promise<void>;
  handleBulkDownload: (paths: string[]) => Promise<void>;
  handleFileUpload: (file: File, targetPath: string) => Promise<void>;
}

export function useFileOperations(selectedPath: string): UseFileOperationsReturn {
  const fetchDirectoryContents = useBrowseStore((state) => state.fetchDirectoryContents);

  // 파일 업로드 실행 함수
  const performUpload = useCallback(
    async (file: File, targetPath: string, overwrite: boolean = false): Promise<void> => {
      const formData = new FormData();
      formData.append('file', file);
      formData.append('targetPath', targetPath);
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
      await fetchDirectoryContents(targetPath);
    },
    [fetchDirectoryContents]
  );

  // 파일 업로드 처리 (중복 확인 포함)
  const handleFileUpload = useCallback(
    async (file: File, targetPath: string) => {
      try {
        await performUpload(file, targetPath, false);
      } catch (error: unknown) {
        // 파일 중복 에러 (409)
        if (error && typeof error === 'object' && 'status' in error && error.status === 409) {
          Modal.confirm({
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
    [performUpload]
  );

  // 이름 변경 처리
  const handleRename = useCallback(
    async (oldPath: string, newName: string) => {
      if (!newName.trim()) {
        message.error('새 이름을 입력하세요');
        return;
      }

      try {
        const response = await fetch('/api/browse/rename', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            oldPath,
            newName: newName.trim(),
          }),
        });

        if (!response.ok) {
          const error = await response.json();
          throw new Error(error.message || 'Failed to rename');
        }

        message.success('이름이 변경되었습니다');

        // 목록 새로고침
        await fetchDirectoryContents(selectedPath);
      } catch (error) {
        message.error(error instanceof Error ? error.message : '이름 변경 실패');
      }
    },
    [selectedPath, fetchDirectoryContents]
  );

  // 새 폴더 만들기 처리
  const handleCreateFolder = useCallback(
    async (parentPath: string, folderName: string) => {
      if (!folderName.trim()) {
        message.error('폴더 이름을 입력하세요');
        return;
      }

      try {
        const response = await fetch('/api/browse/create-folder', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            parentPath,
            folderName: folderName.trim(),
          }),
        });

        if (!response.ok) {
          const error = await response.json();
          throw new Error(error.message || 'Failed to create folder');
        }

        message.success('폴더가 생성되었습니다');

        // 디렉토리 목록 새로고침
        await fetchDirectoryContents(selectedPath);
      } catch (error) {
        message.error(error instanceof Error ? error.message : '폴더 생성 실패');
      }
    },
    [selectedPath, fetchDirectoryContents]
  );

  // 다중 다운로드 처리
  const handleBulkDownload = useCallback(async (paths: string[]) => {
    if (paths.length === 0) return;

    try {
      const response = await fetch('/api/browse/download-multiple', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ paths }),
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
    } catch (error) {
      message.error(error instanceof Error ? error.message : '다운로드 실패');
    }
  }, []);

  // 다중 삭제 처리
  const handleBulkDelete = useCallback(
    async (paths: string[]) => {
      if (paths.length === 0) return;

      Modal.confirm({
        title: '삭제 확인',
        content: `선택한 ${paths.length}개 항목을 삭제하시겠습니까?`,
        okText: '삭제',
        okType: 'danger',
        cancelText: '취소',
        onOk: async () => {
          try {
            const response = await fetch('/api/browse/delete-multiple', {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ paths }),
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

            // 목록 새로고침
            await fetchDirectoryContents(selectedPath);
          } catch (error) {
            message.error(error instanceof Error ? error.message : '삭제 실패');
          }
        },
      });
    },
    [selectedPath, fetchDirectoryContents]
  );

  // 이동 처리
  const handleMove = useCallback(
    async (sources: string[], destination: string) => {
      if (sources.length === 0) return;

      try {
        const response = await fetch('/api/browse/move', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            sources,
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

        // 목록 새로고침
        await fetchDirectoryContents(selectedPath);
      } catch (error) {
        message.error(error instanceof Error ? error.message : '이동 실패');
      }
    },
    [selectedPath, fetchDirectoryContents]
  );

  // 복사 처리
  const handleCopy = useCallback(
    async (sources: string[], destination: string) => {
      if (sources.length === 0) return;

      try {
        const response = await fetch('/api/browse/copy', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            sources,
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

        // 목록 새로고침
        await fetchDirectoryContents(selectedPath);
      } catch (error) {
        message.error(error instanceof Error ? error.message : '복사 실패');
      }
    },
    [selectedPath, fetchDirectoryContents]
  );

  // 삭제 처리
  const handleDelete = useCallback(
    async (record: { path: string; name: string; isDir: boolean }) => {
      Modal.confirm({
        title: '삭제 확인',
        content: `"${record.name}"을(를) 삭제하시겠습니까?${
          record.isDir ? ' (폴더 내 모든 파일도 삭제됩니다)' : ''
        }`,
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
    },
    [selectedPath, fetchDirectoryContents]
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
