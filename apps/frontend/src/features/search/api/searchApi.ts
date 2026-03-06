import { apiFetch } from "@/api/client";
import { toApiError } from "@/api/error";
import type { SearchFilesResponse } from "../types";
import i18n from "@/i18n";

const DEFAULT_LIMIT = 80;
const MAX_LIMIT = 400;

interface SearchFilesOptions {
  signal?: AbortSignal;
}

export async function searchFiles(
  query: string,
  limit = DEFAULT_LIMIT,
  options: SearchFilesOptions = {}
): Promise<SearchFilesResponse> {
  const trimmedQuery = query.trim();
  if (!trimmedQuery) {
    return {
      items: [],
      limit: Math.max(1, Math.min(limit, MAX_LIMIT)),
      hasMore: false,
    };
  }

  const normalizedLimit = Math.max(1, Math.min(limit, MAX_LIMIT));
  const response = await apiFetch(
    `/api/search/files?q=${encodeURIComponent(trimmedQuery)}&limit=${normalizedLimit}`,
    { signal: options.signal }
  );
  if (!response.ok) {
    throw await toApiError(response, i18n.t('search.loadResultsFailed'));
  }
  return response.json() as Promise<SearchFilesResponse>;
}
