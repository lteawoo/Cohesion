import { render } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import FolderTree from './FolderTree';
import type { FileNode } from '../types';

const h = vi.hoisted(() => {
  const fetchBaseDirectories = vi.fn();
  const fetchDirectoryContents = vi.fn();
  const fetchSpaceDirectoryContents = vi.fn();
  const fetchSpaces = vi.fn();
  const openContextMenu = vi.fn();
  const onSelect = vi.fn();

  const spaceState = {
    spaces: [],
    error: null as Error | null,
    fetchSpaces,
  };

  const browseState = {
    treeRefreshVersion: 0,
    treeInvalidationTargets: [] as Array<{ path: string; spaceId?: number }>,
    selectedPath: '',
    selectedSpace: undefined,
  };

  return {
    fetchBaseDirectories,
    fetchDirectoryContents,
    fetchSpaceDirectoryContents,
    fetchSpaces,
    openContextMenu,
    onSelect,
    spaceState,
    browseState,
  };
});

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

vi.mock('@/stores/spaceStore', () => ({
  useSpaceStore: (selector: (state: typeof h.spaceState) => unknown) => selector(h.spaceState),
}));

vi.mock('@/stores/browseStore', () => ({
  useBrowseStore: (selector: (state: typeof h.browseState) => unknown) => selector(h.browseState),
}));

vi.mock('@/stores/contextMenuStore', () => ({
  useContextMenuStore: (selector: (state: { openContextMenu: typeof h.openContextMenu }) => unknown) =>
    selector({ openContextMenu: h.openContextMenu }),
}));

vi.mock('../hooks/useBrowseApi', () => ({
  useBrowseApi: () => ({
    isLoading: false,
    error: null,
    fetchBaseDirectories: h.fetchBaseDirectories,
    fetchDirectoryContents: h.fetchDirectoryContents,
    fetchSpaceDirectoryContents: h.fetchSpaceDirectoryContents,
  }),
}));

vi.mock('@ant-design/icons', () => ({
  DeleteOutlined: () => null,
}));

vi.mock('antd', () => {
  type MockTreeNode = {
    title: unknown;
    key: string;
    isLeaf: boolean;
    children?: MockTreeNode[];
  };

  const DirectoryTree = ({
    treeData = [],
    expandedKeys = [],
    onExpand,
    onSelect,
  }: {
    treeData?: MockTreeNode[];
    expandedKeys?: Array<string>;
    onExpand?: (keys: string[]) => void;
    onSelect?: (keys: string[], info: { node: { key: string; isLeaf: boolean } }) => void;
  }) => {
    const expandedKeySet = new Set((expandedKeys ?? []).map((key) => String(key)));

    const toggleNode = (key: string) => {
      const next = new Set(expandedKeySet);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      onExpand?.(Array.from(next));
    };

    const selectNode = (key: string, isLeaf: boolean) => {
      onSelect?.([key], { node: { key, isLeaf } });
    };

    const renderNodes = (nodes: MockTreeNode[]) =>
      nodes.map((node) => {
        const key = String(node.key);
        return (
          <div key={key} data-testid="tree-node" data-key={key} data-title={String(node.title)}>
            <button type="button" data-testid="tree-switcher" data-key={key} onClick={() => toggleNode(key)}>
              toggle
            </button>
            <button type="button" data-testid="tree-selector" data-key={key} onClick={() => selectNode(key, node.isLeaf)}>
              {String(node.title)}
            </button>
            {expandedKeySet.has(key) && node.children ? <div>{renderNodes(node.children)}</div> : null}
          </div>
        );
      });

    return <div data-testid="tree">{renderNodes(treeData)}</div>;
  };

  return {
    Tree: { DirectoryTree },
    Spin: () => <div data-testid="spin" />,
    Alert: ({ message, description, action }: { message?: string; description?: string; action?: ReactNode }) => (
      <div data-testid="alert">
        {message}
        {description}
        {action}
      </div>
    ),
    Button: ({ children, onClick }: { children?: ReactNode; onClick?: () => void }) => (
      <button type="button" onClick={onClick}>
        {children ?? 'button'}
      </button>
    ),
  };
});

