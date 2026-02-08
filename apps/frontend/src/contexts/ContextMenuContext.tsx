import React, { createContext, useContext, useState, useCallback, useEffect } from 'react';
import type { ReactNode } from 'react';
import { Dropdown } from 'antd';
import type { MenuProps } from 'antd';

interface ContextMenuState {
  visible: boolean;
  x: number;
  y: number;
  items: MenuProps['items'];
}

interface ContextMenuContextType {
  openContextMenu: (x: number, y: number, items: MenuProps['items']) => void;
  closeContextMenu: () => void;
}

const ContextMenuContext = createContext<ContextMenuContextType | undefined>(undefined);

export const ContextMenuProvider: React.FC<{ children: ReactNode }> = ({ children }) => {
  const [contextMenu, setContextMenu] = useState<ContextMenuState>({
    visible: false,
    x: 0,
    y: 0,
    items: [],
  });

  const openContextMenu = useCallback((x: number, y: number, items: MenuProps['items']) => {
    setContextMenu({
      visible: true,
      x,
      y,
      items,
    });
  }, []);

  const closeContextMenu = useCallback(() => {
    setContextMenu(prev => ({
      ...prev,
      visible: false,
    }));
  }, []);

  // 외부 클릭 및 ESC 키로 닫기
  useEffect(() => {
    if (!contextMenu.visible) return;

    const handleClick = () => closeContextMenu();
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') closeContextMenu();
    };

    document.addEventListener('click', handleClick);
    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('click', handleClick);
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [contextMenu.visible, closeContextMenu]);

  return (
    <ContextMenuContext.Provider value={{ openContextMenu, closeContextMenu }}>
      {children}
      {/* 전역 컨텍스트 메뉴 렌더링 */}
      <Dropdown
        key={`${contextMenu.x}-${contextMenu.y}`}
        menu={{ items: contextMenu.items }}
        open={contextMenu.visible}
        trigger={[]}
        getPopupContainer={() => document.body}
      >
        <span
          style={{
            position: 'fixed',
            left: contextMenu.x,
            top: contextMenu.y,
            width: 1,
            height: 1,
            pointerEvents: 'none',
          }}
        />
      </Dropdown>
    </ContextMenuContext.Provider>
  );
};

export const useContextMenu = () => {
  const context = useContext(ContextMenuContext);
  if (!context) {
    throw new Error('useContextMenu must be used within ContextMenuProvider');
  }
  return context;
};
