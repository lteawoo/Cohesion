import { apiFetch } from './client';
import { toApiError } from './error';

export type AuditResult = 'success' | 'partial' | 'failure' | 'denied';

export interface AuditLogItem {
  id: number;
  occurredAt: string;
  actor: string;
  action: string;
  result: AuditResult;
  target: string;
  requestId: string;
  spaceId?: number;
  metadata: Record<string, unknown>;
}

export interface AuditLogListResponse {
  items: AuditLogItem[];
  page: number;
  pageSize: number;
  total: number;
}

export interface AuditLogListParams {
  page?: number;
  pageSize?: number;
  from?: string;
  to?: string;
  user?: string;
  action?: string;
  spaceId?: number;
  result?: AuditResult;
}

function buildListQuery(params: AuditLogListParams): string {
  const query = new URLSearchParams();
  if (params.page && params.page > 0) {
    query.set('page', String(params.page));
  }
  if (params.pageSize && params.pageSize > 0) {
    query.set('pageSize', String(params.pageSize));
  }
  if (params.from) {
    query.set('from', params.from);
  }
  if (params.to) {
    query.set('to', params.to);
  }
  if (params.user) {
    query.set('user', params.user);
  }
  if (params.action) {
    query.set('action', params.action);
  }
  if (params.spaceId && params.spaceId > 0) {
    query.set('spaceId', String(params.spaceId));
  }
  if (params.result) {
    query.set('result', params.result);
  }
  return query.toString();
}

export async function listAuditLogs(params: AuditLogListParams): Promise<AuditLogListResponse> {
  const query = buildListQuery(params);
  const url = query ? `/api/audit/logs?${query}` : '/api/audit/logs';
  const response = await apiFetch(url);
  if (!response.ok) {
    throw await toApiError(response, 'Failed to load audit logs');
  }
  return response.json();
}

export async function getAuditLog(id: number): Promise<AuditLogItem> {
  const response = await apiFetch(`/api/audit/logs/${id}`);
  if (!response.ok) {
    throw await toApiError(response, 'Failed to load audit log detail');
  }
  return response.json();
}
