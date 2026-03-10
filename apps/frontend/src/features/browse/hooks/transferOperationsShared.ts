import type { TreeInvalidationTarget } from '@/stores/browseStore';
import type { Space } from '@/features/space/types';

export type UploadConflictPolicy = 'overwrite' | 'rename' | 'skip';
export type TransferMode = 'move' | 'copy';
export type TransferConflictPolicy = 'overwrite' | 'rename' | 'skip';
export type UploadSource = File | File[] | FileList;
export type UploadStatus = 'uploaded' | 'skipped';
export type TrashConflictPolicy = 'overwrite' | 'rename' | 'skip';

export interface TrashItem {
  id: number;
  originalPath: string;
  itemName: string;
  isDir: boolean;
  itemSize: number;
  deletedBy: string;
  deletedAt: string;
}

export interface DownloadTicketResponse {
  downloadUrl?: string;
  fileName?: string;
}

export interface UploadResponsePayload {
  message?: string;
  filename?: string;
  status?: UploadStatus;
}

export interface UploadResult {
  status: UploadStatus;
  filename: string;
}

export interface ArchiveDownloadJobResponse {
  jobId?: string;
  status?: 'queued' | 'running' | 'ready' | 'failed' | 'expired' | 'canceled';
  fileName?: string;
  sourceCount?: number;
  totalItems?: number;
  processedItems?: number;
  totalSourceBytes?: number;
  processedSourceBytes?: number;
  failureReason?: string;
  artifactSize?: number;
}

export interface DownloadTransferOptions {
  loaded?: number;
  total?: number;
  message?: string;
  spaceId?: number;
}

export interface UploadExecutionTask {
  transferId: string;
  file: File;
}

export interface UploadExecutionResult {
  transferId: string;
  file: File;
  outcome: 'uploaded' | 'skipped' | 'failed' | 'canceled';
  filename?: string;
  message?: string;
}

export interface UploadBatchQueueEntry {
  tasks: UploadExecutionTask[];
  targetPath: string;
  settledResults: UploadExecutionResult[];
  resolve: (results: UploadExecutionResult[]) => void;
  reject: (error: unknown) => void;
}

export interface ArchiveQueueEntry {
  transferId: string;
  archiveSpaceId: number;
  relativePaths: string[];
  fallbackArchiveName: string;
  resolve: () => void;
  reject: (error: unknown) => void;
}

export interface UploadSummary {
  uploaded: number;
  skipped: number;
  failed: number;
}

export interface TransferFailurePayload {
  path?: string;
  reason?: string;
  code?: string;
}

export interface TransferResponsePayload {
  succeeded?: string[];
  skipped?: string[];
  failed?: TransferFailurePayload[];
}

export interface TransferSummary {
  succeeded: number;
  skipped: number;
  failed: number;
}

export interface TransferOperationResult {
  summary: TransferSummary;
  succeededSources: string[];
  failedReasons: string[];
  abortedByUser: boolean;
}

export interface TransferConflictSelection {
  policy: TransferConflictPolicy;
  applyToRemaining: boolean;
}

export interface TrashListResponsePayload {
  items?: TrashItem[];
}

export interface TrashRestoreSuccessPayload {
  id?: number;
  originalPath?: string;
}

export interface TrashRestoreFailurePayload {
  id?: number;
  originalPath?: string;
  reason?: string;
  code?: string;
}

export interface TrashRestoreResponsePayload {
  succeeded?: TrashRestoreSuccessPayload[];
  skipped?: TrashRestoreSuccessPayload[];
  failed?: TrashRestoreFailurePayload[];
}

export interface TrashDeleteSuccessPayload {
  id?: number;
}

export interface TrashDeleteFailurePayload {
  id?: number;
  reason?: string;
}

export interface TrashDeleteResponsePayload {
  succeeded?: TrashDeleteSuccessPayload[];
  failed?: TrashDeleteFailurePayload[];
}

export interface TrashEmptyFailurePayload {
  id?: number;
  reason?: string;
}

export interface TrashEmptyResponsePayload {
  removed?: number;
  failed?: TrashEmptyFailurePayload[];
}

export interface TransferMessageApi {
  error: (content: string) => void;
  info: (content: string) => void;
  success: (content: string) => void;
  warning: (content: string) => void;
}

export type Translate = (key: string, values?: Record<string, string | number>) => string;

export const MAX_ACTIVE_UPLOAD_BATCHES = 2;
export const MAX_ACTIVE_ARCHIVE_TASKS = 2;

export function normalizeRelativePath(path: string): string {
  return path.replace(/^\/+/, '').replace(/\/+$/, '');
}

export function getParentPath(path: string): string {
  if (!path) return '';
  const normalizedPath = normalizeRelativePath(path);
  if (!normalizedPath) return '';
  const lastSlashIndex = normalizedPath.lastIndexOf('/');
  if (lastSlashIndex <= 0) return '';
  return normalizedPath.slice(0, lastSlashIndex);
}

export function createInvalidationTarget(path: string, space?: Space): TreeInvalidationTarget {
  return {
    path,
    spaceId: space?.id,
  };
}

export function triggerBrowserDownloadFromUrl(url: string, fileName?: string): void {
  const anchor = document.createElement('a');
  anchor.href = url;
  if (fileName) {
    anchor.download = fileName;
  }
  document.body.appendChild(anchor);
  anchor.click();
  document.body.removeChild(anchor);
}

export function wasPageReloaded(): boolean {
  if (typeof window === 'undefined' || typeof performance === 'undefined') {
    return false;
  }
  const navigationEntries = performance.getEntriesByType('navigation') as PerformanceNavigationTiming[];
  if (navigationEntries.length > 0) {
    return navigationEntries[0]?.type === 'reload';
  }
  const legacyNavigation = performance.navigation;
  return typeof legacyNavigation !== 'undefined' && legacyNavigation.type === legacyNavigation.TYPE_RELOAD;
}

export function normalizeUploadSource(files: UploadSource): File[] {
  if (files instanceof File) {
    return [files];
  }
  if (Array.isArray(files)) {
    return files;
  }
  return Array.from(files);
}

export function isDestinationConflictFailure(item: { code?: string }): boolean {
  return item.code === 'destination_exists';
}

export function createTransferId(): string {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
    return crypto.randomUUID();
  }
  return `transfer-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

export function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === 'AbortError';
}
