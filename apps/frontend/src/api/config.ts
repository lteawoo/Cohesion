export interface ServerConfig {
  port: string;
  httpEnabled: boolean;
  webdavEnabled: boolean;
  ftpEnabled: boolean;
  ftpPort: number;
  sftpEnabled: boolean;
  sftpPort: number;
}

export interface DatabaseConfig {
  url: string;
  user: string;
  password: string;
  dbname: string;
}

export interface Config {
  server: ServerConfig;
  database: DatabaseConfig;
}

/**
 * 현재 서버 설정을 조회합니다
 */
export async function getConfig(): Promise<Config> {
  const response = await fetch('/api/config');
  if (!response.ok) {
    throw new Error('Failed to fetch config');
  }
  return response.json();
}

/**
 * 서버 설정을 업데이트합니다
 */
export async function updateConfig(config: Config): Promise<void> {
  const response = await fetch('/api/config', {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(config),
  });

  if (!response.ok) {
    throw new Error('Failed to update config');
  }
}

/**
 * 서버를 재시작합니다
 * @returns 새로운 포트 번호
 */
export async function restartServer(): Promise<string> {
  const response = await fetch('/api/system/restart', {
    method: 'POST',
  });

  if (!response.ok) {
    throw new Error('Failed to restart server');
  }

  const data = await response.json();
  return data.new_port;
}

/**
 * 서버 health check
 * 개발 모드에서는 상대 경로 사용 (Vite proxy를 통해 백엔드로 전달)
 */
export async function healthCheck(): Promise<boolean> {
  try {
    console.log(`[healthCheck] Checking /api/health`);

    const response = await fetch('/api/health', {
      method: 'GET',
      cache: 'no-cache',
    });

    console.log(`[healthCheck] Response status: ${response.status}`);
    return response.ok;
  } catch (error) {
    console.log(`[healthCheck] Error:`, error);
    return false;
  }
}

/**
 * 새 포트로 reconnect될 때까지 polling
 * 개발 모드에서는 Vite proxy를 통해 현재 오리진으로 체크
 */
export async function waitForReconnect(newPort: string, maxAttempts = 30): Promise<boolean> {
  console.log(`[waitForReconnect] Waiting for server (port ${newPort}), max attempts: ${maxAttempts}`);

  for (let i = 0; i < maxAttempts; i++) {
    await new Promise(resolve => setTimeout(resolve, 1000)); // 1초 대기

    console.log(`[waitForReconnect] Attempt ${i + 1}/${maxAttempts}...`);
    // 개발 모드에서는 상대 경로 사용 (Vite proxy를 통해 백엔드로 전달)
    const isHealthy = await healthCheck();

    if (isHealthy) {
      console.log(`[waitForReconnect] Success on attempt ${i + 1}`);
      return true;
    }
  }

  console.log(`[waitForReconnect] Failed after ${maxAttempts} attempts`);
  return false;
}
