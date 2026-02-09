import React, { useEffect, useCallback } from 'react';
import { Dropdown } from 'antd';
import { useContextMenuStore } from '@/stores/contextMenuStore';

const ContextMenu: React.FC = () => {
  const { visible, x, y, items, closeContextMenu } = useContextMenuStore();

  const handleClose = useCallback(() => {
    closeContextMenu();
  }, [closeContextMenu]);

  useEffect(() => {
    if (!visible) return;

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
  }, [visible, handleClose]);

  return (
    <Dropdown
      key={`${x}-${y}`}
      menu={{ items }}
      open={visible}
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
