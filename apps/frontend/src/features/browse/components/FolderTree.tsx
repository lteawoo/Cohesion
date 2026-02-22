
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
const PATH_SYNC_PARTIAL_CHILD_THRESHOLD = 40;

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
  selectedKeys?: React.Key[];
  isSearchMode?: boolean;
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

function resolveAncestorExpandKeysFromSelection(key: string): string[] {
  if (!key.startsWith('space-')) {
    return [key];
  }

  const separatorIndex = key.indexOf('::');
  if (separatorIndex < 0) {
    return [key];
  }

  const rootKey = key.substring(0, separatorIndex);
  const relativePath = key.substring(separatorIndex + 2);
  const segments = relativePath.split('/').filter(Boolean);
  if (segments.length === 0) {
    return [rootKey];
  }
  // 선택 노드는 확장하지 않고 상위 경로만 확장해 초기 렌더 부하를 줄입니다.
  const ancestorKeys = segments
    .slice(0, -1)
    .map((_, index) => `${rootKey}::${segments.slice(0, index + 1).join('/')}`);
  return [rootKey, ...ancestorKeys];
}

function resolveNextPathSegmentForKey(selectedKey: string, currentKey: string): string | null {
  const selectedSepIndex = selectedKey.indexOf('::');
  if (selectedSepIndex < 0) {
    return null;
  }

  const selectedRoot = selectedKey.substring(0, selectedSepIndex);
  const currentSepIndex = currentKey.indexOf('::');
  const currentRoot = currentSepIndex >= 0 ? currentKey.substring(0, currentSepIndex) : currentKey;
  if (selectedRoot !== currentRoot) {
    return null;
  }

  const selectedSegments = selectedKey
    .substring(selectedSepIndex + 2)
    .split('/')
    .filter(Boolean);
  const currentSegments = currentSepIndex >= 0
    ? currentKey.substring(currentSepIndex + 2).split('/').filter(Boolean)
    : [];

  if (currentSegments.length >= selectedSegments.length) {
    return null;
  }
  for (let index = 0; index < currentSegments.length; index += 1) {
    if (currentSegments[index] !== selectedSegments[index]) {
      return null;
    }
  }
  return selectedSegments[currentSegments.length] ?? null;
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

function mergeExpandedKeys(prev: React.Key[], keysToExpand: string[]): React.Key[] {
  if (keysToExpand.length === 0) {
    return prev;
  }
  const merged = new Set(prev);
  const beforeSize = merged.size;
  for (const key of keysToExpand) {
    merged.add(key);
  }
  if (merged.size === beforeSize) {
    return prev;
  }
  return Array.from(merged);
}

function stripRootChildren(list: TreeDataNode[]): TreeDataNode[] {
  let changed = false;
  const next = list.map((node) => {
    if (!node.children || node.children.length === 0) {
      return node;
    }
    changed = true;
    return {
      ...node,
      children: undefined,
    };
  });
  return changed ? next : list;
}

const FolderTree: React.FC<FolderTreeProps> = ({
  onSelect,
  rootPath,
  rootName,
  showBaseDirectories = false,
  onSpaceDelete,
  selectedKeys,
  isSearchMode = false,
}) => {
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
  const treeDataRef = useRef<TreeDataNode[]>([]);
  const partialLoadedKeysRef = useRef<Set<string>>(new Set());
  const pathSyncModeRef = useRef(false);
  const pathSyncTargetKeyRef = useRef<string | null>(null);
  const prevIsSearchModeRef = useRef(isSearchMode);
  const loadingKeysRef = useRef<Set<React.Key>>(new Set());
  const handledRefreshVersionRef = useRef(0);
  const { isLoading, error: browseError, fetchBaseDirectories, fetchDirectoryContents, fetchSpaceDirectoryContents } = useBrowseApi();
  const openContextMenu = useContextMenuStore((state) => state.openContextMenu);

  useEffect(() => {
    treeDataRef.current = treeData;
  }, [treeData]);

  useEffect(() => {
    const previousIsSearchMode = prevIsSearchModeRef.current;
    const selectedKey = selectedKeys?.length ? String(selectedKeys[0]) : null;

    if (previousIsSearchMode && !isSearchMode && selectedKey) {
      pathSyncModeRef.current = true;
      pathSyncTargetKeyRef.current = selectedKey;
      partialLoadedKeysRef.current.clear();
    }

    if (isSearchMode) {
      pathSyncModeRef.current = false;
      pathSyncTargetKeyRef.current = null;
      partialLoadedKeysRef.current.clear();
    } else if (
      pathSyncModeRef.current &&
      pathSyncTargetKeyRef.current &&
      selectedKey &&
      pathSyncTargetKeyRef.current !== selectedKey
    ) {
      pathSyncModeRef.current = false;
      pathSyncTargetKeyRef.current = null;
    }

    prevIsSearchModeRef.current = isSearchMode;
  }, [isSearchMode, selectedKeys]);

  useEffect(() => {
    if (!pathSyncModeRef.current) {
      return;
    }
    const targetKey = pathSyncTargetKeyRef.current;
    if (!targetKey) {
      return;
    }
    if (findNodeByKey(treeData, targetKey)) {
      pathSyncModeRef.current = false;
      pathSyncTargetKeyRef.current = null;
    }
  }, [treeData]);

  const loadChildrenForKey = useCallback(
    async (key: React.Key) => {
      if (loadedKeys.includes(key)) {
        return;
      }
      if (loadingKeysRef.current.has(key)) {
        return;
      }
      if (!findNodeByKey(treeDataRef.current, key)) {
        return;
      }

      loadingKeysRef.current.add(key);
      try {
        const keyStr = key as string;
        let path: string;
        let spacePrefix = '';

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
          const spaceId = parseInt(spacePrefix.replace('space-', ''), 10);
          const currentSpace = spaces?.find((s) => s.id === spaceId);
          if (!currentSpace) {
            return;
          }
          contents = await fetchSpaceDirectoryContents(spaceId, path);
        } else {
          path = keyStr;
          contents = await fetchDirectoryContents(path, showBaseDirectories);
        }

        const directoryNodes = (contents ?? [])
          .filter((node) => node.isDir);
        const syncTargetKey = pathSyncModeRef.current ? pathSyncTargetKeyRef.current : null;
        const nextSegment = syncTargetKey ? resolveNextPathSegmentForKey(syncTargetKey, keyStr) : null;
        const focusedNodes = nextSegment
          ? directoryNodes.filter((node) => node.name === nextSegment)
          : directoryNodes;
        const shouldKeepPathOnly =
          focusedNodes.length > 0 &&
          focusedNodes.length < directoryNodes.length &&
          directoryNodes.length > PATH_SYNC_PARTIAL_CHILD_THRESHOLD;
        const effectiveNodes = shouldKeepPathOnly ? focusedNodes : directoryNodes;

        if (shouldKeepPathOnly) {
          partialLoadedKeysRef.current.add(keyStr);
        } else {
          partialLoadedKeysRef.current.delete(keyStr);
        }

        const directoryChildren = effectiveNodes
          .map((node) => ({
            title: node.name,
            key: spacePrefix ? `${spacePrefix}::${node.path}` : node.path,
            isLeaf: false,
          }));
        setTreeData((origin) => updateTreeData(origin, key, directoryChildren));
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

  // 검색 모드에서는 트리 내부 상태를 루트만 남기고 정리해 누적 비용을 줄입니다.
  useEffect(() => {
    if (showBaseDirectories || rootPath || !isSearchMode) {
      return;
    }
    pathSyncModeRef.current = false;
    pathSyncTargetKeyRef.current = null;
    partialLoadedKeysRef.current.clear();
    setExpandedKeys((prev) => (prev.length === 0 ? prev : []));
    setLoadedKeys((prev) => (prev.length === 0 ? prev : []));
    setTreeData((prev) => stripRootChildren(prev));
  }, [isSearchMode, rootPath, showBaseDirectories]);

  // selectedKeys 기준으로 자동 확장합니다.
  useEffect(() => {
    if (showBaseDirectories || rootPath || isSearchMode) {
      return;
    }
    if (!selectedKeys || selectedKeys.length === 0) {
      return;
    }
    const selectedKey = String(selectedKeys[0]);
    const keysToExpand = resolveAncestorExpandKeysFromSelection(selectedKey);
    setExpandedKeys((prev) => mergeExpandedKeys(prev, keysToExpand));
  }, [isSearchMode, rootPath, selectedKeys, showBaseDirectories]);

  const handleExpand: DirectoryTreeProps['onExpand'] = (keys: React.Key[]) => {
    const keysToHydrate = keys
      .map((key) => String(key))
      .filter((key) => partialLoadedKeysRef.current.has(key));

    if (keysToHydrate.length > 0) {
      pathSyncModeRef.current = false;
      pathSyncTargetKeyRef.current = null;
      for (const key of keysToHydrate) {
        partialLoadedKeysRef.current.delete(key);
      }
      const keySet = new Set(keysToHydrate);
      setLoadedKeys((prev) => prev.filter((key) => !keySet.has(String(key))));
    }

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
        const spaceId = parseInt(spacePrefix.replace('space-', ''), 10);
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
      const spaceId = parseInt(key.replace('space-', ''), 10);
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
        selectedKeys={selectedKeys}
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
