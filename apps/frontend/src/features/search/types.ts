export interface SearchFileResult {
  spaceId: number;
  spaceName: string;
  name: string;
  path: string;
  parentPath: string;
  isDir: boolean;
  size: number;
  modTime: string;
}
