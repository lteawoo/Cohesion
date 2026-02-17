export interface FileInfo {
  name: string;
  path: string;
  isDir: boolean;
}

// Space 관련 타입
export interface Space {
  id: number;
  space_name: string;
  space_desc?: string;
  space_path?: string;
  icon?: string;
  space_category?: string;
  created_at?: string;
  created_user_id?: string;
  updated_at?: string;
  updated_user_id?: string;
}

export interface CreateSpaceRequest {
  space_name: string;
  space_desc?: string;
  space_path: string;
  icon?: string;
  space_category?: string;
}

export interface CreateSpaceResponse {
  id: number;
  space_name: string;
  message: string;
}

export interface SpaceApiError {
  error: string;
}

export interface DeleteSpaceResponse {
  message: string;
}
