import { createContext } from 'react';
import type { AuthUser } from '@/api/auth';

export interface AuthContextValue {
  user: AuthUser | null;
  isLoading: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  refreshSession: () => Promise<void>;
}

export const AuthContext = createContext<AuthContextValue | undefined>(undefined);
