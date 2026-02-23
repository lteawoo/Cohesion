import { apiFetch } from './client';
import i18n from '@/i18n';

export interface AuthUser {
  id: number;
  username: string;
  nickname: string;
  role: string;
  permissions: string[];
}

export interface LoginRequest {
  username: string;
  password: string;
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

export async function login(payload: LoginRequest): Promise<AuthUser> {
  const response = await apiFetch('/api/auth/login', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  });

  if (!response.ok) {
    await throwResponseError(response, i18n.t('apiErrors.authLoginFailed'));
  }

  const data = await response.json() as { user: AuthUser };
  return data.user;
}

export async function me(): Promise<AuthUser> {
  const response = await apiFetch('/api/auth/me');
  if (!response.ok) {
    await throwResponseError(response, i18n.t('apiErrors.authUnauthorized'));
  }
  return response.json();
}

export async function refreshAuth(): Promise<AuthUser> {
  const response = await apiFetch('/api/auth/refresh', {
    method: 'POST',
  });
  if (!response.ok) {
    await throwResponseError(response, i18n.t('apiErrors.authRefreshFailed'));
  }
  const data = await response.json() as { user: AuthUser };
  return data.user;
}

export async function logout(): Promise<void> {
  const response = await apiFetch('/api/auth/logout', {
    method: 'POST',
  });
  if (!response.ok) {
    await throwResponseError(response, i18n.t('apiErrors.authLogoutFailed'));
  }
}
