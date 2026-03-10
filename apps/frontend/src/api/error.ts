interface ErrorLikeResponse {
  message?: unknown;
  error?: unknown;
  code?: unknown;
  hint?: unknown;
}

function normalizeMessage(raw: unknown): string | null {
  if (typeof raw !== 'string') {
    return null;
  }
  const trimmed = raw.trim();
  if (trimmed === '') {
    return null;
  }
  return trimmed;
}

function normalizeOptionalString(raw: unknown): string | undefined {
  return normalizeMessage(raw) ?? undefined;
}

interface ApiErrorPayload {
  code?: string;
  hint?: string;
  message: string;
}

export class ApiError extends Error {
  code?: string;
  hint?: string;
  status: number;

  constructor(message: string, options: { status: number; code?: string; hint?: string }) {
    super(message);
    this.name = 'ApiError';
    this.status = options.status;
    this.code = options.code;
    this.hint = options.hint;
  }
}

async function readApiErrorPayload(response: Response, fallback: string): Promise<ApiErrorPayload> {
  try {
    const data = (await response.json()) as ErrorLikeResponse;
    const message = normalizeMessage(data.message) ?? normalizeMessage(data.error);
    return {
      message: message ?? `${fallback} (status: ${response.status})`,
      code: normalizeOptionalString(data.code),
      hint: normalizeOptionalString(data.hint),
    };
  } catch {
    // ignore parse failure and use fallback
  }

  return { message: `${fallback} (status: ${response.status})` };
}

export async function readApiErrorMessage(response: Response, fallback: string): Promise<string> {
  return (await readApiErrorPayload(response, fallback)).message;
}

export async function toApiError(response: Response, fallback: string): Promise<Error> {
  const payload = await readApiErrorPayload(response, fallback);
  return new ApiError(payload.message, {
    status: response.status,
    code: payload.code,
    hint: payload.hint,
  });
}
