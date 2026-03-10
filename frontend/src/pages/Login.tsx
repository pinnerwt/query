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

  const inputClass =
    'w-full bg-white border border-slate-200 rounded-lg px-4 py-2.5 text-sm text-slate-800 placeholder:text-slate-400 focus:outline-none focus:ring-2 focus:ring-amber-500/20 focus:border-amber-500 transition-colors';

  return (
    <div class="min-h-screen flex">
      {/* Brand panel — hidden on mobile */}
      <div class="hidden lg:flex lg:w-3/5 bg-gradient-to-br from-amber-500 to-orange-600 items-center justify-center p-12">
        <div class="max-w-md text-white">
          <h1 class="text-4xl font-bold tracking-tight mb-4">Query</h1>
          <p class="text-xl text-amber-100 leading-relaxed">
            餐廳管理平台 — 菜單、QR Code、訂單，一站搞定。
          </p>
        </div>
      </div>

      {/* Form panel */}
      <div class="flex-1 flex items-center justify-center bg-stone-50 px-4">
        <div class="w-full max-w-sm">
          {/* Mobile brand header */}
          <div class="lg:hidden text-center mb-8">
            <h1 class="text-3xl font-bold text-amber-600 tracking-tight">Query</h1>
            <p class="text-sm text-slate-500 mt-1">餐廳管理平台</p>
          </div>

          <div class="bg-white shadow-lg rounded-2xl p-8">
            <h2 class="text-xl font-semibold text-slate-800 text-center mb-6">
              {isRegister ? '建立帳號' : '登入'}
            </h2>
            <form onSubmit={submit} class="space-y-4">
              {isRegister && (
                <input
                  type="text"
                  placeholder="名稱"
                  value={name}
                  onInput={(e) => setName((e.target as HTMLInputElement).value)}
                  class={inputClass}
                  required
                />
              )}
              <input
                type="email"
                placeholder="Email"
                value={email}
                onInput={(e) => setEmail((e.target as HTMLInputElement).value)}
                class={inputClass}
                required
              />
              <input
                type="password"
                placeholder="密碼"
                value={password}
                onInput={(e) => setPassword((e.target as HTMLInputElement).value)}
                class={inputClass}
                required
                minLength={6}
              />
              {error && (
                <div class="bg-red-50 border border-red-200 text-red-700 rounded-lg px-4 py-2.5 text-sm">
                  {error}
                </div>
              )}
              <button
                type="submit"
                disabled={loading}
                class="w-full bg-amber-600 text-white font-medium rounded-lg py-2.5 hover:bg-amber-700 disabled:opacity-50 transition-colors"
              >
                {loading ? '...' : isRegister ? '註冊' : '登入'}
              </button>
            </form>
            <p class="text-center text-sm mt-4 text-slate-500">
              {isRegister ? '已有帳號？' : '還沒有帳號？'}
              <button
                onClick={() => { setIsRegister(!isRegister); setError(''); }}
                class="text-amber-600 hover:text-amber-700 font-medium ml-1"
              >
                {isRegister ? '登入' : '註冊'}
              </button>
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
