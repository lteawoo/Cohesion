import { apiFetch } from './client';

export interface RoleItem {
  name: string;
  description: string;
  isSystem: boolean;
  permissions: string[];
}

export interface PermissionItem {
  key: string;
  description: string;
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

export async function listRoles(): Promise<RoleItem[]> {
  const response = await apiFetch('/api/roles');
  if (!response.ok) {
    throw await parseError(response);
  }
  return response.json();
}

export async function createRole(name: string, description: string): Promise<RoleItem> {
  const response = await apiFetch('/api/roles', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, description }),
  });
  if (!response.ok) {
    throw await parseError(response);
  }
  return response.json();
}

export async function deleteRole(name: string): Promise<void> {
  const response = await apiFetch(`/api/roles/${encodeURIComponent(name)}`, { method: 'DELETE' });
  if (!response.ok) {
    throw await parseError(response);
  }
}

export async function listPermissionDefinitions(): Promise<PermissionItem[]> {
  const response = await apiFetch('/api/permissions');
  if (!response.ok) {
    throw await parseError(response);
  }
  return response.json();
}

export async function updateRolePermissions(roleName: string, permissions: string[]): Promise<void> {
  const response = await apiFetch(`/api/roles/${encodeURIComponent(roleName)}/permissions`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ permissions }),
  });
  if (!response.ok) {
    throw await parseError(response);
  }
}
