import { newSubmissionKey } from './api';

export interface PendingSubmission {
  key: string;
  name: string;
  size: number;
  lastModified: number;
}

export function startSubmission(file: File): PendingSubmission {
  return { key: newSubmissionKey(), name: file.name, size: file.size, lastModified: file.lastModified };
}

export function matchesSubmission(file: File, submission: PendingSubmission) {
  return file.name === submission.name && file.size === submission.size && file.lastModified === submission.lastModified;
}

function storageKey(token: string) {
  return `cohesion.fileRequest.${token}`;
}

export function readSubmission(token: string): PendingSubmission | null {
  try {
    const value = JSON.parse(sessionStorage.getItem(storageKey(token)) ?? 'null');
    if (value && typeof value.key === 'string' && /^[A-Za-z0-9_-]{32}$/.test(value.key)
      && typeof value.name === 'string' && Number.isSafeInteger(value.size) && value.size >= 0
      && Number.isSafeInteger(value.lastModified)) return value;
  } catch { /* Storage can be unavailable in a restricted browser. */ }
  return null;
}

export function rememberSubmission(token: string, value: PendingSubmission | null) {
  try {
    if (value) sessionStorage.setItem(storageKey(token), JSON.stringify(value));
    else sessionStorage.removeItem(storageKey(token));
  } catch { /* The current page can still finish its upload using the in-memory key. */ }
}
