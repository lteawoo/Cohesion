
import React, { useCallback, useEffect, useRef, useState } from 'react';
import { Tree, Spin, Alert, Button } from 'antd';
import type { GetProps, MenuProps } from 'antd';
import type { EventDataNode } from 'antd/es/tree';
import { useContextMenuStore } from '@/stores/contextMenuStore';
import { useSpaceStore } from '@/stores/spaceStore';
import { useBrowseStore } from '@/stores/browseStore';
import type { TreeInvalidationTarget } from '@/stores/browseStore';
import { DeleteOutlined } from '@ant-design/icons';
import { useBrowseApi } from '../hooks/useBrowseApi';
import type { FileNode, TreeDataNode } from '../types';
import type { Space } from '@/features/space/types';

type DirectoryTreeProps = GetProps<typeof Tree.DirectoryTree>;

// API 응답(FileNode)을 Ant Design Tree가 요구하는 형식(TreeDataNode)으로 변환합니다.
const convertToFileTreeData = (nodes: FileNode[]): TreeDataNode[] => {
  if (!nodes) return [];

  return nodes
    .filter(node => node.isDir) // 폴더만 필터링합니다.
    .map(node => ({
      title: node.name,
      key: node.path,
      isLeaf: false, 
    }));
};

interface FolderTreeProps {
  onSelect: (path: string, space?: Space) => void;
  rootPath?: string;
  rootName?: string;
  showBaseDirectories?: boolean;
  onSpaceDelete?: (space: Space) => void;
}

function resolveTargetKey(
  target: TreeInvalidationTarget,
  spaces: Space[],
  rootPath?: string,
  showBaseDirectories?: boolean
): string | null {
  if (showBaseDirectories) {
    return target.path;
  }

  if (rootPath) {
    return target.path;
  }

  if (!target.spaceId) {
    return null;
  }

  const space = spaces.find((item) => item.id === target.spaceId);
  if (!space) {
    return null;
  }

  if (!target.path) {
    return `space-${space.id}`;
  }

  return `space-${space.id}::${target.path}`;
}

function findNodeByKey(list: TreeDataNode[], key: React.Key): TreeDataNode | null {
  for (const node of list) {
    if (node.key === key) {
      return node;
    }
    if (node.children) {
      const childNode = findNodeByKey(node.children, key);
      if (childNode) {
        return childNode;
      }
    }
  }
  return null;
}

function collectNodeKeys(node: TreeDataNode): React.Key[] {
  const keys: React.Key[] = [node.key];
  if (!node.children) {
    return keys;
  }

  for (const child of node.children) {
    keys.push(...collectNodeKeys(child));
  }
  return keys;
}

function clearChildrenByKeys(list: TreeDataNode[], keys: Set<string>): TreeDataNode[] {
  return list.map((node) => {
    const nodeKey = String(node.key);
    if (keys.has(nodeKey)) {
      return {
        ...node,
        children: undefined,
      };
    }
    if (node.children) {
      return {
        ...node,
        children: clearChildrenByKeys(node.children, keys),
      };
    }
    return node;
  });
}

