import React, { useState, useMemo } from 'react';
import { Modal, theme, App } from 'antd';
import FolderTree from './FolderTree';
import type { Space } from '@/features/space/types';
import { useTranslation } from 'react-i18next';

interface DestinationPickerModalProps {
  visible: boolean;
  mode: 'move' | 'copy';
  sourceCount: number;
  sources: string[];
  currentPath: string;
  currentSpace?: Space;
  onConfirm: (destination: string, destinationSpace?: Space) => void;
  onCancel: () => void;
}

const DestinationPickerModal: React.FC<DestinationPickerModalProps> = ({
  visible,
  mode,
  sourceCount,
  sources,
  currentPath,
  currentSpace,
  onConfirm,
  onCancel,
}) => {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const [selectedDestination, setSelectedDestination] = useState<string>('');
  const [selectedDestinationSpace, setSelectedDestinationSpace] = useState<Space | undefined>();

  const handleAfterOpenChange = (open: boolean) => {
    if (!open) {
      return;
    }
    setSelectedDestination(currentPath);
    setSelectedDestinationSpace(currentSpace);
  };

  const handleOk = () => {
    const hasDestinationSelection = Boolean(selectedDestinationSpace) || selectedDestination !== '';
    if (!hasDestinationSelection) {
      message.warning(t('destinationPicker.selectDestinationWarning'));
      return;
    }

    const currentSpaceId = currentSpace?.id ?? null;
    const destinationSpaceId = selectedDestinationSpace?.id ?? currentSpaceId;
    const isSameSpaceDestination = currentSpaceId !== null && destinationSpaceId === currentSpaceId;

    // 현재 경로로는 이동/복사 불가
    if (isSameSpaceDestination && selectedDestination === currentPath) {
      message.error(t('destinationPicker.sameDestinationError'));
      return;
    }

    // 소스 경로 중 하나로는 이동/복사 불가
    if (isSameSpaceDestination && sources.includes(selectedDestination)) {
      message.error(t('destinationPicker.sourceDestinationError'));
      return;
    }

    // 소스가 폴더이고, 대상이 소스의 하위 디렉토리인지 확인
    const isSubdirectory = isSameSpaceDestination && sources.some(source => {
      return selectedDestination.startsWith(source + '/');
    });
    if (isSubdirectory) {
      message.error(t('destinationPicker.subdirectoryError'));
      return;
    }

    onConfirm(selectedDestination, selectedDestinationSpace);
    setSelectedDestination('');
  };

  const handleCancel = () => {
    setSelectedDestination('');
    setSelectedDestinationSpace(undefined);
    onCancel();
  };

  const handleSelect = (path: string, space?: Space) => {
    setSelectedDestination(path);
    setSelectedDestinationSpace(space);
  };

  const { token } = theme.useToken();

  // 선택된 경로를 Space 상대 경로로 표시
  const displayPath = useMemo(() => {
    if (selectedDestinationSpace) {
      return selectedDestination
        ? `${selectedDestinationSpace.space_name}/${selectedDestination}`
        : `${selectedDestinationSpace.space_name}/`;
    }
    if (!selectedDestination) return '';
    const leafName = selectedDestination.split('/').filter(Boolean).pop();
    return leafName ?? t('destinationPicker.selectedLabel');
  }, [selectedDestination, selectedDestinationSpace, t]);

  const modeText = mode === 'move'
    ? t('destinationPicker.moveMode')
    : t('destinationPicker.copyMode');

  const treeSelectedKeys = useMemo<React.Key[]>(() => {
    if (!selectedDestinationSpace) {
      return [];
    }
    if (!selectedDestination) {
      return [`space-${selectedDestinationSpace.id}`];
    }
    return [`space-${selectedDestinationSpace.id}::${selectedDestination}`];
  }, [selectedDestination, selectedDestinationSpace]);

  return (
    <Modal
      title={t('destinationPicker.title', { mode: modeText, count: sourceCount })}
      open={visible}
      onOk={handleOk}
      onCancel={handleCancel}
      afterOpenChange={handleAfterOpenChange}
      maskClosable={false}
      styles={{ mask: { pointerEvents: 'auto' } }}
      okText={mode === 'move' ? t('destinationPicker.okMove') : t('destinationPicker.okCopy')}
      cancelText={t('destinationPicker.cancel')}
      width={600}
    >
      <div style={{ marginBottom: '16px', color: token.colorTextSecondary, fontSize: '14px' }}>
        {t('destinationPicker.selectTargetDescription', { mode: modeText })}
      </div>
      <div style={{ border: `1px solid ${token.colorBorder}`, borderRadius: '4px', padding: '8px', maxHeight: '400px', overflow: 'auto' }}>
        <FolderTree
          onSelect={handleSelect}
          selectedKeys={treeSelectedKeys}
        />
      </div>
      {(selectedDestinationSpace || selectedDestination) && (
        <div style={{ marginTop: '16px', padding: '8px', backgroundColor: token.colorBgContainer, border: `1px solid ${token.colorBorder}`, borderRadius: '4px' }}>
          {t('destinationPicker.selectedPath')}: <strong>{displayPath}</strong>
        </div>
      )}
    </Modal>
  );
};

export default DestinationPickerModal;
