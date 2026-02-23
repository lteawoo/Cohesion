import React, { useEffect, useRef } from 'react';
import { Modal, Input } from 'antd';
import type { InputRef } from 'antd';
import { useTranslation } from 'react-i18next';

interface CreateFolderModalProps {
  visible: boolean;
  folderName: string;
  onConfirm: () => void;
  onCancel: () => void;
  onChange: (folderName: string) => void;
}

const CreateFolderModal: React.FC<CreateFolderModalProps> = ({
  visible,
  folderName,
  onConfirm,
  onCancel,
  onChange,
}) => {
  const { t } = useTranslation();
  const inputRef = useRef<InputRef>(null);

  useEffect(() => {
    if (visible && inputRef.current) {
      // 모달이 완전히 열린 후 포커스 설정
      setTimeout(() => {
        inputRef.current?.focus();
      }, 100);
    }
  }, [visible]);

  return (
    <Modal
      title={t('createFolderModal.title')}
      open={visible}
      onOk={onConfirm}
      onCancel={onCancel}
      maskClosable={false}
      styles={{ mask: { pointerEvents: 'auto' } }}
      okText={t('createFolderModal.create')}
      cancelText={t('createFolderModal.cancel')}
    >
      <Input
        ref={inputRef}
        placeholder={t('createFolderModal.placeholder')}
        value={folderName}
        onChange={(e: React.ChangeEvent<HTMLInputElement>) => onChange(e.target.value)}
        onPressEnter={onConfirm}
      />
    </Modal>
  );
};

export default CreateFolderModal;
