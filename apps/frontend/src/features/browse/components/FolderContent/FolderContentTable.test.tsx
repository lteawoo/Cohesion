import { render } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import type { FileNode } from '../../types';
import FolderContentTable from './FolderContentTable';

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

function renderTable(
  dataSource: FileNode[],
  onItemDoubleClick: (path: string) => void,
  showActions = false
) {
  return render(
    <FolderContentTable
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
      canWriteFiles={true}
      showActions={showActions}
    />
  );
}

describe('FolderContentTable', () => {
  it('calls onItemDoubleClick for file rows', async () => {
    const user = userEvent.setup();
    const onItemDoubleClick = vi.fn();
    const file = buildRecord({ name: 'notes.txt', path: '/notes.txt', isDir: false });

    const { getByText } = renderTable([file], onItemDoubleClick);
    await user.dblClick(getByText('notes.txt'));

    expect(onItemDoubleClick).toHaveBeenCalledWith('/notes.txt');
  });

  it('keeps folder double click navigation callback behavior', async () => {
    const user = userEvent.setup();
    const onItemDoubleClick = vi.fn();
    const folder = buildRecord({ name: 'docs', path: '/docs', isDir: true, size: 0 });

    const { getByText } = renderTable([folder], onItemDoubleClick);
    await user.dblClick(getByText('docs'));

    expect(onItemDoubleClick).toHaveBeenCalledWith('/docs');
  });

  it('does not trigger row activation when double-clicking action button', async () => {
    const user = userEvent.setup();
    const onItemDoubleClick = vi.fn();
    const file = buildRecord({ name: 'notes.txt', path: '/notes.txt', isDir: false });

    const { getByLabelText } = renderTable([file], onItemDoubleClick, true);
    await user.dblClick(getByLabelText('browseMenu.more'));

    expect(onItemDoubleClick).not.toHaveBeenCalled();
  });
});
