import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { App, Button, Empty, Radio, Space as AntSpace, Table, Typography } from 'antd';
import type { TableColumnsType } from 'antd';
import { DeleteOutlined, ReloadOutlined, RollbackOutlined } from '@ant-design/icons';
import { apiFetch } from '@/api/client';
import { toApiError } from '@/api/error';
import { useBrowseStore } from '@/stores/browseStore';
import { useSpaceStore } from '@/stores/spaceStore';
import { formatDate, formatSize } from '../constants';
import { FileTypeIcon } from './FileTypeIcon';

type TrashConflictPolicy = 'overwrite' | 'rename' | 'skip';

interface TrashItem {
  id: number;
  originalPath: string;
  itemName: string;
  isDir: boolean;
  itemSize: number;
  deletedBy: string;
  deletedAt: string;
}

interface TrashListResponsePayload {
  items?: TrashItem[];
}

interface TrashRestoreSuccessPayload {
  id?: number;
  originalPath?: string;
}

interface TrashRestoreFailurePayload {
  id?: number;
  originalPath?: string;
  reason?: string;
  code?: string;
}

interface TrashRestoreResponsePayload {
  succeeded?: TrashRestoreSuccessPayload[];
  skipped?: TrashRestoreSuccessPayload[];
  failed?: TrashRestoreFailurePayload[];
}

interface TrashDeleteSuccessPayload {
  id?: number;
}

interface TrashDeleteFailurePayload {
  id?: number;
  reason?: string;
}

interface TrashDeleteResponsePayload {
  succeeded?: TrashDeleteSuccessPayload[];
  failed?: TrashDeleteFailurePayload[];
}

interface TrashEmptyFailurePayload {
  id?: number;
  reason?: string;
}

interface TrashEmptyResponsePayload {
  removed?: number;
  failed?: TrashEmptyFailurePayload[];
}

interface GlobalTrashItem extends TrashItem {
  rowKey: string;
  spaceId: number;
  spaceName: string;
}

function isDestinationConflictFailure(item: TrashRestoreFailurePayload): boolean {
  return item.code === 'destination_exists';
}

function summarizeTrashRestoreFailure(item: TrashRestoreFailurePayload): string {
  const targetPath = item.originalPath ?? `#${item.id ?? 'unknown'}`;
  return `${targetPath}: ${item.reason ?? '복원 실패'}`;
}

function parseDeletedAt(value: string): number {
  const parsed = Date.parse(value);
  return Number.isNaN(parsed) ? 0 : parsed;
}

function buildGroupedIds(targetItems: GlobalTrashItem[]): Map<number, number[]> {
  const grouped = new Map<number, number[]>();
  targetItems.forEach((item) => {
    const current = grouped.get(item.spaceId);
    if (current) {
      current.push(item.id);
      return;
    }
    grouped.set(item.spaceId, [item.id]);
  });
  return grouped;
}

