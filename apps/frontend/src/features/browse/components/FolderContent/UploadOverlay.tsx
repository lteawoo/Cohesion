import React from 'react';
import { InboxOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

interface UploadOverlayProps {
  visible: boolean;
}

const UploadOverlay: React.FC<UploadOverlayProps> = ({ visible }) => {
  const { t } = useTranslation();

  if (!visible) return null;

  return (
    <div
      style={{
        position: 'absolute',
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        backgroundColor: 'var(--browse-dragover-bg)',
        border: '2px dashed var(--browse-selection-border-color)',
        borderRadius: '8px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 999,
        pointerEvents: 'none',
      }}
    >
      <div style={{ textAlign: 'center' }}>
        <InboxOutlined style={{ fontSize: '64px', color: 'var(--ant-color-primary, #415a77)', marginBottom: '16px' }} />
        <div style={{ fontSize: '20px', fontWeight: 'bold', color: 'var(--ant-color-primary, #415a77)' }}>
          {t('folderContent.dropToUpload')}
        </div>
      </div>
    </div>
  );
};

export default UploadOverlay;
