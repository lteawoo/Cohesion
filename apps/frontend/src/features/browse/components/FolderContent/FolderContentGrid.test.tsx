import { render } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import type { FileNode } from '../../types';
import FolderContentGrid from './FolderContentGrid';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

const noop = vi.fn();

function buildRecord(overrides: Partial<FileNode>): FileNode {
  return {
    name: 'item',
    path: '/item',
    isDir: false,
    modTime: '2026-03-01T00:00:00.000Z',
    size: 128,
    ...overrides,
  };
}

function renderGrid(dataSource: FileNode[], onItemDoubleClick: (path: string) => void) {
  return render(
    <FolderContentGrid
      dataSource={dataSource}
      loading={false}
      selectedItems={new Set()}
      dragOverFolder={null}
      onItemClick={noop}
      onItemDoubleClick={onItemDoubleClick}
      onContextMenu={noop}
      onItemDragStart={noop}
      onItemDragEnd={noop}
      onFolderDragOver={noop}
      onFolderDragLeave={noop}
      onFolderDrop={noop}
      disableDraggable={true}
    />
  );
}

describe('FolderContentGrid', () => {
  it('calls onItemDoubleClick for file cards', async () => {
    const user = userEvent.setup();
    const onItemDoubleClick = vi.fn();
    const file = buildRecord({ name: 'draft.md', path: '/draft.md', isDir: false });

    const { getByText } = renderGrid([file], onItemDoubleClick);
    await user.dblClick(getByText('draft.md'));

    expect(onItemDoubleClick).toHaveBeenCalledWith('/draft.md');
  });

  it('keeps folder double click callback behavior', async () => {
    const user = userEvent.setup();
    const onItemDoubleClick = vi.fn();
    const folder = buildRecord({ name: 'archive', path: '/archive', isDir: true, size: 0 });

    const { getByText } = renderGrid([folder], onItemDoubleClick);
    await user.dblClick(getByText('archive'));

    expect(onItemDoubleClick).toHaveBeenCalledWith('/archive');
  });
});
