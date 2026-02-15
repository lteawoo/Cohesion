import React from 'react';
import { Modal, Input } from 'antd';

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
  return (
    <Modal
      title="이름 변경"
      open={visible}
      onOk={onConfirm}
      onCancel={onCancel}
      maskClosable={false}
      styles={{ mask: { pointerEvents: 'auto' } }}
      okText="변경"
      cancelText="취소"
    >
      <Input
        placeholder="새 이름"
        value={initialName}
        onChange={(e: React.ChangeEvent<HTMLInputElement>) => onChange(e.target.value)}
        onPressEnter={onConfirm}
      />
    </Modal>
  );
};

export default RenameModal;
