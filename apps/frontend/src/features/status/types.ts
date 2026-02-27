export interface ProtocolStatus {
  status: 'healthy' | 'unhealthy' | 'unavailable' | 'external';
  message: string;
  port?: string;
  path?: string;
}

export interface StatusResponse {
  protocols: Record<string, ProtocolStatus>;
  hosts: string[];
}

export interface UpdateCheckResponse {
  currentVersion: string;
  latestVersion?: string;
  updateAvailable: boolean;
  releaseUrl: string;
  checkedAt: string;
  error?: string;
}

export interface SystemVersionResponse {
  version: string;
  commit: string;
  buildDate: string;
}
