
import React from 'react';
import { theme } from 'antd';
import FolderContent from './FolderContent';

const FileExplorer: React.FC = () => {
  const { token } = theme.useToken();

  return (
    <div
      className="ui-main-scroll ui-page-padding"
      style={{ background: token.colorBgLayout }}
    >
      <FolderContent />
    </div>
  );
};

export default FileExplorer;
