import React from 'react';
import { InboxOutlined } from '@ant-design/icons';

interface UploadOverlayProps {
  visible: boolean;
}

const UploadOverlay: React.FC<UploadOverlayProps> = ({ visible }) => {
  if (!visible) return null;

  return (
    <div
      style={{
        position: 'absolute',
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        backgroundColor: 'rgba(24, 144, 255, 0.1)',
        border: '2px dashed #1890ff',
        borderRadius: '8px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 999,
        pointerEvents: 'none',
      }}
    >
      <div style={{ textAlign: 'center' }}>
        <InboxOutlined style={{ fontSize: '64px', color: '#1890ff', marginBottom: '16px' }} />
        <div style={{ fontSize: '20px', fontWeight: 'bold', color: '#1890ff' }}>
          파일을 놓아 업로드
        </div>
      </div>
    </div>
  );
};

export default UploadOverlay;
