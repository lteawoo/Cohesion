import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { login as loginApi, logout as logoutApi, me, refreshAuth } from '@/api/auth';
import { AuthContext, type AuthContextValue } from './context';

export const AuthProvider = ({ children }: { children: ReactNode }) => {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  const refreshSession = useCallback(async () => {
    try {
      const currentUser = await me();
      setUser(currentUser);
      return;
    } catch {
      // continue to refresh flow
    }

    try {
      await refreshAuth();
      const currentUser = await me();
      setUser(currentUser);
    } catch {
      setUser(null);
    }
  }, []);

  useEffect(() => {
    (async () => {
      try {
        await refreshSession();
      } finally {
        setIsLoading(false);
      }
    })();
  }, [refreshSession]);

  const login = useCallback(async (username: string, password: string) => {
    await loginApi({ username, password });
    const currentUser = await me();
    setUser(currentUser);
  }, []);

  const logout = useCallback(async () => {
    await logoutApi();
    setUser(null);
  }, []);

  const value = useMemo<AuthContextValue>(() => ({
    user,
    isLoading,
    login,
    logout,
    refreshSession,
  }), [user, isLoading, login, logout, refreshSession]);

  return (
    <AuthContext.Provider value={value}>
      {children}
    </AuthContext.Provider>
  );
};
