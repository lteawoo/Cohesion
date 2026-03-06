import { beforeEach, describe, expect, it } from 'vitest';
import type { FileNode } from '@/features/browse/types';
import type { Space } from '@/features/space/types';
import { useBrowseStore } from './browseStore';

const sampleContent: FileNode[] = [
  {
    name: 'report.txt',
    path: 'docs/report.txt',
    isDir: false,
    modTime: '2026-03-06T10:00:00.000Z',
    size: 128,
  },
];

describe('useBrowseStore.reconcileSelectedSpace', () => {
  beforeEach(() => {
    useBrowseStore.setState({
      selectedPath: 'docs',
      selectedSpace: { id: 1, space_name: 'Alpha' },
      content: sampleContent,
      isLoading: false,
      error: null,
      treeRefreshVersion: 0,
      treeInvalidationTargets: [],
      trashOpenRequest: null,
    });
  });

  it('replaces selectedSpace with the latest space object by id', () => {
    const refreshedSpaces: Space[] = [{ id: 1, space_name: 'Alpha Renamed' }];

    useBrowseStore.getState().reconcileSelectedSpace(refreshedSpaces);

    expect(useBrowseStore.getState().selectedSpace?.space_name).toBe('Alpha Renamed');
    expect(useBrowseStore.getState().selectedPath).toBe('docs');
    expect(useBrowseStore.getState().content).toEqual(sampleContent);
  });

  it('clears browse state when the selected space disappears from the shared list', () => {
    useBrowseStore.getState().reconcileSelectedSpace([]);

    expect(useBrowseStore.getState().selectedSpace).toBeUndefined();
    expect(useBrowseStore.getState().selectedPath).toBe('');
    expect(useBrowseStore.getState().content).toEqual([]);
  });
});
