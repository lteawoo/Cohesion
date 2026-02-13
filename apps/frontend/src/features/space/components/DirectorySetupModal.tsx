import { Modal, Input, App } from "antd";
import FolderTree from "../../browse/components/FolderTree";
import { useState } from "react";
import { useSpaceStore } from "@/stores/spaceStore";

const { TextArea } = Input;

export default function DirectorySetupModal({
  isOpen,
  onClose
}: {
  isOpen: boolean;
  onClose: () => void;
}) {
  const { message } = App.useApp();
  const [selectedPath, setSelectedPath] = useState<string>('');
  const [spaceName, setSpaceName] = useState<string>('');
  const [spaceDesc, setSpaceDesc] = useState<string>('');
  const createSpace = useSpaceStore((state) => state.createSpace);
  const isLoading = useSpaceStore((state) => state.isLoading);

  const handleClose = () => {
    setSelectedPath('');
    setSpaceName('');
    setSpaceDesc('');
    onClose();
  };

  const handleSelect = (path: string) => {
    setSelectedPath(path);

    // 선택된 폴더 이름을 기본 Space 이름으로 설정
    if (!spaceName) {
      const folderName = path.split('/').pop() || '';
      setSpaceName(folderName);
    }
  };

  const handleOk = async () => {
    // 유효성 검사
    if (!spaceName.trim()) {
      message.error('Space 이름을 입력해주세요.');
      return;
    }

    if (!selectedPath) {
      message.error('폴더를 선택해주세요.');
      return;
    }

    try {
      await createSpace(spaceName.trim(), selectedPath);
      message.success('Space가 성공적으로 생성되었습니다.');
      handleClose();
    } catch (error) {
      message.error(error instanceof Error ? error.message : 'Space 생성에 실패했습니다.');
    }
  };

  return (
    <Modal
      title="새 Space 생성"
      open={isOpen}
      onOk={handleOk}
      onCancel={handleClose}
      width={600}
      okButtonProps={{
        disabled: !selectedPath || !spaceName.trim(),
        loading: isLoading
      }}
      cancelButtonProps={{ disabled: isLoading }}
      destroyOnHidden={true}
    >
      <div style={{ marginBottom: '16px' }}>
        <label style={{ display: 'block', marginBottom: '8px' }}>
          Space 이름 <span style={{ color: 'red' }}>*</span>
        </label>
        <Input
          placeholder="Space 이름을 입력하세요"
          value={spaceName}
          onChange={(e: React.ChangeEvent<HTMLInputElement>) => setSpaceName(e.target.value)}
          maxLength={100}
          disabled={isLoading}
        />
      </div>

      <div style={{ marginBottom: '16px' }}>
        <label style={{ display: 'block', marginBottom: '8px' }}>
          설명 (선택)
        </label>
        <TextArea
          placeholder="Space에 대한 설명을 입력하세요"
          value={spaceDesc}
          onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => setSpaceDesc(e.target.value)}
          rows={3}
          disabled={isLoading}
        />
      </div>

      <div style={{ marginBottom: '16px' }}>
        <label style={{ display: 'block', marginBottom: '8px' }}>
          폴더 선택 <span style={{ color: 'red' }}>*</span>
        </label>
        <div style={{ fontStyle: 'italic', marginBottom: '8px', fontSize: '12px' }}>
          선택된 폴더: {selectedPath || '없음'}
        </div>
        <div style={{
          height: '40vh',
          overflow: 'auto',
          border: '1px solid #d9d9d9',
          borderRadius: '4px',
          padding: '8px'
        }}>
          <FolderTree onSelect={handleSelect} showBaseDirectories={true} />
        </div>
      </div>
    </Modal>
  );
}
