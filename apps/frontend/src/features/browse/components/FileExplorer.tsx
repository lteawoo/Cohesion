
import React from 'react';
import { theme } from 'antd';
import FolderContent from './FolderContent';

const FileExplorer: React.FC = () => {
  const { token } = theme.useToken();

  return (
    <div style={{
      height: '100%',
      minHeight: 0,
      overflow: 'hidden',
      padding: '16px',
      background: token.colorBgLayout
    }}>
      <FolderContent />
    </div>
  );
};

export default FileExplorer;
