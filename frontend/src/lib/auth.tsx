import { createContext } from 'preact';
import { useContext, useState, useEffect, useCallback } from 'preact/hooks';
import type { Owner } from './api';
import { getMe } from './api';

interface AuthCtx {
  owner: Owner | null;
  token: string | null;
  loading: boolean;
  setToken: (t: string | null) => void;
  logout: () => void;
}

const AuthContext = createContext<AuthCtx>({
  owner: null,
  token: null,
  loading: true,
  setToken: () => {},
  logout: () => {},
});

export function AuthProvider({ children }: { children: preact.ComponentChildren }) {
  const [token, setTokenState] = useState<string | null>(() => localStorage.getItem('token'));
  const [owner, setOwner] = useState<Owner | null>(null);
  const [loading, setLoading] = useState(true);

  const setToken = useCallback((t: string | null) => {
    if (t) localStorage.setItem('token', t);
    else localStorage.removeItem('token');
    setTokenState(t);
  }, []);

  const logout = useCallback(() => {
    setToken(null);
    setOwner(null);
  }, [setToken]);

  useEffect(() => {
    if (!token) {
      setLoading(false);
      setOwner(null);
      return;
    }
    setLoading(true);
    getMe()
      .then(setOwner)
      .catch(() => {
        setToken(null);
        setOwner(null);
      })
      .finally(() => setLoading(false));
  }, [token, setToken]);

  return (
    <AuthContext.Provider value={{ owner, token, loading, setToken, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export const useAuth = () => useContext(AuthContext);
