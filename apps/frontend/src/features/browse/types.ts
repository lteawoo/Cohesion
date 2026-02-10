
// browse 기능과 관련된 타입들을 정의합니다.
export interface FileNode {
  name: string;
  path: string;
  isDir: boolean;
  modTime: string;
  size: number;
}

export type TreeDataNode = {
  title: string;
  key: string;
  isLeaf: boolean;
  children?: TreeDataNode[];
};

// FolderContent related types
export type ViewMode = 'table' | 'grid';

export interface SortConfig {
  sortBy: 'name' | 'modTime' | 'size';
  sortOrder: 'ascend' | 'descend';
}

export interface RenameModalState {
  visible: boolean;
  record?: FileNode;
  newName: string;
}

export interface CreateFolderModalState {
  visible: boolean;
  folderName: string;
}

export interface DestinationModalState {
  visible: boolean;
  mode: 'move' | 'copy';
}

export interface DragData {
  type: 'cohesion-internal';
  paths: string[];
}

// Box selection types
export interface SelectionBox {
  startX: number;
  startY: number;
  currentX: number;
  currentY: number;
}
