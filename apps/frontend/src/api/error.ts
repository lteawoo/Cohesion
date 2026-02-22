interface ErrorLikeResponse {
  message?: unknown;
  error?: unknown;
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

export async function readApiErrorMessage(response: Response, fallback: string): Promise<string> {
  try {
    const data = (await response.json()) as ErrorLikeResponse;
    const message = normalizeMessage(data.message) ?? normalizeMessage(data.error);
    if (message) {
      return message;
    }
  } catch {
    // ignore parse failure and use fallback
  }

  return `${fallback} (status: ${response.status})`;
}

export async function toApiError(response: Response, fallback: string): Promise<Error> {
  const message = await readApiErrorMessage(response, fallback);
  return new Error(message);
}

