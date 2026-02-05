
import React, { useEffect, useRef, useState } from 'react';
import { Tree, Spin, Menu } from 'antd';
import type { GetProps, MenuProps } from 'antd';
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
  const loadingKeysRef = useRef<Set<React.Key>>(new Set());
  const { isLoading, fetchBaseDirectories, fetchDirectoryContents } = useBrowseApi();
  
  // 컨텍스트 메뉴 상태 관리
  const [contextMenu, setContextMenu] = useState<{
    visible: boolean;
    x: number;
    y: number;
    space?: Space;
  }>({ visible: false, x: 0, y: 0 });

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
        }
      } else if (rootPath && rootName) {
        // 단일 Space를 루트 노드로 표시
        if (isMounted) {
          setTreeData([{
            title: rootName,
            key: rootPath,
            isLeaf: false,
          }]);
        }
      } else if (showBaseDirectories) {
        // base directories 로드 (모달 등에서 사용)
        const baseDirs = await fetchBaseDirectories();
        if (isMounted) {
          setTreeData(convertToFileTreeData(baseDirs));
        }
      } else {
        // 트리를 비움
        if (isMounted) {
          setTreeData([]);
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
          
          // Space 노드인지 확인
          if (typeof key === 'string' && key.startsWith('space-')) {
            const spaceId = parseInt(key.replace('space-', ''));
            const space = spaces?.find(s => s.id === spaceId);
            if (!space) {
              resolve();
              return;
            }
            path = space.space_path;
          } else {
            path = key as string;
          }

          const contents = await fetchDirectoryContents(path, showBaseDirectories);
          const newChildren = convertToFileTreeData(contents);

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

  const handleSelect: DirectoryTreeProps['onSelect'] = (keys: React.Key[]) => {
    if (keys.length > 0) {
      const key = keys[0] as string;
      
      // Space 노드 선택 시 해당 Space의 경로와 Space 정보를 반환
      if (key.startsWith('space-')) {
        const spaceId = parseInt(key.replace('space-', ''));
        const space = spaces?.find(s => s.id === spaceId);
        if (space) {
          onSelect(space.space_path, space);
        }
      } else {
        onSelect(key);
      }
    }
  };

  // 우클릭 핸들러
  const handleRightClick: DirectoryTreeProps['onRightClick'] = ({ event, node }) => {
    if (!onSpaceDelete) return;

    const key = node.key as string;
    
    // Space 노드인지 확인
    if (key.startsWith('space-')) {
      const spaceId = parseInt(key.replace('space-', ''));
      const space = spaces?.find(s => s.id === spaceId);
      
      if (space) {
        event.preventDefault();
        setContextMenu({
          visible: true,
          x: event.clientX,
          y: event.clientY,
          space,
        });
      }
    }
  };

  // 컨텍스트 메뉴 닫기
  useEffect(() => {
    const handleClick = () => {
      setContextMenu({ visible: false, x: 0, y: 0 });
    };

    if (contextMenu.visible) {
      document.addEventListener('click', handleClick);
      return () => document.removeEventListener('click', handleClick);
    }
  }, [contextMenu.visible]);

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

  const menuItems: MenuProps['items'] = contextMenu.space ? [
    {
      key: 'delete',
      icon: <DeleteOutlined />,
      label: '삭제',
      danger: true,
      onClick: () => {
        if (contextMenu.space) {
          onSpaceDelete?.(contextMenu.space);
        }
      },
    },
  ] : [];

  return (
    <>
      <Tree.DirectoryTree
        onSelect={handleSelect}
        onRightClick={handleRightClick}
        loadData={onLoadData}
        treeData={treeData}
        loadedKeys={loadedKeys}
        showIcon={true}
        icon={<FolderOutlined />}
        expandAction="click"
      />
      {contextMenu.visible && (
        <Menu
          items={menuItems}
          style={{
            position: 'fixed',
            left: contextMenu.x,
            top: contextMenu.y,
            zIndex: 1000,
          }}
        />
      )}
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

