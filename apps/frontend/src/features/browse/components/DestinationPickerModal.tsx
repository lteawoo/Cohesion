import React, { useState } from 'react';
import { Modal, message } from 'antd';
import FolderTree from './FolderTree';
import type { Space } from '@/features/space/types';

interface DestinationPickerModalProps {
  visible: boolean;
  mode: 'move' | 'copy';
  sourceCount: number;
  sources: string[];
  currentPath: string;
  selectedSpace?: Space;
  onConfirm: (destination: string) => void;
  onCancel: () => void;
}

const DestinationPickerModal: React.FC<DestinationPickerModalProps> = ({
  visible,
  mode,
  sourceCount,
  sources,
  currentPath,
  selectedSpace,
  onConfirm,
  onCancel,
}) => {
  const [selectedDestination, setSelectedDestination] = useState<string>('');

  const handleOk = () => {
    if (!selectedDestination) {
      message.warning('대상 폴더를 선택하세요');
      return;
    }

    // 현재 경로로는 이동/복사 불가
    if (selectedDestination === currentPath) {
      message.error('같은 위치로 이동/복사할 수 없습니다');
      return;
    }

    // 소스 경로 중 하나로는 이동/복사 불가
    if (sources.includes(selectedDestination)) {
      message.error('선택한 항목으로 이동/복사할 수 없습니다');
      return;
    }

    // 소스가 폴더이고, 대상이 소스의 하위 디렉토리인지 확인
    const isSubdirectory = sources.some(source => {
      return selectedDestination.startsWith(source + '/');
    });
    if (isSubdirectory) {
      message.error('하위 폴더로 이동/복사할 수 없습니다');
      return;
    }

    onConfirm(selectedDestination);
    setSelectedDestination('');
  };

  const handleCancel = () => {
    setSelectedDestination('');
    onCancel();
  };

  const handleSelect = (path: string) => {
    setSelectedDestination(path);
  };

  return (
    <Modal
      title={`${mode === 'move' ? '이동' : '복사'} - ${sourceCount}개 항목`}
      open={visible}
      onOk={handleOk}
      onCancel={handleCancel}
      okText={mode === 'move' ? '이동' : '복사'}
      cancelText="취소"
      width={600}
    >
      <div style={{ marginBottom: '16px', color: '#8c8c8c', fontSize: '14px' }}>
        {mode === 'move' ? '이동할' : '복사할'} 대상 폴더를 선택하세요
      </div>
      <div style={{ border: '1px solid #d9d9d9', borderRadius: '4px', padding: '8px', maxHeight: '400px', overflow: 'auto' }}>
        <FolderTree
          onSelect={handleSelect}
        />
      </div>
      {selectedDestination && (
        <div style={{ marginTop: '16px', padding: '8px', backgroundColor: '#f0f0f0', borderRadius: '4px' }}>
          선택된 경로: <strong>{selectedDestination}</strong>
        </div>
      )}
    </Modal>
  );
};

export default DestinationPickerModal;
