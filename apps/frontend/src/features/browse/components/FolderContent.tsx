
import React, { useEffect, useState } from 'react';
import { List, Spin, Empty } from 'antd';
import { FolderOutlined } from '@ant-design/icons';
import { useBrowseApi } from '../hooks/useBrowseApi';
import type { FileNode } from '../types';

interface FolderContentProps {
  selectedPath: string;
}

const FolderContent: React.FC<FolderContentProps> = ({ selectedPath }) => {
  const [content, setContent] = useState<FileNode[]>([]);
  const { isLoading, fetchDirectoryContents } = useBrowseApi();

  useEffect(() => {
    if (selectedPath) {
      const loadContent = async () => {
        const contents = await fetchDirectoryContents(selectedPath);
        const foldersOnly = contents.filter(item => item.isDir); // 폴더만 필터링
        setContent(foldersOnly);
      };
      loadContent();
    }
  }, [selectedPath, fetchDirectoryContents]);

  if (!selectedPath) {
    return <Empty description="왼쪽 트리에서 폴더를 선택하세요." />;
  }

  if (isLoading) {
    return <Spin />;
  }

  return (
    <List
      header={<div>{selectedPath}</div>}
      bordered
      dataSource={content}
      renderItem={(item) => (
        <List.Item>
          <List.Item.Meta
            avatar={<FolderOutlined />}
            title={<a>{item.name}</a>}
          />
        </List.Item>
      )}
    />
  );
};

export default FolderContent;
