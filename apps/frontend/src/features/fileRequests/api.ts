import { apiFetch } from '@/api/client';

export interface FileRequestLink {
  publicOrigin?: string;
  id: number;
  title: string;
  token: string;
  expires: number;
  byteLimit: number;
  received: number;
}

export async function manageRequest(spaceId: number, folder: string, init: RequestInit = {}, id?: number) {
  const query = new URLSearchParams({ folder: folder.replace(/^\/+/, '') });
  if (id !== undefined) query.set('id', String(id));
  const response = await apiFetch(`/api/spaces/${spaceId}/files/file-requests?${query}`, init);
  if (!response.ok) throw new Error(String(response.status));
  return response;
}

export class SubmissionError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

export async function submitRequest(token: string, key?: string, init: RequestInit = {}) {
  const headers = new Headers(init.headers);
  headers.set('X-File-Request', token);
  if (key) headers.set('X-Submission-Key', key);
  const response = await fetch('/api/file-drop', { ...init, headers, credentials: 'omit', cache: 'no-store' });
  if (!response.ok) {
    const detail = await response.json().catch(() => ({}));
    throw new SubmissionError(response.status, String(detail.error ?? ''));
  }
  return response;
}

export function newSubmissionKey(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(24));
  return btoa(String.fromCharCode(...bytes)).replace(/\+/g, '-').replace(/\//g, '_');
}

export async function sendSubmission(token: string, key: string, file: File, progress: (offset: number) => void, signal?: AbortSignal) {
  const created = await submitRequest(token, key, {
    method: 'POST', signal, headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ filename: file.name, size: file.size }),
  });
  let state = await created.json();
  if (state.status === 'uploaded') return;
  if (!Number.isSafeInteger(state.offset) || state.offset < 0 || state.offset > file.size) throw new Error('Invalid offset');
  progress(state.offset);
  while (state.offset < file.size) {
    const end = Math.min(state.offset + 4 * 1024 * 1024, file.size);
    const response = await submitRequest(token, key, {
      method: 'PATCH', signal, headers: { 'Upload-Offset': String(state.offset) }, body: file.slice(state.offset, end),
    });
    state = await response.json();
    if (state.offset !== end) throw new Error('Invalid offset');
    progress(end);
  }
  const completed = await submitRequest(token, key, { method: 'PUT', signal });
  if ((await completed.json()).status !== 'uploaded') throw new Error('Incomplete submission');
}
