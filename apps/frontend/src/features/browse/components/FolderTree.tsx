
import React, { useEffect, useState } from 'react';
import { Tree, Spin } from 'antd';
import type { GetProps } from 'antd';
import type { EventDataNode } from 'antd/es/tree';
import { FolderOutlined } from '@ant-design/icons';
import { useBrowseApi } from '../hooks/useBrowseApi';
import type { FileNode, TreeDataNode } from '../types';

type DirectoryTreeProps = GetProps<typeof Tree.DirectoryTree>;

// API 응답(FileNode)을 Ant Design Tree가 요구하는 형식(TreeDataNode)으로 변환합니다.
const convertToFileTreeData = (nodes: FileNode[]): TreeDataNode[] => {
  if (!nodes) return [];

  return nodes
    .filter(node => node.isDir) // 폴더만 필터링합니다.
    .map(node => ({
      title: node.name,
      key: node.path,
      isLeaf: false, // 폴더는 자식이 있을 수 있으므로 isLeaf: false
    }));
};

const FolderTree: React.FC<{ onSelect: (path: string) => void }> = ({ onSelect }) => {
  const [treeData, setTreeData] = useState<TreeDataNode[]>([]);
  const { isLoading, fetchBaseDirectories, fetchDirectoryContents } = useBrowseApi();

  // 컴포넌트 마운트 시 최상위 기본 디렉토리를 불러옵니다.
  useEffect(() => {
    const loadBaseDirs = async () => {
      const baseDirs = await fetchBaseDirectories();
      setTreeData(convertToFileTreeData(baseDirs));
    };
    loadBaseDirs();
  }, [fetchBaseDirectories]);

  // 트리 노드를 확장할 때 자식 노드를 비동기적으로 불러옵니다 (Lazy Loading).
  const onLoadData = (node: EventDataNode): Promise<void> => {
    return new Promise((resolve) => {
      (async () => {
        const { key, children } = node;
        // 이미 자식 노드가 로드된 경우, 다시 호출하지 않습니다.
        if (children) {
          resolve();
          return;
        }

        const contents = await fetchDirectoryContents(key as string);
        const newChildren = convertToFileTreeData(contents);

        // 기존 트리 데이터에 새로 불러온 자식 노드를 추가합니다.
        setTreeData(origin =>
          updateTreeData(origin, key, newChildren)
        );

        resolve();
      })();
    });
  };

  const handleSelect: DirectoryTreeProps['onSelect'] = (keys: React.Key[]) => {
    if (keys.length > 0) {
      onSelect(keys[0] as string);
    }
  };

  if (isLoading && treeData.length === 0) {
    return <Spin />;
  }

  return (
    <Tree.DirectoryTree
      onSelect={handleSelect}
      loadData={onLoadData}
      treeData={treeData}
      showIcon={true}
      icon={<FolderOutlined />}
    />
  );
};

// 기존 트리에 새로운 자식 노드들을 추가하기 위한 헬퍼 함수
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

