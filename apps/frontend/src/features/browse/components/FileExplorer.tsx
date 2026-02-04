
import React from 'react';
import { theme } from 'antd';
import { useOutletContext } from 'react-router';
import FolderContent from './FolderContent';

interface OutletContext {
  selectedPath: string;
  onPathChange: (path: string) => void;
}

const FileExplorer: React.FC = () => {
  const { token } = theme.useToken();
  const { selectedPath, onPathChange } = useOutletContext<OutletContext>();

  return (
    <div style={{
      height: 'calc(100vh - 64px)',
      overflow: 'auto',
      padding: '24px',
      background: token.colorBgLayout
    }}>
      <FolderContent
        selectedPath={selectedPath}
        onPathChange={onPathChange}
      />
    </div>
  );
};

export default FileExplorer;
