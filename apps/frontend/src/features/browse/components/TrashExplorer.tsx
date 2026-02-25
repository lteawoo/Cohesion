import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { App, Button, Empty, Radio, Space as AntSpace, Table, Typography } from 'antd';
import type { TableColumnsType } from 'antd';
import { DeleteOutlined, FolderFilled, RollbackOutlined } from '@ant-design/icons';
import { apiFetch } from '@/api/client';
import { toApiError } from '@/api/error';
import { useBrowseStore } from '@/stores/browseStore';
import { useSpaceStore } from '@/stores/spaceStore';
import { formatDate, formatSize } from '../constants';
import { FileTypeIcon } from './FileTypeIcon';
import { useTranslation } from 'react-i18next';

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

function summarizeTrashRestoreFailure(item: TrashRestoreFailurePayload, fallbackMessage: string): string {
  const targetPath = item.originalPath ?? `#${item.id ?? 'unknown'}`;
  return `${targetPath}: ${item.reason ?? fallbackMessage}`;
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
  const { t } = useTranslation();
  const { message, modal } = App.useApp();

  const spaces = useSpaceStore((state) => state.spaces);
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
            throw await toApiError(response, t('trashExplorer.listLoadFailedBySpace', { spaceName: space.space_name }));
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
        failures.push(result.reason instanceof Error ? result.reason.message : t('trashExplorer.listLoadFailed'));
      });

      merged.sort((left, right) => parseDeletedAt(right.deletedAt) - parseDeletedAt(left.deletedAt));

      setItems(merged);
      setSelectedRowKeys((previous) => {
        const validKeys = new Set(merged.map((item) => item.rowKey));
        return previous.filter((key) => validKeys.has(String(key)));
      });

      if (failures.length > 0) {
        if (failures.length === spaces.length) {
          message.error(failures[0] ?? t('trashExplorer.listLoadFailed'));
        } else {
          message.warning(t('trashExplorer.partialListLoadFailed', { count: failures.length }));
        }
      }
    } finally {
      setLoading(false);
    }
  }, [message, spaces, t]);

  useEffect(() => {
    if (spaces.length === 0) {
      setItems((previous) => (previous.length === 0 ? previous : []));
      setSelectedRowKeys((previous) => (previous.length === 0 ? previous : []));
      return;
    }
    void loadTrashItems();
  }, [loadTrashItems, spaces.length]);

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
      throw await toApiError(response, t('trashExplorer.restoreFailed'));
    }
    return (await response.json()) as TrashRestoreResponsePayload;
  }, [t]);

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
      throw await toApiError(response, t('trashExplorer.permanentDeleteFailed'));
    }
    return (await response.json()) as TrashDeleteResponsePayload;
  }, [t]);

  const requestTrashEmpty = useCallback(async (spaceId: number): Promise<TrashEmptyResponsePayload> => {
    const response = await apiFetch(`/api/spaces/${spaceId}/files/trash-empty`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
    });
    if (!response.ok) {
      throw await toApiError(response, t('trashExplorer.emptyFailed'));
    }
    return (await response.json()) as TrashEmptyResponsePayload;
  }, [t]);

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
        title: t('trashExplorer.conflictPolicyTitle'),
        content: (
          <AntSpace direction="vertical" size={12} style={{ width: '100%' }}>
            <Typography.Text>
              {t('trashExplorer.conflictDetected', { count: conflictCount })}
            </Typography.Text>
            <Typography.Text type="secondary">
              {t('trashExplorer.conflictPolicyDescription')}
            </Typography.Text>
            <Radio.Group
              defaultValue="overwrite"
              onChange={(event) => {
                selectedPolicy = event.target.value as TrashConflictPolicy;
              }}
            >
              <AntSpace direction="vertical" size={8}>
                <Radio value="overwrite">{t('trashExplorer.overwrite')}</Radio>
                <Radio value="rename">{t('trashExplorer.rename')}</Radio>
                <Radio value="skip">{t('trashExplorer.skip')}</Radio>
              </AntSpace>
            </Radio.Group>
          </AntSpace>
        ),
        okText: t('trashExplorer.apply'),
        cancelText: t('trashExplorer.stopRestore'),
        onOk: () => {
          settle(selectedPolicy);
        },
        onCancel: () => {
          settle(null);
        },
      });
    });
  }, [modal, t]);

  const handleRestoreConfirm = useCallback(() => {
    if (selectedItems.length === 0) {
      message.warning(t('folderContent.selectRestoreItems'));
      return;
    }

    modal.confirm({
      title: t('folderContent.restoreConfirmTitle'),
      content: t('folderContent.restoreConfirmContent', { count: selectedItems.length }),
      okText: t('folderContent.restore'),
      cancelText: t('folderContent.cancel'),
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
                failedReasons.push(summarizeTrashRestoreFailure(item, t('trashExplorer.restoreFailure')));
              });
            } catch (error) {
              failedReasons.push(error instanceof Error ? error.message : t('trashExplorer.restoreFailed'));
            }
          }

          const conflictCount = Array.from(conflictIdsBySpace.values()).reduce((sum, ids) => sum + ids.length, 0);
          if (conflictCount > 0) {
            const policy = await promptTrashConflictPolicy(conflictCount);
            if (!policy) {
              failedReasons.push(...unresolvedConflicts.map((item) => summarizeTrashRestoreFailure(item, t('trashExplorer.restoreFailure'))));
            } else {
              for (const [spaceId, ids] of conflictIdsBySpace.entries()) {
                try {
                  const retried = await requestTrashRestore(spaceId, ids, policy);
                  succeededCount += retried.succeeded?.length ?? 0;
                  skippedCount += retried.skipped?.length ?? 0;
                  failedReasons.push(...(retried.failed ?? []).map((item) => summarizeTrashRestoreFailure(item, t('trashExplorer.restoreFailure'))));
                } catch (error) {
                  failedReasons.push(error instanceof Error ? error.message : t('trashExplorer.restoreFailed'));
                }
              }
            }
          }

          const failedCount = failedReasons.length;
          const summaryMessage = t('trashExplorer.restoreSummary', {
            succeeded: succeededCount,
            skipped: skippedCount,
            failed: failedCount,
          });
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
    t,
  ]);

  const handleDeleteConfirm = useCallback(() => {
    if (selectedItems.length === 0) {
      message.warning(t('folderContent.selectPermanentDeleteItems'));
      return;
    }

    modal.confirm({
      title: t('folderContent.permanentDeleteConfirmTitle'),
      content: t('folderContent.permanentDeleteConfirmContent', { count: selectedItems.length }),
      okText: t('folderContent.permanentDelete'),
      okType: 'danger',
      cancelText: t('folderContent.cancel'),
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
                failedReasons.push(item.reason ?? t('trashExplorer.permanentDeleteReasonFallback', { id: item.id ?? 'unknown' }));
              });
            } catch (error) {
              failedReasons.push(error instanceof Error ? error.message : t('trashExplorer.permanentDeleteFailed'));
            }
          }

          if (failedReasons.length > 0) {
            const firstFailure = failedReasons[0] ? ` - ${failedReasons[0]}` : '';
            message.warning(`${t('trashExplorer.permanentDeleteSummary', {
              succeeded: succeededCount,
              failed: failedReasons.length,
            })}${firstFailure}`);
          } else {
            message.success(t('trashExplorer.permanentDeleteSuccess', { count: succeededCount }));
          }

          await Promise.all([loadTrashItems(), refreshSelectedFolder()]);
        } finally {
          setProcessing(false);
        }
      },
    });
  }, [loadTrashItems, message, modal, refreshSelectedFolder, requestTrashDelete, selectedItems, t]);

  const handleEmptyConfirm = useCallback(() => {
    if (items.length === 0) {
      message.info(t('folderContent.trashEmptyInfo'));
      return;
    }

    modal.confirm({
      title: t('folderContent.emptyTrashConfirmTitle'),
      content: t('folderContent.emptyTrashConfirmContent', { count: items.length }),
      okText: t('folderContent.emptyTrash'),
      okType: 'danger',
      cancelText: t('folderContent.cancel'),
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
                failedReasons.push(item.reason ?? t('trashExplorer.emptyReasonFallback', { id: item.id ?? 'unknown' }));
              });
            } catch (error) {
              failedReasons.push(error instanceof Error ? error.message : t('trashExplorer.emptyFailed'));
            }
          }

          if (failedReasons.length > 0) {
            const firstFailure = failedReasons[0] ? ` - ${failedReasons[0]}` : '';
            message.warning(`${t('trashExplorer.emptySummary', {
              removed: removedCount,
              failed: failedReasons.length,
            })}${firstFailure}`);
          } else {
            message.success(t('trashExplorer.emptySuccess', { count: removedCount }));
          }

          await Promise.all([loadTrashItems(), refreshSelectedFolder()]);
        } finally {
          setProcessing(false);
        }
      },
    });
  }, [items, loadTrashItems, message, modal, refreshSelectedFolder, requestTrashEmpty, t]);

  const columns = useMemo<TableColumnsType<GlobalTrashItem>>(() => {
    return [
      {
        title: t('trashExplorer.name'),
        dataIndex: 'itemName',
        key: 'itemName',
        ellipsis: true,
        render: (_: unknown, record: GlobalTrashItem) => (
          <AntSpace>
            {record.isDir ? (
              <FolderFilled style={{ color: 'var(--app-folder-icon-color, #415a77)', fontSize: 16 }} />
            ) : (
              <FileTypeIcon filename={record.itemName} size={16} />
            )}
            <span>{record.itemName}</span>
          </AntSpace>
        ),
      },
      {
        title: t('trashExplorer.space'),
        dataIndex: 'spaceName',
        key: 'spaceName',
        width: 140,
        ellipsis: true,
      },
      {
        title: t('trashExplorer.originalPath'),
        dataIndex: 'originalPath',
        key: 'originalPath',
        ellipsis: true,
      },
      {
        title: t('trashExplorer.deletedAt'),
        dataIndex: 'deletedAt',
        key: 'deletedAt',
        width: 180,
        render: (value: string) => formatDate(value),
      },
      {
        title: t('trashExplorer.deletedBy'),
        dataIndex: 'deletedBy',
        key: 'deletedBy',
        width: 120,
      },
      {
        title: t('trashExplorer.size'),
        dataIndex: 'itemSize',
        key: 'itemSize',
        width: 120,
        align: 'right',
        render: (size: number, record: GlobalTrashItem) => (record.isDir ? '-' : formatSize(size)),
      },
    ];
  }, [t]);

  if (spaces.length === 0 && !loading) {
    return (
      <div style={{ height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <Empty description={t('trashExplorer.noConnectedSpace')} />
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
          {t('trashExplorer.title')}
        </Typography.Title>
        <AntSpace wrap>
          <Button icon={<RollbackOutlined />} onClick={handleRestoreConfirm} disabled={selectedItems.length === 0 || loading || processing}>
            {t('trashExplorer.restore')}
          </Button>
          <Button icon={<DeleteOutlined />} danger onClick={handleDeleteConfirm} disabled={selectedItems.length === 0 || loading || processing}>
            {t('trashExplorer.permanentDelete')}
          </Button>
          <Button danger onClick={handleEmptyConfirm} disabled={items.length === 0 || loading || processing}>
            {t('trashExplorer.empty')}
          </Button>
        </AntSpace>
      </div>

      <div style={{ minHeight: 0, flex: 1 }}>
        <Table<GlobalTrashItem>
          size="small"
          rowKey={(record: GlobalTrashItem) => record.rowKey}
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
          locale={{ emptyText: <Empty description={t('trashExplorer.emptyText')} /> }}
        />
      </div>
    </div>
  );
};

export default TrashExplorer;
