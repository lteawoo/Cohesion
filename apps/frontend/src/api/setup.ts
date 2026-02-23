import { apiFetch } from './client';
import i18n from '@/i18n';

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
    await throwResponseError(response, i18n.t('apiErrors.setupStatusFailed'));
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
    await throwResponseError(response, i18n.t('apiErrors.setupAdminFailed'));
  }
}
