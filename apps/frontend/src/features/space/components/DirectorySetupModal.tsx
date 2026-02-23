import { Modal, Input, App, theme } from "antd";
import FolderTree from "../../browse/components/FolderTree";
import { useState } from "react";
import { useSpaceStore } from "@/stores/spaceStore";
import { useTranslation } from "react-i18next";

const { TextArea } = Input;

export default function DirectorySetupModal({
  isOpen,
  onClose
}: {
  isOpen: boolean;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const [selectedPath, setSelectedPath] = useState<string>('');
  const [spaceName, setSpaceName] = useState<string>('');
  const [spaceDesc, setSpaceDesc] = useState<string>('');
  const [isCreating, setIsCreating] = useState(false);
  const createSpace = useSpaceStore((state) => state.createSpace);
  const { token } = theme.useToken();

  const handleClose = () => {
    if (isCreating) {
      return;
    }
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
      message.error(t('directorySetup.spaceNameRequired'));
      return;
    }

    if (!selectedPath) {
      message.error(t('directorySetup.folderRequired'));
      return;
    }

    setIsCreating(true);
    try {
      await createSpace(spaceName.trim(), selectedPath, spaceDesc);
      message.success(t('directorySetup.createSuccess'));
      setSelectedPath('');
      setSpaceName('');
      setSpaceDesc('');
      onClose();
    } catch (error) {
      message.error(error instanceof Error ? error.message : t('directorySetup.createFailed'));
    } finally {
      setIsCreating(false);
    }
  };

  return (
    <Modal
      title={t('directorySetup.title')}
      open={isOpen}
      onOk={handleOk}
      onCancel={handleClose}
      width={600}
      okButtonProps={{
        disabled: !selectedPath || !spaceName.trim() || isCreating,
        loading: isCreating
      }}
      cancelButtonProps={{ disabled: isCreating }}
      destroyOnHidden={true}
    >
      <div style={{ marginBottom: 16 }}>
        <label style={{ display: 'block', marginBottom: 8, fontWeight: 500 }}>
          {t('directorySetup.spaceNameLabel')} <span style={{ color: token.colorError }}>*</span>
        </label>
        <Input
          placeholder={t('directorySetup.spaceNamePlaceholder')}
          value={spaceName}
          onChange={(e: React.ChangeEvent<HTMLInputElement>) => setSpaceName(e.target.value)}
          maxLength={100}
          disabled={isCreating}
        />
      </div>

      <div style={{ marginBottom: 16 }}>
        <label style={{ display: 'block', marginBottom: 8, fontWeight: 500 }}>
          {t('directorySetup.descriptionLabel')}
        </label>
        <TextArea
          placeholder={t('directorySetup.descriptionPlaceholder')}
          value={spaceDesc}
          onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => setSpaceDesc(e.target.value)}
          rows={3}
          disabled={isCreating}
        />
      </div>

      <div style={{ marginBottom: 16 }}>
        <label style={{ display: 'block', marginBottom: 8, fontWeight: 500 }}>
          {t('directorySetup.folderSelectLabel')} <span style={{ color: token.colorError }}>*</span>
        </label>
        <div style={{ fontStyle: 'italic', marginBottom: 8, fontSize: 12 }}>
          {t('directorySetup.selectedFolder')}: {selectedPath || t('directorySetup.none')}
        </div>
        <div
          style={{
            height: '40vh',
            overflow: 'auto',
            border: `1px solid ${token.colorBorder}`,
            borderRadius: 4,
            padding: 8,
          }}
        >
          <FolderTree onSelect={handleSelect} showBaseDirectories={true} />
        </div>
      </div>
    </Modal>
  );
}
