import { useState } from 'preact/hooks';
import { route } from 'preact-router';
import { login, register } from '../lib/api';
import { useAuth } from '../lib/auth';

export default function Login() {
  const { setToken } = useAuth();
  const [isRegister, setIsRegister] = useState(false);
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [name, setName] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const submit = async (e: Event) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      const res = isRegister
        ? await register(email, password, name)
        : await login(email, password);
      setToken(res.token);
      route('/app/');
    } catch (err: any) {
      setError(err.message || 'Failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div class="min-h-screen flex items-center justify-center bg-gray-50 px-4">
      <div class="w-full max-w-sm">
        <h1 class="text-2xl font-bold text-center mb-6">
          {isRegister ? '建立帳號' : '登入'}
        </h1>
        <form onSubmit={submit} class="space-y-4">
          {isRegister && (
            <input
              type="text"
              placeholder="名稱"
              value={name}
              onInput={(e) => setName((e.target as HTMLInputElement).value)}
              class="w-full border rounded px-3 py-2"
              required
            />
          )}
          <input
            type="email"
            placeholder="Email"
            value={email}
            onInput={(e) => setEmail((e.target as HTMLInputElement).value)}
            class="w-full border rounded px-3 py-2"
            required
          />
          <input
            type="password"
            placeholder="密碼"
            value={password}
            onInput={(e) => setPassword((e.target as HTMLInputElement).value)}
            class="w-full border rounded px-3 py-2"
            required
            minLength={6}
          />
          {error && <p class="text-red-600 text-sm">{error}</p>}
          <button
            type="submit"
            disabled={loading}
            class="w-full bg-blue-600 text-white rounded py-2 hover:bg-blue-700 disabled:opacity-50"
          >
            {loading ? '...' : isRegister ? '註冊' : '登入'}
          </button>
        </form>
        <p class="text-center text-sm mt-4 text-gray-600">
          {isRegister ? '已有帳號？' : '還沒有帳號？'}
          <button
            onClick={() => { setIsRegister(!isRegister); setError(''); }}
            class="text-blue-600 underline ml-1"
          >
            {isRegister ? '登入' : '註冊'}
          </button>
        </p>
      </div>
    </div>
  );
}
