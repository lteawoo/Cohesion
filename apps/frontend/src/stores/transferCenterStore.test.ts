import { beforeEach, describe, expect, it } from 'vitest';
import { useTransferCenterStore } from './transferCenterStore';

describe('transferCenterStore', () => {
  beforeEach(() => {
    window.sessionStorage.clear();
    useTransferCenterStore.persist.clearStorage();
    useTransferCenterStore.getState().reset();
  });

  it('keeps active transfers ahead of terminal history', () => {
    useTransferCenterStore.getState().upsertTransfer({
      id: 'terminal',
      kind: 'download',
      name: 'done.pdf',
      status: 'handed_off',
      updatedAt: 10,
    });
    useTransferCenterStore.getState().upsertTransfer({
      id: 'active',
      kind: 'upload',
      name: 'active.bin',
      status: 'uploading',
      loaded: 8,
      total: 16,
      progressPercent: 50,
      updatedAt: 5,
    });

    expect(useTransferCenterStore.getState().transfers.map((transfer) => transfer.id)).toEqual([
      'active',
      'terminal',
    ]);
  });

  it('evicts the oldest clearable terminal entries beyond the session cap', () => {
    Array.from({ length: 25 }).forEach((_, index) => {
      useTransferCenterStore.getState().upsertTransfer({
        id: `download-${index + 1}`,
        kind: 'download',
        name: `download-${index + 1}.zip`,
        status: 'handed_off',
        updatedAt: index + 1,
      });
    });

    const transfers = useTransferCenterStore.getState().transfers;
    expect(transfers).toHaveLength(24);
    expect(transfers.some((transfer) => transfer.id === 'download-1')).toBe(false);
    expect(transfers[0]?.id).toBe('download-25');
  });

  it('persists serializable transfer metadata in session storage', () => {
    useTransferCenterStore.getState().upsertTransfer({
      id: 'archive-1',
      kind: 'archive',
      name: 'docs.zip',
      status: 'running',
      jobId: 'job-1',
      spaceId: 1,
      processedItems: 1,
      totalItems: 4,
      processedSourceBytes: 64,
      totalSourceBytes: 256,
      updatedAt: 10,
    });

    const raw = window.sessionStorage.getItem('cohesion-transfer-center-v1');
    expect(raw).toBeTruthy();
    const persisted = JSON.parse(raw ?? '{}');
    expect(persisted.state.transfers[0]).toMatchObject({
      id: 'archive-1',
      kind: 'archive',
      jobId: 'job-1',
      spaceId: 1,
      status: 'running',
    });
  });
});
