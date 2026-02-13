import React, { useEffect, useRef } from 'react';
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
  const inputRef = useRef<any>(null);

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
      title="새 폴더 만들기"
      open={visible}
      onOk={onConfirm}
      onCancel={onCancel}
      okText="생성"
      cancelText="취소"
    >
      <Input
        ref={inputRef}
        placeholder="폴더 이름"
        value={folderName}
        onChange={(e: React.ChangeEvent<HTMLInputElement>) => onChange(e.target.value)}
        onPressEnter={onConfirm}
      />
    </Modal>
  );
};

export default CreateFolderModal;
