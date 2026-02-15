import React from 'react';
import type { SelectionBox } from '../../types';

interface BoxSelectionOverlayProps extends SelectionBox {
  visible: boolean;
}

const BoxSelectionOverlay: React.FC<BoxSelectionOverlayProps> = ({
  visible,
  startX,
  startY,
  currentX,
  currentY,
}) => {
  if (!visible) return null;

  const left = Math.min(startX, currentX);
  const top = Math.min(startY, currentY);
  const width = Math.abs(currentX - startX);
  const height = Math.abs(currentY - startY);

  return (
    <div
      style={{
        position: 'absolute',
        left: `${left}px`,
        top: `${top}px`,
        width: `${width}px`,
        height: `${height}px`,
        border: '2px solid #1890ff',
        backgroundColor: 'rgba(24, 144, 255, 0.1)',
        pointerEvents: 'none',
        zIndex: 1000,
      }}
    />
  );
};

export default BoxSelectionOverlay;
