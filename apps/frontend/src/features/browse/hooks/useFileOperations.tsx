import { useCallback } from 'react';
import { App, Radio, Space as AntSpace, Typography } from 'antd';
import { useBrowseStore } from '@/stores/browseStore';
import type { TreeInvalidationTarget } from '@/stores/browseStore';
import type { Space } from '@/features/space/types';
import { apiFetch } from '@/api/client';

type UploadConflictPolicy = 'overwrite' | 'rename' | 'skip';
type UploadSource = File | File[] | FileList;
type UploadStatus = 'uploaded' | 'skipped';

interface UseFileOperationsReturn {
  handleRename: (oldPath: string, newName: string) => Promise<void>;
  handleCreateFolder: (parentPath: string, folderName: string) => Promise<void>;
  handleDelete: (record: { path: string; name: string; isDir: boolean }) => Promise<void>;
  handleBulkDelete: (paths: string[]) => Promise<void>;
  handleMove: (sources: string[], destination: string, destinationSpace?: Space) => Promise<void>;
  handleCopy: (sources: string[], destination: string, destinationSpace?: Space) => Promise<void>;
  handleBulkDownload: (paths: string[]) => Promise<void>;
  handleFileUpload: (files: UploadSource, targetPath: string) => Promise<void>;
}

interface DownloadTicketResponse {
  downloadUrl?: string;
  fileName?: string;
}

interface UploadResponsePayload {
  message?: string;
  filename?: string;
  status?: UploadStatus;
}

interface UploadResult {
  status: UploadStatus;
  filename: string;
}

