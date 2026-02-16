import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { type AuthUser, login as loginApi, logout as logoutApi, me } from '@/api/auth';
import { AuthContext, type AuthContextValue } from './context';

export const AuthProvider = ({ children }: { children: ReactNode }) => {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  const refreshSession = useCallback(async () => {
    try {
      const currentUser = await me();
      setUser(currentUser);
    } catch {
      setUser(null);
    }
  }, []);

  useEffect(() => {
    if (window.location.pathname === '/login') {
      setIsLoading(false);
      return;
    }

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
