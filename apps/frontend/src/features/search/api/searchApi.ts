import { apiFetch } from "@/api/client";
import { toApiError } from "@/api/error";
import type { SearchFileResult } from "../types";

const DEFAULT_LIMIT = 80;

interface SearchFilesOptions {
  signal?: AbortSignal;
}

export async function searchFiles(
  query: string,
  limit = DEFAULT_LIMIT,
  options: SearchFilesOptions = {}
): Promise<SearchFileResult[]> {
  const trimmedQuery = query.trim();
  if (!trimmedQuery) {
    return [];
  }

  const normalizedLimit = Math.max(1, Math.min(limit, 200));
  const response = await apiFetch(
    `/api/search/files?q=${encodeURIComponent(trimmedQuery)}&limit=${normalizedLimit}`,
    { signal: options.signal }
  );
  if (!response.ok) {
    throw await toApiError(response, "검색 결과를 불러오지 못했습니다.");
  }
  return response.json() as Promise<SearchFileResult[]>;
}
