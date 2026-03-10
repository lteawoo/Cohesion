import { ApiError } from '@/api/error';

const BROWSE_PERMISSION_DENIED_MESSAGE = 'permission denied';

type Translate = (key: string) => string;

function isBrowsePermissionDeniedError(error: Error): error is ApiError {
  return error instanceof ApiError
    && error.status === 403
    && error.message.trim().toLowerCase() === BROWSE_PERMISSION_DENIED_MESSAGE;
}

export function buildBrowsePermissionGuidanceMessage(t: Translate): string {
  return `${t('browseApi.permissionDeniedReason')} ${t('directorySetup.validation.permissionDeniedHint')}`.trim();
}

export function normalizeBrowseError(
  error: unknown,
  options: {
    fallbackMessage: string;
    t: Translate;
  },
): Error {
  const candidate = error instanceof Error ? error : new Error(options.fallbackMessage);
  if (!isBrowsePermissionDeniedError(candidate)) {
    return candidate;
  }

  return new ApiError(buildBrowsePermissionGuidanceMessage(options.t), {
    status: candidate.status,
    code: candidate.code,
    hint: candidate.hint,
  });
}
