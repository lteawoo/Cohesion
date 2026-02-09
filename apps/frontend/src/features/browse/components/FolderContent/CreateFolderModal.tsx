import React from 'react';
import { Modal, Input } from 'antd';

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
  return (
    <Modal
      title="새 폴더 만들기"
      open={visible}
      onOk={onConfirm}
      onCancel={onCancel}
      okText="생성"
      cancelText="취소"
    >
      <Input
        placeholder="폴더 이름"
        value={folderName}
        onChange={(e: React.ChangeEvent<HTMLInputElement>) => onChange(e.target.value)}
        onPressEnter={onConfirm}
        autoFocus
      />
    </Modal>
  );
};

export default CreateFolderModal;
