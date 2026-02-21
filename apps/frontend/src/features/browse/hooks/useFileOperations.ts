import { useCallback } from 'react';
import { App } from 'antd';
import { useBrowseStore } from '@/stores/browseStore';
import type { TreeInvalidationTarget } from '@/stores/browseStore';
import type { Space } from '@/features/space/types';
import { apiFetch } from '@/api/client';

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

function normalizeRelativePath(path: string): string {
  return path.replace(/^\/+/, '').replace(/\/+$/, '');
}

function getParentPath(path: string): string {
  if (!path) return '';
  const normalizedPath = normalizeRelativePath(path);
  if (!normalizedPath) return '';
  const lastSlashIndex = normalizedPath.lastIndexOf('/');
  if (lastSlashIndex <= 0) return '';
  return normalizedPath.slice(0, lastSlashIndex);
}

function createInvalidationTarget(path: string, space?: Space): TreeInvalidationTarget {
  return {
    path,
    spaceId: space?.id,
  };
}

function resolveDownloadFileName(contentDisposition: string | null, fallbackFileName: string): string {
  if (!contentDisposition) {
    return fallbackFileName;
  }

  const utf8Match = contentDisposition.match(/filename\*\s*=\s*UTF-8''([^;]+)/i);
  if (utf8Match?.[1]) {
    try {
      return decodeURIComponent(utf8Match[1].trim());
    } catch {
      return utf8Match[1].trim();
    }
  }

  const fileNameMatch = contentDisposition.match(/filename\s*=\s*"([^"]+)"|filename\s*=\s*([^;]+)/i);
  const matched = fileNameMatch?.[1] ?? fileNameMatch?.[2];
  if (matched) {
    return matched.trim();
  }

  return fallbackFileName;
}

function triggerBrowserDownload(blob: Blob, fileName: string): void {
  const url = window.URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = fileName;
  document.body.appendChild(anchor);
  anchor.click();
  window.URL.revokeObjectURL(url);
  document.body.removeChild(anchor);
}

export function useFileOperations(selectedPath: string, selectedSpace?: Space): UseFileOperationsReturn {
  const { message, modal } = App.useApp();
  const fetchSpaceContents = useBrowseStore((state) => state.fetchSpaceContents);
  const invalidateTree = useBrowseStore((state) => state.invalidateTree);

  // 현재 경로로 목록 새로고침 (Space 필수)
  const refreshContents = useCallback(async () => {
    if (!selectedSpace) return;
    await fetchSpaceContents(selectedSpace.id, normalizeRelativePath(selectedPath));
  }, [selectedPath, selectedSpace, fetchSpaceContents]);

  const readErrorMessage = useCallback(async (response: Response, fallback: string): Promise<string> => {
    try {
      const error = await response.json();
      if (error?.message && typeof error.message === 'string') {
        return error.message;
      }
    } catch {
      // ignore parse errors and fallback to default message
    }
    return fallback;
  }, []);

  const downloadResponse = useCallback(
    async (response: Response, fallbackFileName: string) => {
      if (!response.ok) {
        const errorMessage = await readErrorMessage(response, '다운로드 실패');
        throw new Error(errorMessage);
      }

      const resolvedFileName = resolveDownloadFileName(response.headers.get('Content-Disposition'), fallbackFileName);
      const blob = await response.blob();
      triggerBrowserDownload(blob, resolvedFileName);
    },
    [readErrorMessage]
  );

  // 파일 업로드 실행 함수
  const performUpload = useCallback(
    async (file: File, targetPath: string, overwrite: boolean = false): Promise<void> => {
      if (!selectedSpace) throw new Error('Space가 선택되지 않았습니다');

      const formData = new FormData();
      formData.append('file', file);
      formData.append('path', normalizeRelativePath(targetPath));
      if (overwrite) {
        formData.append('overwrite', 'true');
      }

      const response = await apiFetch(`/api/spaces/${selectedSpace.id}/files/upload`, {
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
        const response = await apiFetch(`/api/spaces/${selectedSpace.id}/files/rename`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            path: normalizeRelativePath(oldPath),
            newName: newName.trim(),
          }),
        });

        if (!response.ok) {
          const error = await response.json();
          throw new Error(error.message || 'Failed to rename');
        }

        message.success('이름이 변경되었습니다');
        await refreshContents();
        invalidateTree([createInvalidationTarget(getParentPath(oldPath), selectedSpace)]);
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
        const response = await apiFetch(`/api/spaces/${selectedSpace.id}/files/create-folder`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            parentPath: normalizeRelativePath(parentPath),
            folderName: folderName.trim(),
          }),
        });

        if (!response.ok) {
          const error = await response.json();
          throw new Error(error.message || 'Failed to create folder');
        }

        message.success('폴더가 생성되었습니다');
        await refreshContents();
        invalidateTree([createInvalidationTarget(parentPath, selectedSpace)]);
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
        const relativePaths = paths.map(p => normalizeRelativePath(p));

        if (relativePaths.length === 1) {
          const response = await apiFetch(
            `/api/spaces/${selectedSpace.id}/files/download?path=${encodeURIComponent(relativePaths[0])}`
          );
          await downloadResponse(response, relativePaths[0].split('/').pop() || 'download.bin');
          return;
        }

        const response = await apiFetch(`/api/spaces/${selectedSpace.id}/files/download-multiple`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ paths: relativePaths }),
        });
        await downloadResponse(response, `download-${Date.now()}.zip`);
      } catch (error) {
        message.error(error instanceof Error ? error.message : '다운로드 실패');
      }
    },
    [selectedSpace, message, downloadResponse]
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
            const relativePaths = paths.map(p => normalizeRelativePath(p));
            const response = await apiFetch(`/api/spaces/${selectedSpace.id}/files/delete-multiple`, {
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
            invalidateTree(paths.map((path) => createInvalidationTarget(getParentPath(path), selectedSpace)));
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
        const relativeSources = sources.map(s => normalizeRelativePath(s));
        const relativeDestination = normalizeRelativePath(destination);

        const response = await apiFetch(`/api/spaces/${selectedSpace.id}/files/move`, {
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
        invalidateTree([
          ...sources.map((source) => createInvalidationTarget(getParentPath(source), selectedSpace)),
          createInvalidationTarget(destination, dstSpace),
        ]);
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
        const relativeSources = sources.map(s => normalizeRelativePath(s));
        const relativeDestination = normalizeRelativePath(destination);

        const response = await apiFetch(`/api/spaces/${selectedSpace.id}/files/copy`, {
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
        invalidateTree([createInvalidationTarget(destination, dstSpace)]);
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
            const response = await apiFetch(`/api/spaces/${selectedSpace.id}/files/delete`, {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ path: normalizeRelativePath(record.path) }),
            });

            if (!response.ok) {
              const error = await response.json();
              throw new Error(error.message || 'Failed to delete');
            }

            message.success('삭제되었습니다');
            await refreshContents();
            invalidateTree([createInvalidationTarget(getParentPath(record.path), selectedSpace)]);
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
