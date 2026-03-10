import { createContext } from 'preact';
import { useContext, useState, useEffect, useCallback } from 'preact/hooks';
import type { Owner } from './api';
import { getMe, logout as apiLogout } from './api';

interface AuthCtx {
  owner: Owner | null;
  loading: boolean;
  refreshAuth: () => void;
  logout: () => void;
}

const AuthContext = createContext<AuthCtx>({
  owner: null,
  loading: true,
  refreshAuth: () => {},
  logout: () => {},
});

export function AuthProvider({ children }: { children: preact.ComponentChildren }) {
  const [owner, setOwner] = useState<Owner | null>(null);
  const [loading, setLoading] = useState(true);

  const refreshAuth = useCallback(() => {
    setLoading(true);
    getMe()
      .then(setOwner)
      .catch(() => setOwner(null))
      .finally(() => setLoading(false));
  }, []);

  const logout = useCallback(() => {
    apiLogout()
      .catch(() => {})
      .finally(() => {
        setOwner(null);
      });
  }, []);

  // Check session on mount
  useEffect(() => {
    refreshAuth();
  }, [refreshAuth]);

  return (
    <AuthContext.Provider value={{ owner, loading, refreshAuth, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export const useAuth = () => useContext(AuthContext);