const TrashExplorer: React.FC = () => {
  const { message, modal } = App.useApp();

  const spaces = useSpaceStore((state) => state.spaces);
  const fetchSpaces = useSpaceStore((state) => state.fetchSpaces);
  const selectedPath = useBrowseStore((state) => state.selectedPath);
  const selectedSpace = useBrowseStore((state) => state.selectedSpace);
  const fetchSpaceContents = useBrowseStore((state) => state.fetchSpaceContents);

  const [items, setItems] = useState<GlobalTrashItem[]>([]);
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([]);
  const [loading, setLoading] = useState(false);
  const [processing, setProcessing] = useState(false);

  const selectedItems = useMemo(() => {
    if (selectedRowKeys.length === 0) {
      return [];
    }
    const keySet = new Set(selectedRowKeys.map((key) => String(key)));
    return items.filter((item) => keySet.has(item.rowKey));
  }, [items, selectedRowKeys]);

  const loadTrashItems = useCallback(async () => {
    if (spaces.length === 0) {
      setItems([]);
      setSelectedRowKeys([]);
      return;
    }

    setLoading(true);
    try {
      const settled = await Promise.allSettled(
        spaces.map(async (space) => {
          const response = await apiFetch(`/api/spaces/${space.id}/files/trash`, {
            method: 'GET',
          });
          if (!response.ok) {
            throw await toApiError(response, `[${space.space_name}] 휴지통 목록 조회 실패`);
          }
          const payload = (await response.json()) as TrashListResponsePayload;
          return (payload.items ?? []).map((item) => ({
            ...item,
            rowKey: `${space.id}:${item.id}`,
            spaceId: space.id,
            spaceName: space.space_name,
          }));
        })
      );

      const merged: GlobalTrashItem[] = [];
      const failures: string[] = [];
      settled.forEach((result) => {
        if (result.status === 'fulfilled') {
          merged.push(...result.value);
          return;
        }
        failures.push(result.reason instanceof Error ? result.reason.message : '휴지통 목록 조회 실패');
      });

      merged.sort((left, right) => parseDeletedAt(right.deletedAt) - parseDeletedAt(left.deletedAt));

      setItems(merged);
      setSelectedRowKeys((previous) => {
        const validKeys = new Set(merged.map((item) => item.rowKey));
        return previous.filter((key) => validKeys.has(String(key)));
      });

      if (failures.length > 0) {
        if (failures.length === spaces.length) {
          message.error(failures[0] ?? '휴지통 목록 조회 실패');
        } else {
          message.warning(`일부 Space 휴지통 조회 실패 (${failures.length}개)`);
        }
      }
    } finally {
      setLoading(false);
    }
  }, [message, spaces]);

  useEffect(() => {
    if (spaces.length === 0) {
      void fetchSpaces();
      return;
    }
    void loadTrashItems();
  }, [fetchSpaces, loadTrashItems, spaces.length]);

  const refreshSelectedFolder = useCallback(async () => {
    if (!selectedSpace) {
      return;
    }
    await fetchSpaceContents(selectedSpace.id, selectedPath);
  }, [fetchSpaceContents, selectedPath, selectedSpace]);

  const requestTrashRestore = useCallback(async (
    spaceId: number,
    ids: number[],
    conflictPolicy?: TrashConflictPolicy
  ): Promise<TrashRestoreResponsePayload> => {
    const payload: { ids: number[]; conflictPolicy?: TrashConflictPolicy } = { ids };
    if (conflictPolicy) {
      payload.conflictPolicy = conflictPolicy;
    }
    const response = await apiFetch(`/api/spaces/${spaceId}/files/trash-restore`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (!response.ok) {
      throw await toApiError(response, '휴지통 복원 실패');
    }
    return (await response.json()) as TrashRestoreResponsePayload;
  }, []);

  const requestTrashDelete = useCallback(async (
    spaceId: number,
    ids: number[]
  ): Promise<TrashDeleteResponsePayload> => {
    const response = await apiFetch(`/api/spaces/${spaceId}/files/trash-delete`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ids }),
    });
    if (!response.ok) {
      throw await toApiError(response, '휴지통 영구 삭제 실패');
    }
    return (await response.json()) as TrashDeleteResponsePayload;
  }, []);

  const requestTrashEmpty = useCallback(async (spaceId: number): Promise<TrashEmptyResponsePayload> => {
    const response = await apiFetch(`/api/spaces/${spaceId}/files/trash-empty`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
    });
    if (!response.ok) {
      throw await toApiError(response, '휴지통 비우기 실패');
    }
    return (await response.json()) as TrashEmptyResponsePayload;
  }, []);

  const promptTrashConflictPolicy = useCallback((conflictCount: number): Promise<TrashConflictPolicy | null> => {
    return new Promise((resolve) => {
      let selectedPolicy: TrashConflictPolicy = 'overwrite';
      let settled = false;
      const settle = (value: TrashConflictPolicy | null) => {
        if (settled) {
          return;
        }
        settled = true;
        resolve(value);
      };

      modal.confirm({
        title: '복원 충돌 처리',
        content: (
          <AntSpace direction="vertical" size={12} style={{ width: '100%' }}>
            <Typography.Text>
              복원 대상 중 {conflictCount}개 항목이 기존 경로와 충돌합니다.
            </Typography.Text>
            <Typography.Text type="secondary">
              충돌 항목 처리 정책을 선택하세요.
            </Typography.Text>
            <Radio.Group
              defaultValue="overwrite"
              onChange={(event) => {
                selectedPolicy = event.target.value as TrashConflictPolicy;
              }}
            >
              <AntSpace direction="vertical" size={8}>
                <Radio value="overwrite">덮어쓰기</Radio>
                <Radio value="rename">이름 변경</Radio>
                <Radio value="skip">건너뛰기</Radio>
              </AntSpace>
            </Radio.Group>
          </AntSpace>
        ),
        okText: '적용',
        cancelText: '복원 중단',
        onOk: () => {
          settle(selectedPolicy);
        },
        onCancel: () => {
          settle(null);
        },
      });
    });
  }, [modal]);

  const handleRestoreConfirm = useCallback(() => {
    if (selectedItems.length === 0) {
      message.warning('복원할 항목을 선택하세요');
      return;
    }

    modal.confirm({
      title: '복원 확인',
      content: `선택한 ${selectedItems.length}개 항목을 복원하시겠습니까?`,
      okText: '복원',
      cancelText: '취소',
      onOk: async () => {
        setProcessing(true);
        try {
          const groupedIds = buildGroupedIds(selectedItems);
          let succeededCount = 0;
          let skippedCount = 0;
          const failedReasons: string[] = [];
          const conflictIdsBySpace = new Map<number, number[]>();
          const unresolvedConflicts: TrashRestoreFailurePayload[] = [];

          for (const [spaceId, ids] of groupedIds.entries()) {
            try {
              const payload = await requestTrashRestore(spaceId, ids);
              const succeeded = payload.succeeded ?? [];
              const skipped = payload.skipped ?? [];
              const failed = payload.failed ?? [];

              succeededCount += succeeded.length;
              skippedCount += skipped.length;

              failed.forEach((item) => {
                if (isDestinationConflictFailure(item) && typeof item.id === 'number') {
                  const existing = conflictIdsBySpace.get(spaceId);
                  if (existing) {
                    existing.push(item.id);
                  } else {
                    conflictIdsBySpace.set(spaceId, [item.id]);
                  }
                  unresolvedConflicts.push(item);
                  return;
                }
                failedReasons.push(summarizeTrashRestoreFailure(item));
              });
            } catch (error) {
              failedReasons.push(error instanceof Error ? error.message : '휴지통 복원 실패');
            }
          }

          const conflictCount = Array.from(conflictIdsBySpace.values()).reduce((sum, ids) => sum + ids.length, 0);
          if (conflictCount > 0) {
            const policy = await promptTrashConflictPolicy(conflictCount);
            if (!policy) {
              failedReasons.push(...unresolvedConflicts.map(summarizeTrashRestoreFailure));
            } else {
              for (const [spaceId, ids] of conflictIdsBySpace.entries()) {
                try {
                  const retried = await requestTrashRestore(spaceId, ids, policy);
                  succeededCount += retried.succeeded?.length ?? 0;
                  skippedCount += retried.skipped?.length ?? 0;
                  failedReasons.push(...(retried.failed ?? []).map(summarizeTrashRestoreFailure));
                } catch (error) {
                  failedReasons.push(error instanceof Error ? error.message : '휴지통 복원 실패');
                }
              }
            }
          }

          const failedCount = failedReasons.length;
          const summaryMessage = `복원 결과: 성공 ${succeededCount}개 / 건너뜀 ${skippedCount}개 / 실패 ${failedCount}개`;
          if (failedCount > 0) {
            const firstFailure = failedReasons[0] ? ` - ${failedReasons[0]}` : '';
            message.warning(`${summaryMessage}${firstFailure}`);
          } else {
            message.success(summaryMessage);
          }

          await Promise.all([loadTrashItems(), refreshSelectedFolder()]);
        } finally {
          setProcessing(false);
        }
      },
    });
  }, [
    loadTrashItems,
    message,
    modal,
    promptTrashConflictPolicy,
    refreshSelectedFolder,
    requestTrashRestore,
    selectedItems,
  ]);

  const handleDeleteConfirm = useCallback(() => {
    if (selectedItems.length === 0) {
      message.warning('영구 삭제할 항목을 선택하세요');
      return;
    }

    modal.confirm({
      title: '영구 삭제 확인',
      content: `선택한 ${selectedItems.length}개 항목을 영구 삭제하시겠습니까? 이 작업은 되돌릴 수 없습니다.`,
      okText: '영구 삭제',
      okType: 'danger',
      cancelText: '취소',
      onOk: async () => {
        setProcessing(true);
        try {
          const groupedIds = buildGroupedIds(selectedItems);
          let succeededCount = 0;
          const failedReasons: string[] = [];

          for (const [spaceId, ids] of groupedIds.entries()) {
            try {
              const payload = await requestTrashDelete(spaceId, ids);
              succeededCount += payload.succeeded?.length ?? 0;
              (payload.failed ?? []).forEach((item) => {
                failedReasons.push(item.reason ?? `#${item.id ?? 'unknown'} 영구 삭제 실패`);
              });
            } catch (error) {
              failedReasons.push(error instanceof Error ? error.message : '휴지통 영구 삭제 실패');
            }
          }

          if (failedReasons.length > 0) {
            const firstFailure = failedReasons[0] ? ` - ${failedReasons[0]}` : '';
            message.warning(`영구 삭제 결과: 성공 ${succeededCount}개 / 실패 ${failedReasons.length}개${firstFailure}`);
          } else {
            message.success(`${succeededCount}개 항목을 영구 삭제했습니다`);
          }

          await Promise.all([loadTrashItems(), refreshSelectedFolder()]);
        } finally {
          setProcessing(false);
        }
      },
    });
  }, [loadTrashItems, message, modal, refreshSelectedFolder, requestTrashDelete, selectedItems]);

  const handleEmptyConfirm = useCallback(() => {
    if (items.length === 0) {
      message.info('휴지통이 비어 있습니다');
      return;
    }

    modal.confirm({
      title: '휴지통 비우기',
      content: `휴지통 항목 ${items.length}개를 모두 영구 삭제하시겠습니까? 이 작업은 되돌릴 수 없습니다.`,
      okText: '비우기',
      okType: 'danger',
      cancelText: '취소',
      onOk: async () => {
        setProcessing(true);
        try {
          const targetSpaceIds = Array.from(new Set(items.map((item) => item.spaceId)));
          let removedCount = 0;
          const failedReasons: string[] = [];

          for (const spaceId of targetSpaceIds) {
            try {
              const payload = await requestTrashEmpty(spaceId);
              removedCount += typeof payload.removed === 'number' ? payload.removed : 0;
              (payload.failed ?? []).forEach((item) => {
                failedReasons.push(item.reason ?? `#${item.id ?? 'unknown'} 비우기 실패`);
              });
            } catch (error) {
              failedReasons.push(error instanceof Error ? error.message : '휴지통 비우기 실패');
            }
          }

          if (failedReasons.length > 0) {
            const firstFailure = failedReasons[0] ? ` - ${failedReasons[0]}` : '';
            message.warning(`휴지통 비우기 결과: 제거 ${removedCount}개 / 실패 ${failedReasons.length}개${firstFailure}`);
          } else {
            message.success(`휴지통 비우기 완료 (${removedCount}개)`);
          }

          await Promise.all([loadTrashItems(), refreshSelectedFolder()]);
        } finally {
          setProcessing(false);
        }
      },
    });
  }, [items, loadTrashItems, message, modal, refreshSelectedFolder, requestTrashEmpty]);

  const columns = useMemo<TableColumnsType<GlobalTrashItem>>(() => {
    return [
      {
        title: '이름',
        dataIndex: 'itemName',
        key: 'itemName',
        ellipsis: true,
        render: (_, record) => (
          <AntSpace>
            <FileTypeIcon filename={record.itemName} isDirectory={record.isDir} size={16} />
            <span>{record.itemName}</span>
          </AntSpace>
        ),
      },
      {
        title: 'Space',
        dataIndex: 'spaceName',
        key: 'spaceName',
        width: 140,
        ellipsis: true,
      },
      {
        title: '원래 경로',
        dataIndex: 'originalPath',
        key: 'originalPath',
        ellipsis: true,
      },
      {
        title: '삭제 시각',
        dataIndex: 'deletedAt',
        key: 'deletedAt',
        width: 180,
        render: (value: string) => formatDate(value),
      },
      {
        title: '삭제자',
        dataIndex: 'deletedBy',
        key: 'deletedBy',
        width: 120,
      },
      {
        title: '크기',
        dataIndex: 'itemSize',
        key: 'itemSize',
        width: 120,
        align: 'right',
        render: (size: number, record) => (record.isDir ? '-' : formatSize(size)),
      },
    ];
  }, []);

  if (spaces.length === 0 && !loading) {
    return (
      <div style={{ height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <Empty description="연결된 Space가 없습니다." />
      </div>
    );
  }

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column', gap: 12, padding: 16 }}>
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          gap: 12,
          flexWrap: 'wrap',
        }}
      >
        <Typography.Title level={5} style={{ margin: 0 }}>
          휴지통
        </Typography.Title>
        <AntSpace wrap>
          <Button icon={<ReloadOutlined />} onClick={() => void loadTrashItems()} disabled={loading || processing}>
            새로고침
          </Button>
          <Button icon={<RollbackOutlined />} onClick={handleRestoreConfirm} disabled={selectedItems.length === 0 || loading || processing}>
            복원
          </Button>
          <Button icon={<DeleteOutlined />} danger onClick={handleDeleteConfirm} disabled={selectedItems.length === 0 || loading || processing}>
            영구 삭제
          </Button>
          <Button danger onClick={handleEmptyConfirm} disabled={items.length === 0 || loading || processing}>
            비우기
          </Button>
        </AntSpace>
      </div>

      <div style={{ minHeight: 0, flex: 1 }}>
        <Table<GlobalTrashItem>
          size="small"
          rowKey={(record) => record.rowKey}
          dataSource={items}
          columns={columns}
          loading={loading || processing}
          pagination={{ pageSize: 20, showSizeChanger: false }}
          rowSelection={{
            selectedRowKeys,
            onChange: (nextKeys) => {
              setSelectedRowKeys(nextKeys);
            },
          }}
          scroll={{ y: 'calc(100vh - 260px)' }}
          locale={{ emptyText: <Empty description="휴지통이 비어 있습니다." /> }}
        />
      </div>
    </div>
  );
};

export default TrashExplorer;