function directory(path: string, name: string): FileNode {
  return {
    name,
    path,
    isDir: true,
    modTime: '2026-03-02T00:00:00Z',
    size: 0,
  };
}

function getSwitcherByKey(key: string): HTMLButtonElement {
  const switcher = Array.from(document.querySelectorAll<HTMLButtonElement>('[data-testid="tree-switcher"]'))
    .find((element: HTMLButtonElement) => element.getAttribute('data-key') === key);
  if (!switcher) {
    throw new Error(`switcher not found: ${key}`);
  }
  return switcher as HTMLButtonElement;
}

function getSelectorByKey(key: string): HTMLButtonElement {
  const selector = Array.from(document.querySelectorAll<HTMLButtonElement>('[data-testid="tree-selector"]'))
    .find((element: HTMLButtonElement) => element.getAttribute('data-key') === key);
  if (!selector) {
    throw new Error(`selector not found: ${key}`);
  }
  return selector as HTMLButtonElement;
}

function getVisibleNodeKeys(): string[] {
  return Array.from(document.querySelectorAll<HTMLDivElement>('[data-testid="tree-node"]'))
    .map((node: HTMLDivElement) => node.getAttribute('data-key'))
    .filter((key): key is string => Boolean(key));
}

async function waitForNodeKey(key: string): Promise<void> {
  await vi.waitFor(() => {
    const keys = getVisibleNodeKeys();
    expect(keys).toContain(key);
  });
}

describe('FolderTree (space directory mode)', () => {
  beforeEach(() => {
    vi.clearAllMocks();

    h.fetchBaseDirectories.mockResolvedValue([
      directory('/Users/tester', 'Home'),
      directory('/', '/'),
      directory('/dev', '/dev'),
    ]);

    h.fetchDirectoryContents.mockImplementation(async (path: string) => {
      if (path === '/') {
        return [
          directory('/dev', 'dev'),
          directory('/usr', 'usr'),
        ];
      }
      return [];
    });
  });

  it('keeps unique keys and stable child count while repeatedly toggling root slash', async () => {
    const user = userEvent.setup();
    render(<FolderTree onSelect={h.onSelect} showBaseDirectories />);

    await vi.waitFor(() => {
      expect(h.fetchBaseDirectories).toHaveBeenCalledTimes(1);
    });
    await waitForNodeKey('base::/');

    for (let index = 0; index < 6; index += 1) {
      await user.click(getSwitcherByKey('base::/'));
      const keys = getVisibleNodeKeys();
      expect(new Set(keys).size).toBe(keys.length);

      const visibleDevChildren = Array.from(document.querySelectorAll<HTMLDivElement>('[data-testid="tree-node"]'))
        .filter((node: HTMLDivElement) => node.getAttribute('data-title') === 'dev').length;

      if (index % 2 === 0) {
        expect(visibleDevChildren).toBe(1);
      } else {
        expect(visibleDevChildren).toBe(0);
      }
    }

    expect(h.fetchDirectoryContents).toHaveBeenCalledTimes(1);
    expect(h.fetchDirectoryContents).toHaveBeenCalledWith('/');
  });

  it('decodes scoped keys before forwarding selection path', async () => {
    const user = userEvent.setup();
    render(<FolderTree onSelect={h.onSelect} showBaseDirectories />);

    await vi.waitFor(() => {
      expect(h.fetchBaseDirectories).toHaveBeenCalledTimes(1);
    });
    await waitForNodeKey('base::/');

    await user.click(getSelectorByKey('base::/'));
    expect(h.onSelect).toHaveBeenLastCalledWith('/');

    await waitForNodeKey('system::/dev');
    await user.click(getSelectorByKey('system::/dev'));
    expect(h.onSelect).toHaveBeenLastCalledWith('/dev');
  });
});
