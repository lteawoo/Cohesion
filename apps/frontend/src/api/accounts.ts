export type AccountRole = 'admin' | 'user';

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
  const response = await fetch('/api/accounts');
  if (!response.ok) {
    throw await parseError(response);
  }
  return response.json();
}

export async function createAccount(payload: CreateAccountRequest): Promise<AccountUser> {
  const response = await fetch('/api/accounts', {
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
  const response = await fetch(`/api/accounts/${id}`, {
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
  const response = await fetch(`/api/accounts/${id}`, {
    method: 'DELETE',
  });

  if (!response.ok) {
    throw await parseError(response);
  }
}
