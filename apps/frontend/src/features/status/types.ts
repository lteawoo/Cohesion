export interface ProtocolStatus {
  status: 'healthy' | 'unhealthy' | 'unavailable';
  message: string;
  port?: string;
}

export interface StatusResponse {
  protocols: Record<string, ProtocolStatus>;
  hosts: string[];
}
