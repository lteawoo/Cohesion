import { apiFetch } from './client';

export interface SetupStatusResponse {
  requiresSetup: boolean;
}

export interface BootstrapAdminRequest {
  username: string;
  password: string;
  nickname?: string;
}

function parseErrorMessage(data: unknown, fallback: string): string {
  if (!data || typeof data !== 'object') {
    return fallback;
  }
  const maybeError = (data as { error?: unknown }).error;
  if (typeof maybeError === 'string' && maybeError.trim().length > 0) {
    return maybeError;
  }
  const maybeMessage = (data as { message?: unknown }).message;
  if (typeof maybeMessage === 'string' && maybeMessage.trim().length > 0) {
    return maybeMessage;
  }
  return fallback;
}

async function throwResponseError(response: Response, fallback: string): Promise<never> {
  let body: unknown;
  try {
    body = await response.json();
  } catch {
    body = null;
  }
  throw new Error(parseErrorMessage(body, fallback));
}

export async function getSetupStatus(): Promise<SetupStatusResponse> {
  const response = await apiFetch('/api/setup/status', undefined, { skipAuthHandling: true });
  if (!response.ok) {
    await throwResponseError(response, '초기 설정 상태 조회에 실패했습니다');
  }
  return response.json();
}

export async function bootstrapAdmin(payload: BootstrapAdminRequest): Promise<void> {
  const response = await apiFetch('/api/setup/admin', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  }, { skipAuthHandling: true });
  if (!response.ok) {
    await throwResponseError(response, '관리자 초기 설정에 실패했습니다');
  }
}
