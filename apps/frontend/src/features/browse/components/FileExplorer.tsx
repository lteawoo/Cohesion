
import React from 'react';
import { theme } from 'antd';
import FolderContent from './FolderContent';

const FileExplorer: React.FC = () => {
  const { token } = theme.useToken();

  return (
    <div style={{
      height: 'calc(100vh - 64px)',
      overflow: 'auto',
      padding: '16px',
      background: token.colorBgLayout
    }}>
      <FolderContent />
    </div>
  );
};

export default FileExplorer;
