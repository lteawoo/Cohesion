export interface ProtocolStatus {
  status: 'healthy' | 'unhealthy' | 'unavailable';
  message: string;
}

export interface StatusResponse {
  protocols: Record<string, ProtocolStatus>;
  hosts: string[];
}