const FolderTree: React.FC<FolderTreeProps> = ({ onSelect, rootPath, rootName, showBaseDirectories = false, onSpaceDelete }) => {
  const spaces = useSpaceStore((state) => state.spaces);
  const spaceError = useSpaceStore((state) => state.error);
  const fetchSpaces = useSpaceStore((state) => state.fetchSpaces);
  const treeRefreshVersion = useBrowseStore((state) => state.treeRefreshVersion);
  const treeInvalidationTargets = useBrowseStore((state) => state.treeInvalidationTargets);
  const selectedPath = useBrowseStore((state) => state.selectedPath);
  const selectedSpace = useBrowseStore((state) => state.selectedSpace);
  const [treeData, setTreeData] = useState<TreeDataNode[]>([]);
  const [loadedKeys, setLoadedKeys] = useState<React.Key[]>([]);
  const [expandedKeys, setExpandedKeys] = useState<React.Key[]>([]);
  const [reloadNonce, setReloadNonce] = useState(0);
  const loadingKeysRef = useRef<Set<React.Key>>(new Set());
  const handledRefreshVersionRef = useRef(0);
  const { isLoading, error: browseError, fetchBaseDirectories, fetchDirectoryContents, fetchSpaceDirectoryContents } = useBrowseApi();
  const openContextMenu = useContextMenuStore((state) => state.openContextMenu);

  const loadChildrenForKey = useCallback(
    async (key: React.Key) => {
      if (loadedKeys.includes(key)) {
        return;
      }
      if (loadingKeysRef.current.has(key)) {
        return;
      }

      loadingKeysRef.current.add(key);
      try {
        let path: string;
        let spacePrefix = '';
        const keyStr = key as string;

        let contents;
        if (keyStr.startsWith('space-')) {
          const sepIndex = keyStr.indexOf('::');
          if (sepIndex >= 0) {
            spacePrefix = keyStr.substring(0, sepIndex);
            path = keyStr.substring(sepIndex + 2);
          } else {
            spacePrefix = keyStr;
            path = '';
          }
          const spaceId = parseInt(spacePrefix.replace('space-', ''));
          const space = spaces?.find((s) => s.id === spaceId);
          if (!space) {
            return;
          }
          contents = await fetchSpaceDirectoryContents(spaceId, path);
        } else {
          path = keyStr;
          contents = await fetchDirectoryContents(path, showBaseDirectories);
        }

        const newChildren = (contents ?? [])
          .filter((node) => node.isDir)
          .map((node) => ({
            title: node.name,
            key: spacePrefix ? `${spacePrefix}::${node.path}` : node.path,
            isLeaf: false,
          }));

        setTreeData((origin) => updateTreeData(origin, key, newChildren));
        setLoadedKeys((prev) => (prev.includes(key) ? prev : [...prev, key]));
      } catch {
        // Error state is managed by useBrowseApi.
      } finally {
        loadingKeysRef.current.delete(key);
      }
    },
    [fetchDirectoryContents, fetchSpaceDirectoryContents, showBaseDirectories, spaces, loadedKeys]
  );

  // 초기 트리 데이터를 로드합니다.
  useEffect(() => {
    let isMounted = true;
    const loadInitialData = async () => {
      try {
        // showBaseDirectories가 true면 파일 시스템을 최우선으로 표시
        if (showBaseDirectories) {
          // base directories 로드 (Space 생성 모달 등에서 사용)
          const baseDirs = await fetchBaseDirectories();
          if (isMounted) {
            setTreeData(convertToFileTreeData(baseDirs));
            setLoadedKeys([]);
            setExpandedKeys([]);
          }
        } else if (rootPath && rootName) {
          // 단일 Space를 루트 노드로 표시
          if (isMounted) {
            setTreeData([{
              title: rootName,
              key: rootPath,
              isLeaf: false,
            }]);
            setLoadedKeys([]);
            setExpandedKeys([]);
          }
        } else if (spaces && spaces.length > 0) {
          // Spaces를 루트 노드로 표시 (메인 사이드바에서 사용)
          if (isMounted) {
            setTreeData(spaces.map(space => ({
              title: space.space_name,
              key: `space-${space.id}`,
              isLeaf: false,
            })));
            setLoadedKeys([]);
            setExpandedKeys([]);
          }
        } else {
          // 트리를 비움
          if (isMounted) {
            setTreeData([]);
            setLoadedKeys([]);
            setExpandedKeys([]);
          }
        }
      } catch {
        if (isMounted) {
          setTreeData([]);
          setLoadedKeys([]);
          setExpandedKeys([]);
        }
      }
    };
    loadInitialData();
    return () => { isMounted = false; };
  }, [rootPath, rootName, showBaseDirectories, spaces, fetchBaseDirectories, reloadNonce]);

  // 트리 전체 초기화 요청(legacy fallback)
  useEffect(() => {
    if (
      treeRefreshVersion === 0 ||
      treeInvalidationTargets.length > 0 ||
      handledRefreshVersionRef.current === treeRefreshVersion
    ) {
      return;
    }
    handledRefreshVersionRef.current = treeRefreshVersion;
    setLoadedKeys([]);
    setExpandedKeys([]);
  }, [treeRefreshVersion, treeInvalidationTargets.length]);

  // 작업 영향 경로만 부분 갱신합니다.
  useEffect(() => {
    if (
      treeRefreshVersion === 0 ||
      treeInvalidationTargets.length === 0 ||
      handledRefreshVersionRef.current === treeRefreshVersion
    ) {
      return;
    }
    handledRefreshVersionRef.current = treeRefreshVersion;

    const targetKeySet = new Set(
      treeInvalidationTargets
        .map((target) => resolveTargetKey(target, spaces, rootPath, showBaseDirectories))
        .filter((target): target is string => Boolean(target))
    );

    if (targetKeySet.size === 0) {
      return;
    }

    const resetKeySet = new Set<React.Key>();
    for (const targetKey of targetKeySet) {
      const targetNode = findNodeByKey(treeData, targetKey);
      if (!targetNode) {
        continue;
      }
      collectNodeKeys(targetNode).forEach((key) => resetKeySet.add(key));
    }

    setTreeData((prev) => clearChildrenByKeys(prev, targetKeySet));
    setLoadedKeys((prev) => prev.filter((key) => !resetKeySet.has(key)));
    setExpandedKeys((prev) =>
      prev.filter((key) => {
        const keyStr = String(key);
        return !Array.from(targetKeySet).some((target) => keyStr !== target && keyStr.startsWith(`${target}::`));
      })
    );

    const reloadPromises = expandedKeys
      .filter((key) => targetKeySet.has(String(key)))
      .map((key) => loadChildrenForKey(key));
    void Promise.all(reloadPromises);
  }, [
    expandedKeys,
    loadChildrenForKey,
    rootPath,
    showBaseDirectories,
    spaces,
    treeData,
    treeInvalidationTargets,
    treeRefreshVersion,
  ]);

  // 확장된 노드만 단일 경로로 lazy load합니다.
  useEffect(() => {
    for (const key of expandedKeys) {
      if (!loadedKeys.includes(key) && !loadingKeysRef.current.has(key)) {
        void loadChildrenForKey(key);
      }
    }
  }, [expandedKeys, loadedKeys, loadChildrenForKey]);

  const handleExpand: DirectoryTreeProps['onExpand'] = (keys: React.Key[]) => {
    setExpandedKeys(keys);
  };

  const handleSelect: DirectoryTreeProps['onSelect'] = (
    keys: React.Key[],
    info: Parameters<NonNullable<DirectoryTreeProps['onSelect']>>[1]
  ) => {
    if (keys.length > 0) {
      const key = keys[0] as string;
      const isLeaf = info.node.isLeaf;
      const isSameSelection = (nextPath: string, nextSpace?: Space) =>
        selectedPath === nextPath && (selectedSpace?.id ?? null) === (nextSpace?.id ?? null);

      // 클릭 시에는 "열기만" 수행하고(닫힘 토글 금지), 닫기는 스위처 아이콘에서만 처리합니다.
      if (!isLeaf && !expandedKeys.includes(key)) {
        setExpandedKeys((prev) => (prev.includes(key) ? prev : [...prev, key]));
      }

      if (key.startsWith('space-')) {
        const sepIndex = key.indexOf('::');
        const spacePrefix = sepIndex >= 0 ? key.substring(0, sepIndex) : key;
        const spaceId = parseInt(spacePrefix.replace('space-', ''));
        const space = spaces?.find(s => s.id === spaceId);

        if (sepIndex >= 0) {
          // Space 하위 노드 선택 — 실제 경로와 space 정보 전달
          const nextPath = key.substring(sepIndex + 2);
          if (!isSameSelection(nextPath, space)) {
            onSelect(nextPath, space);
          }
        } else {
          // Space 루트 노드 선택
          if (space) {
            if (!isSameSelection('', space)) {
              onSelect('', space);
            }
          }
        }
      } else {
        if (!isSameSelection(key)) {
          onSelect(key);
        }
      }
    }
  };

  // 우클릭 핸들러
  const handleRightClick: DirectoryTreeProps['onRightClick'] = ({ event, node }: { event: React.MouseEvent; node: EventDataNode<TreeDataNode> }) => {
    if (!onSpaceDelete) return;

    const key = node.key as string;

    // Space 루트 노드인지 확인 (space-{id} 형식, :: 없음)
    if (key.startsWith('space-') && !key.includes('::')) {
      const spaceId = parseInt(key.replace('space-', ''));
      const space = spaces?.find(s => s.id === spaceId);

      if (space) {
        event.preventDefault();

        const menuItems: MenuProps['items'] = [
          {
            key: 'delete',
            icon: <DeleteOutlined />,
            label: '삭제',
            danger: true,
            onClick: () => {
              onSpaceDelete?.(space);
            },
          },
        ];

        openContextMenu(event.clientX, event.clientY, menuItems);
      }
    }
  };


  const retryTreeLoad = () => {
    if (!rootPath && !showBaseDirectories) {
      void fetchSpaces();
    }
    setReloadNonce((prev) => prev + 1);
  };

  if (!rootPath && !showBaseDirectories && (!spaces || spaces.length === 0)) {
    if (spaceError) {
      return (
        <div style={{ padding: '16px 12px', display: 'flex', flexDirection: 'column', gap: 8 }}>
          <Alert
            type="error"
            showIcon
            message="Space 목록을 불러오지 못했습니다."
            description={spaceError.message}
          />
          <Button size="small" onClick={retryTreeLoad}>다시 시도</Button>
        </div>
      );
    }
    return (
      <div style={{ padding: '20px', textAlign: 'center', color: 'var(--ant-color-text-secondary, #778da9)', fontSize: '12px' }}>
        Space를 선택하세요
      </div>
    );
  }

  if (isLoading && treeData.length === 0) {
    return (
      <div style={{ padding: '20px', textAlign: 'center' }}>
        <Spin />
      </div>
    );
  }

  if (browseError && treeData.length === 0) {
    return (
      <div style={{ padding: '16px 12px', display: 'flex', flexDirection: 'column', gap: 8 }}>
        <Alert
          type="error"
          showIcon
          message="폴더 트리를 불러오지 못했습니다."
          description={browseError.message}
        />
        <Button size="small" onClick={retryTreeLoad}>다시 시도</Button>
      </div>
    );
  }

  return (
    <>
      {browseError && treeData.length > 0 && (
        <div style={{ padding: '0 12px 8px' }}>
          <Alert
            type="warning"
            showIcon
            message="일부 폴더를 불러오지 못했습니다."
            description={browseError.message}
            action={<Button size="small" onClick={retryTreeLoad}>재시도</Button>}
          />
        </div>
      )}
      <Tree.DirectoryTree
        className="folder-tree"
        onSelect={handleSelect}
        onExpand={handleExpand}
        onRightClick={handleRightClick}
        treeData={treeData}
        loadedKeys={loadedKeys}
        expandedKeys={expandedKeys}
        showIcon={false}
        switcherIcon={({ expanded, isLeaf }) => {
          if (isLeaf) {
            return <span className="folder-tree-switcher-placeholder" aria-hidden="true" />;
          }
          return (
            <span
              className="material-symbols-rounded folder-tree-switcher-icon"
              style={{
                fontVariationSettings: '"FILL" 1, "wght" 500, "GRAD" 0, "opsz" 20',
              }}
              aria-hidden="true"
            >
              {expanded ? 'folder_open' : 'folder'}
            </span>
          );
        }}
        expandAction={false}
      />
    </>
  );
}

function updateTreeData(list: TreeDataNode[], key: React.Key, children: TreeDataNode[]): TreeDataNode[] {
  return list.map(node => {
    if (node.key === key) {
      return {
        ...node,
        children,
      };
    }
    if (node.children) {
      return {
        ...node,
        children: updateTreeData(node.children, key, children),
      };
    }
    return node;
  });
}

export default FolderTree;
