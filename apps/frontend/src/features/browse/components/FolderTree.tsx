
import React, { useEffect, useRef, useState } from 'react';
import { Tree, Spin } from 'antd';
import type { GetProps, MenuProps } from 'antd';
import { useContextMenu } from '@/contexts/ContextMenuContext';
import { FolderOutlined, DeleteOutlined } from '@ant-design/icons';
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
  spaces?: Space[];
  onSpaceDelete?: (space: Space) => void;
}

const FolderTree: React.FC<FolderTreeProps> = ({ onSelect, rootPath, rootName, showBaseDirectories = false, spaces, onSpaceDelete }) => {
  const [treeData, setTreeData] = useState<TreeDataNode[]>([]);
  const [loadedKeys, setLoadedKeys] = useState<React.Key[]>([]);
  const [expandedKeys, setExpandedKeys] = useState<React.Key[]>([]);
  const loadingKeysRef = useRef<Set<React.Key>>(new Set());
  const { isLoading, fetchBaseDirectories, fetchDirectoryContents } = useBrowseApi();
  const { openContextMenu } = useContextMenu();

  // 초기 트리 데이터를 로드합니다.
  useEffect(() => {
    let isMounted = true;
    const loadInitialData = async () => {
      if (spaces && spaces.length > 0) {
        // Spaces를 루트 노드로 표시
        if (isMounted) {
          setTreeData(spaces.map(space => ({
            title: space.space_name,
            key: `space-${space.id}`,
            isLeaf: false,
          })));
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
      } else if (showBaseDirectories) {
        // base directories 로드 (모달 등에서 사용)
        const baseDirs = await fetchBaseDirectories();
        if (isMounted) {
          setTreeData(convertToFileTreeData(baseDirs));
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
    };
    loadInitialData();
    return () => { isMounted = false; };
  }, [rootPath, rootName, showBaseDirectories, spaces, fetchBaseDirectories]);

  // 트리 노드를 확장할 때 자식 노드를 비동기적으로 불러옵니다 (Lazy Loading).
  const onLoadData = ({ key, children }: {key: React.Key; children?: TreeDataNode[]}): Promise<void> => {
    return new Promise((resolve) => {
      if (children && children.length > 0) {
        resolve();
        return;
      }

      if (loadingKeysRef.current.has(key)) {
        resolve();
        return;
      }

      loadingKeysRef.current.add(key);

      (async () => {
        try {
          let path: string;
          let spacePrefix = '';
          const keyStr = key as string;

          // Space 노드 또는 Space 하위 노드 판별
          if (keyStr.startsWith('space-')) {
            const sepIndex = keyStr.indexOf('::');
            if (sepIndex >= 0) {
              // Space 하위 노드 (space-{id}::{path} 형식)
              spacePrefix = keyStr.substring(0, sepIndex);
              path = keyStr.substring(sepIndex + 2);
            } else {
              // Space 루트 노드
              spacePrefix = keyStr;
              const spaceId = parseInt(keyStr.replace('space-', ''));
              const space = spaces?.find(s => s.id === spaceId);
              if (!space) {
                resolve();
                return;
              }
              path = space.space_path;
            }
          } else {
            path = keyStr;
          }

          const contents = await fetchDirectoryContents(path, showBaseDirectories);
          // Space 하위 노드는 key에 prefix를 붙여 유일성 보장
          const newChildren = (contents ?? [])
            .filter(node => node.isDir)
            .map(node => ({
              title: node.name,
              key: spacePrefix ? `${spacePrefix}::${node.path}` : node.path,
              isLeaf: false,
            }));

          setTreeData(origin => updateTreeData(origin, key, newChildren));
          setLoadedKeys(prev => [...prev, key]);
        } catch (error) {
          console.error('Failed to load directory contents:', error);
        } finally {
          loadingKeysRef.current.delete(key);
          resolve();
        }
      })();
    });
  };

  const handleExpand: DirectoryTreeProps['onExpand'] = (keys: React.Key[]) => {
    setExpandedKeys(keys);
  };

  const handleSelect: DirectoryTreeProps['onSelect'] = (keys: React.Key[]) => {
    if (keys.length > 0) {
      const key = keys[0] as string;

      if (key.startsWith('space-')) {
        const sepIndex = key.indexOf('::');
        if (sepIndex >= 0) {
          // Space 하위 노드 선택 — 실제 경로 추출
          onSelect(key.substring(sepIndex + 2));
        } else {
          // Space 루트 노드 선택
          const spaceId = parseInt(key.replace('space-', ''));
          const space = spaces?.find(s => s.id === spaceId);
          if (space) {
            onSelect(space.space_path, space);
          }
        }
      } else {
        onSelect(key);
      }
    }
  };

  // 우클릭 핸들러
  const handleRightClick: DirectoryTreeProps['onRightClick'] = ({ event, node }: { event: React.MouseEvent; node: any }) => {
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


  if (!rootPath && !showBaseDirectories && (!spaces || spaces.length === 0)) {
    return (
      <div style={{ padding: '20px', textAlign: 'center', color: '#888', fontSize: '12px' }}>
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

  return (
    <Tree.DirectoryTree
      onSelect={handleSelect}
      onExpand={handleExpand}
      onRightClick={handleRightClick}
      loadData={onLoadData}
      treeData={treeData}
      loadedKeys={loadedKeys}
      expandedKeys={expandedKeys}
      showIcon={true}
      icon={<FolderOutlined />}
      expandAction="click"
    />
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

