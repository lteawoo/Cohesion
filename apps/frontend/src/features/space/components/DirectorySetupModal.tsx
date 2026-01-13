import { Modal } from "antd";
import FolderTree from "../../browse/components/FolderTree";
import { useEffect, useState } from "react";

export default function DirectorySetupModal({ isOpen, onClose }: { isOpen: boolean, onClose: () => void }) {
  const [selectedPath, setSelectedPath] = useState<string>('');

  // 모달이 열릴 때마다 선택된 경로를 초기화합니다.
  useEffect(() => {
    if (isOpen) {
      setSelectedPath('');
    }
  }, [isOpen]);

  const handleSelect = (path: string) => {
    setSelectedPath(path);
  };

  const handleOk = () => {
    console.log("Selected Path:", selectedPath);
    onClose();
  };

  return (
    <Modal
      title="Space로 사용할 폴더 선택"
      open={isOpen}
      onOk={handleOk}
      onCancel={onClose}
      width={600}
      okButtonProps={{ disabled: !selectedPath }} // 선택된 경로가 없으면 확인 버튼 비활성화
      destroyOnHidden={true}
    >
      <div style={{ marginBottom: '16px', fontStyle: 'italic' }}>
        선택된 폴더: {selectedPath || '없음'}
      </div>
      <div style={{ height: '50vh', overflow: 'auto' }}>
        <FolderTree onSelect={handleSelect} />
      </div>
    </Modal>
  );
}