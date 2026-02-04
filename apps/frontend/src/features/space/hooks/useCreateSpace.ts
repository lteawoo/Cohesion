import { useState, useCallback } from 'react';
import type { CreateSpaceRequest, CreateSpaceResponse, SpaceApiError } from '../types';

export function useCreateSpace() {
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  const createSpace = useCallback(async (
    request: CreateSpaceRequest
  ): Promise<CreateSpaceResponse | null> => {
    setIsLoading(true);
    setError(null);

    try {
      const response = await fetch('/api/spaces', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(request),
      });

      if (!response.ok) {
        // 에러 응답 파싱
        const errorData: SpaceApiError = await response.json();
        throw new Error(errorData.error || `HTTP error! status: ${response.status}`);
      }

      const data: CreateSpaceResponse = await response.json();
      return data;
    } catch (e) {
      const error = e as Error;
      setError(error);
      throw error; // 호출자가 에러를 처리할 수 있도록 다시 던짐
    } finally {
      setIsLoading(false);
    }
  }, []);

  return { createSpace, isLoading, error };
}
