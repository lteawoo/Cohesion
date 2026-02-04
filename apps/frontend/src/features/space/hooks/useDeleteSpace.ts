import { useState, useCallback } from 'react';
import type { DeleteSpaceResponse, SpaceApiError } from '../types';

export function useDeleteSpace() {
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  const deleteSpace = useCallback(async (id: number): Promise<DeleteSpaceResponse | null> => {
    setIsLoading(true);
    setError(null);

    try {
      const response = await fetch(`/api/spaces/${id}`, {
        method: 'DELETE',
      });

      if (!response.ok) {
        const errorData: SpaceApiError = await response.json();
        throw new Error(errorData.error || `HTTP error! status: ${response.status}`);
      }

      const data: DeleteSpaceResponse = await response.json();
      return data;
    } catch (e) {
      const error = e as Error;
      setError(error);
      throw error;
    } finally {
      setIsLoading(false);
    }
  }, []);

  return { deleteSpace, isLoading, error };
}
