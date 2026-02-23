import React from 'react';
import { Modal, Input } from 'antd';
import { useTranslation } from 'react-i18next';

interface RenameModalProps {
  visible: boolean;
  initialName: string;
  onConfirm: () => void;
  onCancel: () => void;
  onChange: (newName: string) => void;
}

const RenameModal: React.FC<RenameModalProps> = ({
  visible,
  initialName,
  onConfirm,
  onCancel,
  onChange,
}) => {
  const { t } = useTranslation();
  return (
    <Modal
      title={t('renameModal.title')}
      open={visible}
      onOk={onConfirm}
      onCancel={onCancel}
      maskClosable={false}
      styles={{ mask: { pointerEvents: 'auto' } }}
      okText={t('renameModal.confirm')}
      cancelText={t('renameModal.cancel')}
    >
      <Input
        placeholder={t('renameModal.placeholder')}
        value={initialName}
        onChange={(e: React.ChangeEvent<HTMLInputElement>) => onChange(e.target.value)}
        onPressEnter={onConfirm}
      />
    </Modal>
  );
};

export default RenameModal;
