export interface ProtocolStatus {
  status: 'healthy' | 'unhealthy' | 'unavailable';
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
