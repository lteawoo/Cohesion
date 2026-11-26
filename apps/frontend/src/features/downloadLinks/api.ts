import { apiFetch } from '@/api/client';

export interface SharedFile {
  name: string;
  size: number;
  expires: number;
  items?: Array<{ id: number; name: string; size: number }>;
}

export interface DownloadLink extends SharedFile {
  id: number;
  path: string;
  token: string;
  itemCount?: number;
}

export const sharedFileEndpoint = '/api/shared-file';

export class DownloadLinkRequestError extends Error {
  readonly status: number;

  constructor(status: number) {
    super('Download link request failed');
    this.status = status;
  }
}

export async function manageDownloadLinks(spaceId: number, params: URLSearchParams, init: RequestInit) {
  const url = `/api/spaces/${spaceId}/files/download-links`;
  const response = await apiFetch(params.size ? `${url}?${params}` : url, init);
  if (!response.ok) throw new DownloadLinkRequestError(response.status);
  return response;
}

export function shareAddress(token: string) {
  const url = new URL('/share', window.location.origin);
  url.hash = token;
  return url.href;
}