interface UploadSummary {
  uploaded: number;
  skipped: number;
  failed: number;
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

function triggerBrowserDownloadFromUrl(url: string, fileName?: string): void {
  const anchor = document.createElement('a');
  anchor.href = url;
  if (fileName) {
    anchor.download = fileName;
  }
  document.body.appendChild(anchor);
  anchor.click();
  document.body.removeChild(anchor);
}

function normalizeUploadSource(files: UploadSource): File[] {
  if (files instanceof File) {
    return [files];
  }
  if (Array.isArray(files)) {
    return files;
  }
  return Array.from(files);
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

  // 파일 업로드 실행 함수
  const performUpload = useCallback(
    async (file: File, targetPath: string, conflictPolicy?: UploadConflictPolicy): Promise<UploadResult> => {
      if (!selectedSpace) throw new Error('Space가 선택되지 않았습니다');

      const formData = new FormData();
      formData.append('file', file);
      formData.append('path', normalizeRelativePath(targetPath));
      if (conflictPolicy) {
        formData.append('conflictPolicy', conflictPolicy);
      }

      const response = await apiFetch(`/api/spaces/${selectedSpace.id}/files/upload`, {
        method: 'POST',
        body: formData,
      });

      if (!response.ok) {
        const errorMessage = await readErrorMessage(response, '업로드 실패');
        throw { status: response.status, message: errorMessage };
      }

      const result = (await response.json()) as UploadResponsePayload;
      const status = result.status === 'skipped' ? 'skipped' : 'uploaded';
      const filename = typeof result.filename === 'string' && result.filename.trim()
        ? result.filename
        : file.name;
      return { status, filename };
    },
    [selectedSpace, readErrorMessage]
  );

  const promptConflictPolicy = useCallback((fileName: string): Promise<UploadConflictPolicy | null> => {
    return new Promise((resolve) => {
      let selectedPolicy: UploadConflictPolicy = 'overwrite';
      let settled = false;
      const settle = (value: UploadConflictPolicy | null) => {
        if (settled) {
          return;
        }
        settled = true;
        resolve(value);
      };

      modal.confirm({
        title: '업로드 충돌 처리',
        content: (
          <AntSpace direction="vertical" size={12} style={{ width: '100%' }}>
            <Typography.Text>
              "{fileName}" 파일이 이미 존재합니다.
            </Typography.Text>
            <Radio.Group
              defaultValue="overwrite"
              onChange={(event) => {
                selectedPolicy = event.target.value as UploadConflictPolicy;
              }}
            >
              <AntSpace direction="vertical" size={8}>
                <Radio value="overwrite">덮어쓰기</Radio>
                <Radio value="rename">이름 변경</Radio>
                <Radio value="skip">건너뛰기</Radio>
              </AntSpace>
            </Radio.Group>
            <Typography.Text type="secondary">
              선택한 정책은 이번 업로드 작업의 모든 충돌 파일에 동일하게 적용됩니다.
            </Typography.Text>
          </AntSpace>
        ),
        okText: '적용',
        cancelText: '업로드 중단',
        onOk: () => {
          settle(selectedPolicy);
        },
        onCancel: () => {
          settle(null);
        },
      });
    });
  }, [modal]);

  // 파일 업로드 처리 (다중 업로드 + 충돌 정책 일괄 적용)
  const handleFileUpload = useCallback(
    async (files: UploadSource, targetPath: string) => {
      const uploadFiles = normalizeUploadSource(files);
      if (uploadFiles.length === 0) {
        return;
      }

      let batchConflictPolicy: UploadConflictPolicy | null = null;
      let abortedByUser = false;
      const summary: UploadSummary = { uploaded: 0, skipped: 0, failed: 0 };
      const failedReasons: string[] = [];
      const uploadedNames: string[] = [];
      const skippedNames: string[] = [];

      for (const file of uploadFiles) {
        try {
          const result = await performUpload(file, targetPath, batchConflictPolicy ?? undefined);
          if (result.status === 'skipped') {
            summary.skipped += 1;
            skippedNames.push(result.filename);
          } else {
            summary.uploaded += 1;
            uploadedNames.push(result.filename);
          }
          continue;
        } catch (error: unknown) {
          const status = error && typeof error === 'object' && 'status' in error
            ? Number(error.status)
            : 0;
          const errorMessage = error && typeof error === 'object' && 'message' in error
            ? String(error.message)
            : '업로드 실패';

          if (status === 409 && !batchConflictPolicy) {
            const selectedPolicy = await promptConflictPolicy(file.name);
            if (!selectedPolicy) {
              abortedByUser = true;
              break;
            }
            batchConflictPolicy = selectedPolicy;

            try {
              const retried = await performUpload(file, targetPath, batchConflictPolicy);
              if (retried.status === 'skipped') {
                summary.skipped += 1;
                skippedNames.push(retried.filename);
              } else {
                summary.uploaded += 1;
                uploadedNames.push(retried.filename);
              }
            } catch (retryError: unknown) {
              summary.failed += 1;
              const retryMessage = retryError && typeof retryError === 'object' && 'message' in retryError
                ? String(retryError.message)
                : '업로드 실패';
              failedReasons.push(`${file.name}: ${retryMessage}`);
            }
            continue;
          }

          summary.failed += 1;
          failedReasons.push(`${file.name}: ${errorMessage}`);
        }
      }

      if (summary.uploaded > 0 || summary.skipped > 0) {
        await refreshContents();
      }

      if (uploadFiles.length === 1) {
        if (abortedByUser) {
          message.info('업로드를 중단했습니다');
          return;
        }
        if (summary.uploaded === 1) {
          message.success(`"${uploadedNames[0] ?? uploadFiles[0].name}" 업로드 완료`);
          return;
        }
        if (summary.skipped === 1) {
          message.warning(`"${skippedNames[0] ?? uploadFiles[0].name}" 건너뜀`);
          return;
        }
        message.error(failedReasons[0] ?? '업로드 실패');
        return;
      }

      const summaryMessage = `업로드 결과: 성공 ${summary.uploaded}개 / 건너뜀 ${summary.skipped}개 / 실패 ${summary.failed}개`;
      if (abortedByUser) {
        message.warning(`${summaryMessage} (사용자 중단)`);
        return;
      }
      if (summary.failed > 0) {
        const firstFailure = failedReasons[0] ? ` - ${failedReasons[0]}` : '';
        message.warning(`${summaryMessage}${firstFailure}`);
        return;
      }
      message.success(summaryMessage);
    },
    [performUpload, promptConflictPolicy, refreshContents, message]
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
          const ticketResponse = await apiFetch(`/api/spaces/${selectedSpace.id}/files/download-ticket`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: relativePaths[0] }),
          });

          if (!ticketResponse.ok) {
            const errorMessage = await readErrorMessage(ticketResponse, '다운로드 준비 실패');
            throw new Error(errorMessage);
          }

          const payload = (await ticketResponse.json()) as DownloadTicketResponse;
          if (!payload.downloadUrl || typeof payload.downloadUrl !== 'string') {
            throw new Error('다운로드 URL 생성 실패');
          }
          triggerBrowserDownloadFromUrl(payload.downloadUrl, payload.fileName);
          return;
        }

        const ticketResponse = await apiFetch(`/api/spaces/${selectedSpace.id}/files/download-multiple-ticket`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ paths: relativePaths }),
        });

        if (!ticketResponse.ok) {
          const errorMessage = await readErrorMessage(ticketResponse, '다운로드 준비 실패');
          throw new Error(errorMessage);
        }

        const payload = (await ticketResponse.json()) as DownloadTicketResponse;
        if (!payload.downloadUrl || typeof payload.downloadUrl !== 'string') {
          throw new Error('다운로드 URL 생성 실패');
        }
        triggerBrowserDownloadFromUrl(payload.downloadUrl, payload.fileName);
      } catch (error) {
        message.error(error instanceof Error ? error.message : '다운로드 실패');
      }
    },
    [selectedSpace, message, readErrorMessage]
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
