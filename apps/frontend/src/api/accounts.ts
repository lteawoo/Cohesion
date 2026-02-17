import { apiFetch } from './client';
import type { Space } from '@/features/space/types';

export type AccountRole = string;

export interface AccountUser {
  id: number;
  username: string;
  nickname: string;
  role: AccountRole;
  createdAt: string;
  updatedAt: string;
}

export interface CreateAccountRequest {
  username: string;
  password: string;
  nickname: string;
  role: AccountRole;
}

export interface UpdateAccountRequest {
  nickname?: string;
  password?: string;
  role?: AccountRole;
}

export type SpacePermission = 'read' | 'write' | 'manage';

export interface UserSpacePermission {
  userId: number;
  spaceId: number;
  permission: SpacePermission;
}

async function parseError(response: Response): Promise<Error> {
  try {
    const data = await response.json() as { message?: string };
    if (data.message) {
      return new Error(data.message);
    }
  } catch {
    // noop
  }
  return new Error(`Request failed: ${response.status}`);
}

export async function listAccounts(): Promise<AccountUser[]> {
  const response = await apiFetch('/api/accounts');
  if (!response.ok) {
    throw await parseError(response);
  }
  return response.json();
}

export async function createAccount(payload: CreateAccountRequest): Promise<AccountUser> {
  const response = await apiFetch('/api/accounts', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  });

  if (!response.ok) {
    throw await parseError(response);
  }

  return response.json();
}

export async function updateAccount(id: number, payload: UpdateAccountRequest): Promise<AccountUser> {
  const response = await apiFetch(`/api/accounts/${id}`, {
    method: 'PATCH',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  });

  if (!response.ok) {
    throw await parseError(response);
  }

  return response.json();
}

export async function deleteAccount(id: number): Promise<void> {
  const response = await apiFetch(`/api/accounts/${id}`, {
    method: 'DELETE',
  });

  if (!response.ok) {
    throw await parseError(response);
  }
}

export async function listAccountPermissions(id: number): Promise<UserSpacePermission[]> {
  const response = await apiFetch(`/api/accounts/${id}/permissions`);
  if (!response.ok) {
    throw await parseError(response);
  }
  return response.json();
}

export async function updateAccountPermissions(id: number, permissions: UserSpacePermission[]): Promise<void> {
  const response = await apiFetch(`/api/accounts/${id}/permissions`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ permissions }),
  });
  if (!response.ok) {
    throw await parseError(response);
  }
}

export async function listSpaces(): Promise<Space[]> {
  const response = await apiFetch('/api/spaces');
  if (!response.ok) {
    throw await parseError(response);
  }
  return response.json();
}
