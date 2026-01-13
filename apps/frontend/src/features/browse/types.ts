
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
