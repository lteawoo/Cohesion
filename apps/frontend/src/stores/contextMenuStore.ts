import { create } from 'zustand';
import type { MenuProps } from 'antd';

interface ContextMenuStore {
  visible: boolean;
  x: number;
  y: number;
  items: MenuProps['items'];

  openContextMenu: (x: number, y: number, items: MenuProps['items']) => void;
  closeContextMenu: () => void;
}

export const useContextMenuStore = create<ContextMenuStore>((set) => ({
  visible: false,
  x: 0,
  y: 0,
  items: [],

  openContextMenu: (x: number, y: number, items: MenuProps['items']) => {
    set({ visible: true, x, y, items });
  },

  closeContextMenu: () => {
    set({ visible: false });
  },
}));
