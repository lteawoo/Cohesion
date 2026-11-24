import { apiFetch } from '@/api/client';
import { useTransferCenterStore } from '@/stores/transferCenterStore';
import type { UploadTransferItem } from '@/stores/transferCenterStore';
import type { UploadConflictPolicy, UploadResult } from './transferOperationsShared';

export const RESUMABLE_UPLOAD_THRESHOLD = 8 * 1024 * 1024;
const CHUNK_BYTES = 4 * 1024 * 1024;

export function findUpload(id: string): UploadTransferItem | undefined {
  return useTransferCenterStore.getState().transfers.find((item): item is UploadTransferItem => item.id === id && item.kind === 'upload');
}

function sessionUrl(item: UploadTransferItem) {
  const base = `/api/spaces/${item.spaceId}/files/upload-sessions`;
  return item.resume?.sessionId ? `${base}?id=${encodeURIComponent(item.resume.sessionId)}` : base;
}

interface SessionResponse {
  id: string;
  status: 'active' | 'uploaded' | 'skipped';
  filename?: string;
  offset: number;
}

async function sessionResponse(response: Response): Promise<SessionResponse> {
  if (!response.ok) {
    const error = await response.json().catch(() => ({}));
    throw Object.assign(new Error(error.message || error.error || response.statusText), { status: response.status });
  }
  return response.json();
}

export async function cancelSession(item: UploadTransferItem): Promise<UploadResult | undefined> {
  if (!item.resume?.sessionId) return;
  const response = await apiFetch(sessionUrl(item), { method: 'DELETE' });
  if (response.status === 404 || response.status === 204) return;
  const result = await sessionResponse(response);
  if (result.status !== 'active') return { status: result.status, filename: result.filename ?? item.name };
}

export async function sendResumableUpload(
  item: UploadTransferItem, file: File, signal: AbortSignal,
  onOffset: (offset: number) => void, conflictPolicy?: UploadConflictPolicy
): Promise<UploadResult> {
  const resume = item.resume!;
  const url = sessionUrl(item);
  let state = await sessionResponse(await apiFetch(url, resume.sessionId ? { signal } : {
    method: 'POST', signal,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path: resume.targetPath, filename: file.name, size: file.size, conflictPolicy }),
  }));
  if (state.status !== 'active') return { status: state.status, filename: state.filename ?? file.name };
  if (!state.id || !Number.isSafeInteger(state.offset) || state.offset < 0 || state.offset > file.size) throw new Error('Invalid upload session');
  const active = { ...item, resume: { ...resume, sessionId: state.id } };
  useTransferCenterStore.getState().upsertTransfer(active);
  const activeUrl = sessionUrl(active);
  onOffset(state.offset);
  while (state.offset < file.size) {
    const end = Math.min(state.offset + CHUNK_BYTES, file.size);
    const response = await apiFetch(activeUrl, {
      method: 'PATCH', signal,
      headers: { 'Upload-Offset': String(state.offset), 'Content-Type': 'application/octet-stream' },
      body: file.slice(state.offset, end),
    });
    // A concurrent or retried request may already have advanced this session.
    if (response.status === 409) {
      const confirmed = await sessionResponse(await apiFetch(activeUrl, { signal }));
      if (confirmed.status !== 'active') return { status: confirmed.status, filename: confirmed.filename ?? file.name };
      if (!Number.isSafeInteger(confirmed.offset) || confirmed.offset <= state.offset || confirmed.offset > file.size) throw new Error('Upload offset mismatch');
      state = confirmed;
    } else {
      state = await sessionResponse(response);
      if (state.status !== 'active' || state.offset !== end) throw new Error('Invalid upload offset');
    }
    onOffset(state.offset);
  }
  const policy = conflictPolicy ? `&conflictPolicy=${conflictPolicy}` : '';
  const result = await sessionResponse(await apiFetch(activeUrl + policy, { method: 'POST', signal }));
  if (result.status === 'active') throw new Error('Upload was not completed');
  return { status: result.status, filename: result.filename ?? file.name };
}
