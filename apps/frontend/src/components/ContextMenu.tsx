import React, { useEffect, useCallback } from 'react';
import { Dropdown } from 'antd';
import type { MenuProps } from 'antd';

interface ContextMenuProps {
  open: boolean;
  x: number;
  y: number;
  items: MenuProps['items'];
  onClose: () => void;
}

const ContextMenu: React.FC<ContextMenuProps> = ({ open, x, y, items, onClose }) => {
  const handleClose = useCallback(() => {
    onClose();
  }, [onClose]);

  useEffect(() => {
    if (!open) return;

    const handleClick = () => handleClose();
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') handleClose();
    };

    document.addEventListener('click', handleClick);
    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('click', handleClick);
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [open, handleClose]);

  return (
    <Dropdown
      menu={{ items }}
      open={open}
      trigger={[]}
      getPopupContainer={() => document.body}
    >
      <span
        style={{
          position: 'fixed',
          left: x,
          top: y,
          width: 1,
          height: 1,
          pointerEvents: 'none',
        }}
      />
    </Dropdown>
  );
};

export default ContextMenu;
