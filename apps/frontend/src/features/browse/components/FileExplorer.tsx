
import React from 'react';
import { theme } from 'antd';
import { useOutletContext } from 'react-router';
import FolderContent from './FolderContent';
import type { Space } from '@/features/space/types';

interface OutletContext {
  selectedPath: string;
  selectedSpace?: Space;
  onPathChange: (path: string) => void;
}

const FileExplorer: React.FC = () => {
  const { token } = theme.useToken();
  const { selectedPath, selectedSpace, onPathChange } = useOutletContext<OutletContext>();

  return (
    <div style={{
      height: 'calc(100vh - 64px)',
      overflow: 'auto',
      padding: '24px',
      background: token.colorBgLayout
    }}>
      <FolderContent
        selectedPath={selectedPath}
        selectedSpace={selectedSpace}
        onPathChange={onPathChange}
      />
    </div>
  );
};

export default FileExplorer;
