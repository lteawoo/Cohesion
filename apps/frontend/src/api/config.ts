import { apiFetch } from './client';
import i18n from '@/i18n';

export interface ServerConfig {
  port: string;
  webdavEnabled: boolean;
  sftpEnabled: boolean;
  sftpPort: number;
}

export interface Config {
  server: ServerConfig;
}

async function extractErrorMessage(response: Response, fallbackMessage: string): Promise<string> {
  try {
    const payload = (await response.json()) as { error?: string; message?: string };
    const message = payload.error ?? payload.message;
    if (typeof message === 'string' && message.trim().length > 0) {
      return message;
    }
  } catch {
    // ignore parse errors and use fallback message
  }
  return fallbackMessage;
}

/**
 * 현재 서버 설정을 조회합니다
 */
export async function getConfig(): Promise<Config> {
  const response = await apiFetch('/api/config');
  if (!response.ok) {
    throw new Error(i18n.t('apiErrors.configFetchFailed'));
  }
  return response.json();
}

/**
 * 서버 설정을 업데이트합니다
 */
export async function updateConfig(config: Config): Promise<void> {
  const response = await apiFetch('/api/config', {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(config),
  });

  if (!response.ok) {
    throw new Error(await extractErrorMessage(response, i18n.t('apiErrors.configSaveFailed')));
  }
}

/**
 * 서버를 재시작합니다
 * @returns 새로운 포트 번호
 */
export async function restartServer(): Promise<string> {
  const response = await apiFetch('/api/system/restart', {
    method: 'POST',
  });

  if (!response.ok) {
    throw new Error(i18n.t('apiErrors.serverRestartFailed'));
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
    const response = await apiFetch('/api/health', {
      method: 'GET',
      cache: 'no-cache',
    });

    return response.ok;
  } catch {
    return false;
  }
}

/**
 * 새 포트로 reconnect될 때까지 polling
 * 개발 모드에서는 Vite proxy를 통해 현재 오리진으로 체크
 */
export async function waitForReconnect(maxAttempts = 30): Promise<boolean> {
  for (let i = 0; i < maxAttempts; i++) {
    await new Promise(resolve => setTimeout(resolve, 1000)); // 1초 대기

    // 개발 모드에서는 상대 경로 사용 (Vite proxy를 통해 백엔드로 전달)
    const isHealthy = await healthCheck();

    if (isHealthy) {
      return true;
    }
  }

  return false;
}
