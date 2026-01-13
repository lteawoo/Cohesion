
import React, { useState } from 'react';
import { Flex } from 'antd';
import FolderTree from './FolderTree';
import FolderContent from './FolderContent';

const FileExplorer: React.FC = () => {
  const [selectedPath, setSelectedPath] = useState<string>('');

  const handleSelect = (path: string) => {
    setSelectedPath(path);
  };

  return (
    <Flex vertical={false} style={{ height: '100%' }}>
      <div style={{ flex: '0 0 300px', overflow: 'auto', padding: '12px', borderRight: '1px solid #f0f0f0' }}>
        <FolderTree onSelect={handleSelect} />
      </div>
      <div style={{ flex: 1, overflow: 'auto', padding: '12px' }}>
        <FolderContent selectedPath={selectedPath} />
      </div>
    </Flex>
  );
};

export default FileExplorer;
