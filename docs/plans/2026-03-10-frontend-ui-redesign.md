# Frontend UI Redesign Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Transform the plain Preact + Tailwind frontend into a polished restaurant management dashboard with warm amber branding, persistent sidebar navigation, and purpose-built layouts for menu editing and order management.

**Architecture:** Build a Layout shell component (sidebar + topbar) that wraps all authenticated routes. Redesign each page with the new color system and card-based layouts. Add SortableJS for menu drag-and-drop. All styling is Tailwind utility classes — no component library.

**Tech Stack:** Preact 10, preact-router 4, Tailwind CSS 4, SortableJS, Vite 7, TypeScript 5.9

---

### Task 1: Install SortableJS and Update Base Styles

**Files:**
- Modify: `frontend/package.json`
- Modify: `frontend/src/index.css`

**Step 1: Install SortableJS**

Run:
```bash
cd frontend && npm install sortablejs && npm install -D @types/sortablejs
```

**Step 2: Update base CSS**

Replace `frontend/src/index.css` contents with:

```css
@import "tailwindcss";

body {
  font-family: system-ui, -apple-system, 'Segoe UI', sans-serif;
  -webkit-font-smoothing: antialiased;
}
```

**Step 3: Verify build**

Run: `cd frontend && npm run build`
Expected: Build succeeds with no errors.

**Step 4: Commit**

```bash
git add frontend/package.json frontend/package-lock.json frontend/src/index.css
git commit -m "chore: install sortablejs and update base font styles"
```

---

### Task 2: Create Shared UI Components (Toggle, Modal, Skeleton)

**Files:**
- Create: `frontend/src/components/Toggle.tsx`
- Create: `frontend/src/components/Modal.tsx`
- Create: `frontend/src/components/Skeleton.tsx`

**Step 1: Create Toggle component**

Write `frontend/src/components/Toggle.tsx`:

```tsx
interface ToggleProps {
  enabled: boolean;
  onChange: (enabled: boolean) => void;
  label?: string;
}

export default function Toggle({ enabled, onChange, label }: ToggleProps) {
  return (
    <label class="flex items-center gap-3 cursor-pointer">
      <button
        type="button"
        role="switch"
        aria-checked={enabled}
        onClick={() => onChange(!enabled)}
        class={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
          enabled ? 'bg-amber-600' : 'bg-slate-200'
        }`}
      >
        <span
          class={`inline-block h-4 w-4 rounded-full bg-white transition-transform shadow-sm ${
            enabled ? 'translate-x-6' : 'translate-x-1'
          }`}
        />
      </button>
      {label && <span class="text-sm text-slate-700">{label}</span>}
    </label>
  );
}
```

**Step 2: Create Modal component**

Write `frontend/src/components/Modal.tsx`:

```tsx
import type { ComponentChildren } from 'preact';

interface ModalProps {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ComponentChildren;
}

export default function Modal({ open, onClose, title, children }: ModalProps) {
  if (!open) return null;

  return (
    <div class="fixed inset-0 z-50 flex items-center justify-center">
      <div class="absolute inset-0 bg-black/30 backdrop-blur-sm" onClick={onClose} />
      <div class="relative bg-white rounded-2xl shadow-xl w-full max-w-md mx-4 p-6">
        <div class="flex items-center justify-between mb-4">
          <h2 class="text-lg font-semibold text-slate-800">{title}</h2>
          <button onClick={onClose} class="text-slate-400 hover:text-slate-600 text-xl leading-none">&times;</button>
        </div>
        {children}
      </div>
    </div>
  );
}
```

**Step 3: Create Skeleton components**

Write `frontend/src/components/Skeleton.tsx`:

```tsx
export function SkeletonCard() {
  return (
    <div class="bg-white rounded-xl shadow-sm border border-slate-100 p-4 animate-pulse">
      <div class="h-1.5 bg-amber-200 rounded -mx-4 -mt-4 mb-4" />
      <div class="h-5 bg-slate-200 rounded w-3/4 mb-3" />
      <div class="h-4 bg-slate-100 rounded w-1/2 mb-4" />
      <div class="border-t border-slate-100 -mx-4 px-4 pt-3 mt-3 flex gap-4">
        <div class="h-4 bg-slate-100 rounded w-12" />
        <div class="h-4 bg-slate-100 rounded w-12" />
        <div class="h-4 bg-slate-100 rounded w-12" />
      </div>
    </div>
  );
}

export function SkeletonList({ rows = 3 }: { rows?: number }) {
  return (
    <div class="space-y-3 animate-pulse">
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} class="bg-white rounded-xl shadow-sm border border-slate-100 p-4">
          <div class="h-4 bg-slate-200 rounded w-2/3 mb-2" />
          <div class="h-3 bg-slate-100 rounded w-1/3" />
        </div>
      ))}
    </div>
  );
}
```

**Step 4: Verify build**

Run: `cd frontend && npm run build`
Expected: Build succeeds. Components aren't used yet but must compile.

**Step 5: Commit**

```bash
git add frontend/src/components/
git commit -m "feat: add Toggle, Modal, and Skeleton shared components"
```

---

### Task 3: Create Layout Shell (Sidebar + Topbar)

**Files:**
- Create: `frontend/src/components/Layout.tsx`
- Modify: `frontend/src/app.tsx`
- Modify: `frontend/src/lib/auth.tsx`

The Layout component needs access to the current route path to highlight the active nav item. preact-router provides `getCurrentUrl()` for this purpose. The sidebar nav items change based on whether we're inside a restaurant (URL contains `/restaurants/:id`).

**Step 1: Create Layout component**

Write `frontend/src/components/Layout.tsx`:

```tsx
import { useState } from 'preact/hooks';
import { route, getCurrentUrl } from 'preact-router';
import { useAuth } from '../lib/auth';
import type { ComponentChildren } from 'preact';

// Inline SVG icons (Heroicons outline, 24x24)
const icons = {
  dashboard: <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M3.75 6A2.25 2.25 0 0 1 6 3.75h2.25A2.25 2.25 0 0 1 10.5 6v2.25a2.25 2.25 0 0 1-2.25 2.25H6a2.25 2.25 0 0 1-2.25-2.25V6ZM3.75 15.75A2.25 2.25 0 0 1 6 13.5h2.25a2.25 2.25 0 0 1 2.25 2.25V18a2.25 2.25 0 0 1-2.25 2.25H6A2.25 2.25 0 0 1 3.75 18v-2.25ZM13.5 6a2.25 2.25 0 0 1 2.25-2.25H18A2.25 2.25 0 0 1 20.25 6v2.25A2.25 2.25 0 0 1 18 10.5h-2.25a2.25 2.25 0 0 1-2.25-2.25V6ZM13.5 15.75a2.25 2.25 0 0 1 2.25-2.25H18a2.25 2.25 0 0 1 2.25 2.25V18A2.25 2.25 0 0 1 18 20.25h-2.25a2.25 2.25 0 0 1-2.25-2.25v-2.25Z" /></svg>,
  settings: <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.325.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 0 1 1.37.49l1.296 2.247a1.125 1.125 0 0 1-.26 1.431l-1.003.827c-.293.241-.438.613-.43.992a7.723 7.723 0 0 1 0 .255c-.008.378.137.75.43.991l1.004.827c.424.35.534.955.26 1.43l-1.298 2.247a1.125 1.125 0 0 1-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.47 6.47 0 0 1-.22.128c-.331.183-.581.495-.644.869l-.213 1.281c-.09.543-.56.94-1.11.94h-2.594c-.55 0-1.019-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 0 1-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 0 1-1.369-.49l-1.297-2.247a1.125 1.125 0 0 1 .26-1.431l1.004-.827c.292-.24.437-.613.43-.991a6.932 6.932 0 0 1 0-.255c.007-.38-.138-.751-.43-.992l-1.004-.827a1.125 1.125 0 0 1-.26-1.43l1.297-2.247a1.125 1.125 0 0 1 1.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.086.22-.128.332-.183.582-.495.644-.869l.214-1.28Z" /><path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" /></svg>,
  menu: <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M3.75 5.25h16.5m-16.5 4.5h16.5m-16.5 4.5h16.5m-16.5 4.5h16.5" /></svg>,
  orders: <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12h3.75M9 15h3.75M9 18h3.75m3 .75H18a2.25 2.25 0 0 0 2.25-2.25V6.108c0-1.135-.845-2.098-1.976-2.192a48.424 48.424 0 0 0-1.123-.08m-5.801 0c-.065.21-.1.433-.1.664 0 .414.336.75.75.75h4.5a.75.75 0 0 0 .75-.75 2.25 2.25 0 0 0-.1-.664m-5.8 0A2.251 2.251 0 0 1 13.5 2.25H15a2.25 2.25 0 0 1 2.15 1.586m-5.8 0c-.376.023-.75.05-1.124.08C9.095 4.01 8.25 4.973 8.25 6.108V8.25m0 0H4.875c-.621 0-1.125.504-1.125 1.125v11.25c0 .621.504 1.125 1.125 1.125h9.75c.621 0 1.125-.504 1.125-1.125V9.375c0-.621-.504-1.125-1.125-1.125H8.25ZM6.75 12h.008v.008H6.75V12Zm0 3h.008v.008H6.75V15Zm0 3h.008v.008H6.75V18Z" /></svg>,
  hamburger: <svg class="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25h16.5" /></svg>,
  close: <svg class="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" /></svg>,
  logout: <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M15.75 9V5.25A2.25 2.25 0 0 0 13.5 3h-6a2.25 2.25 0 0 0-2.25 2.25v13.5A2.25 2.25 0 0 0 7.5 21h6a2.25 2.25 0 0 0 2.25-2.25V15m3 0 3-3m0 0-3-3m3 3H9" /></svg>,
  back: <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M10.5 19.5 3 12m0 0 7.5-7.5M3 12h18" /></svg>,
};

interface NavItem {
  label: string;
  path: string;
  icon: preact.JSX.Element;
}

export default function Layout({ children }: { children: ComponentChildren }) {
  const { owner, logout } = useAuth();
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const url = getCurrentUrl();

  // Extract restaurant ID from URL if inside a restaurant context
  const restaurantMatch = url.match(/\/app\/restaurants\/(\d+)/);
  const restaurantId = restaurantMatch ? restaurantMatch[1] : null;

  const navItems: NavItem[] = restaurantId
    ? [
        { label: '設定', path: `/app/restaurants/${restaurantId}`, icon: icons.settings },
        { label: '菜單', path: `/app/restaurants/${restaurantId}/menu`, icon: icons.menu },
        { label: '訂單', path: `/app/restaurants/${restaurantId}/orders`, icon: icons.orders },
      ]
    : [];

  const isActive = (path: string) => url === path;

  const navigate = (path: string) => {
    route(path);
    setSidebarOpen(false);
  };

  return (
    <div class="min-h-screen bg-stone-50">
      {/* Mobile overlay */}
      {sidebarOpen && (
        <div class="fixed inset-0 bg-black/30 backdrop-blur-sm z-40 lg:hidden" onClick={() => setSidebarOpen(false)} />
      )}

      {/* Sidebar */}
      <aside class={`fixed inset-y-0 left-0 w-60 bg-slate-900 flex flex-col z-50 transition-transform duration-200 lg:translate-x-0 ${sidebarOpen ? 'translate-x-0' : '-translate-x-full'}`}>
        {/* Logo */}
        <div class="flex items-center justify-between h-16 px-5 border-b border-slate-800">
          <button onClick={() => navigate('/app/')} class="text-lg font-bold text-white tracking-tight">Query</button>
          <button class="text-slate-400 hover:text-white lg:hidden" onClick={() => setSidebarOpen(false)}>{icons.close}</button>
        </div>

        {/* Nav */}
        <nav class="flex-1 py-4 px-3 space-y-1 overflow-y-auto">
          {/* Dashboard link (always shown) */}
          <button
            onClick={() => navigate('/app/')}
            class={`flex items-center gap-3 w-full px-3 py-2.5 rounded-lg text-sm transition-colors ${
              !restaurantId && url === '/app/'
                ? 'bg-slate-800 text-white border-l-2 border-amber-400'
                : 'text-slate-300 hover:bg-slate-800 hover:text-white'
            }`}
          >
            {icons.dashboard}
            <span>Dashboard</span>
          </button>

          {/* Restaurant context nav */}
          {restaurantId && (
            <>
              <button
                onClick={() => navigate('/app/')}
                class="flex items-center gap-3 w-full px-3 py-2.5 rounded-lg text-sm text-slate-400 hover:bg-slate-800 hover:text-white transition-colors"
              >
                {icons.back}
                <span>所有餐廳</span>
              </button>
              <div class="border-t border-slate-800 my-2" />
              {navItems.map((item) => (
                <button
                  key={item.path}
                  onClick={() => navigate(item.path)}
                  class={`flex items-center gap-3 w-full px-3 py-2.5 rounded-lg text-sm transition-colors ${
                    isActive(item.path)
                      ? 'bg-slate-800 text-white border-l-2 border-amber-400'
                      : 'text-slate-300 hover:bg-slate-800 hover:text-white'
                  }`}
                >
                  {item.icon}
                  <span>{item.label}</span>
                </button>
              ))}
            </>
          )}
        </nav>

        {/* User / Logout */}
        <div class="border-t border-slate-800 p-4">
          <div class="flex items-center gap-3">
            <div class="w-8 h-8 bg-amber-600 rounded-full flex items-center justify-center text-sm font-bold text-white">
              {owner?.name?.charAt(0) || '?'}
            </div>
            <div class="flex-1 min-w-0">
              <p class="text-sm font-medium text-white truncate">{owner?.name}</p>
              <p class="text-xs text-slate-400 truncate">{owner?.email}</p>
            </div>
          </div>
          <button
            onClick={logout}
            class="flex items-center gap-2 mt-3 w-full px-3 py-2 rounded-lg text-sm text-slate-400 hover:bg-slate-800 hover:text-red-400 transition-colors"
          >
            {icons.logout}
            <span>登出</span>
          </button>
        </div>
      </aside>

      {/* Main content */}
      <div class="lg:ml-60 min-h-screen">
        {/* Mobile topbar */}
        <header class="sticky top-0 z-30 bg-white border-b border-slate-200 h-16 flex items-center px-4 lg:hidden">
          <button onClick={() => setSidebarOpen(true)} class="text-slate-600 hover:text-slate-800">
            {icons.hamburger}
          </button>
          <span class="ml-3 font-bold text-slate-800 tracking-tight">Query</span>
        </header>

        {/* Page content */}
        <main class="p-4 lg:p-6">
          {children}
        </main>
      </div>
    </div>
  );
}
```

**Step 2: Add `email` to AuthContext consumer access**

The sidebar displays `owner?.email`. Check `auth.tsx` — the `Owner` type from `api.ts` already has `email`. No change needed to auth.tsx since `owner` is already typed as `Owner | null` and includes email.

**Step 3: Update app.tsx to wrap routes in Layout**

Replace `frontend/src/app.tsx` contents:

```tsx
import Router from 'preact-router';
import { useAuth, AuthProvider } from './lib/auth';
import Login from './pages/Login';
import Dashboard from './pages/Dashboard';
import RestaurantEdit from './pages/RestaurantEdit';
import MenuEditor from './pages/MenuEditor';
import Orders from './pages/Orders';
import Layout from './components/Layout';

function AppRoutes() {
  const { owner, loading } = useAuth();

  if (loading) {
    return (
      <div class="flex items-center justify-center min-h-screen bg-stone-50">
        <div class="animate-pulse text-slate-400 text-sm">Loading...</div>
      </div>
    );
  }

  if (!owner) {
    return <Login />;
  }

  return (
    <Layout>
      <Router>
        <Dashboard path="/app/" />
        <RestaurantEdit path="/app/restaurants/:id" />
        <MenuEditor path="/app/restaurants/:id/menu" />
        <Orders path="/app/restaurants/:id/orders" />
        <Dashboard default />
      </Router>
    </Layout>
  );
}

export function App() {
  return (
    <AuthProvider>
      <AppRoutes />
    </AuthProvider>
  );
}
```

**Step 4: Verify build**

Run: `cd frontend && npm run build`
Expected: Build succeeds.

**Step 5: Manual test**

Open `http://localhost:5173/app/` in browser. Verify:
- Login page shows (no sidebar) when not authenticated
- After login, sidebar appears on the left (desktop)
- On mobile viewport, sidebar is hidden, hamburger shows in top bar
- Clicking nav items navigates correctly
- Active nav item has amber left border

**Step 6: Commit**

```bash
git add frontend/src/components/Layout.tsx frontend/src/app.tsx
git commit -m "feat: add app shell with sidebar navigation and mobile drawer"
```

---

### Task 4: Redesign Login Page

**Files:**
- Modify: `frontend/src/pages/Login.tsx`

**Step 1: Rewrite Login.tsx**

Replace entire `frontend/src/pages/Login.tsx` with:

```tsx
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
```

**Step 2: Verify build**

Run: `cd frontend && npm run build`
Expected: Build succeeds.

**Step 3: Commit**

```bash
git add frontend/src/pages/Login.tsx
git commit -m "feat: redesign login page with split brand layout"
```

---

### Task 5: Redesign Dashboard Page

**Files:**
- Modify: `frontend/src/pages/Dashboard.tsx`

**Step 1: Rewrite Dashboard.tsx**

Replace entire `frontend/src/pages/Dashboard.tsx` with:

```tsx
import { useState, useEffect } from 'preact/hooks';
import { route } from 'preact-router';
import { listMyRestaurants, createRestaurant, deleteRestaurant } from '../lib/api';
import type { Restaurant } from '../lib/api';
import type { RoutableProps } from '../lib/route';
import Modal from '../components/Modal';
import { SkeletonCard } from '../components/Skeleton';

export default function Dashboard(_props: RoutableProps) {
  const [restaurants, setRestaurants] = useState<Restaurant[]>([]);
  const [showCreate, setShowCreate] = useState(false);
  const [newName, setNewName] = useState('');
  const [newAddress, setNewAddress] = useState('');
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [deletingId, setDeletingId] = useState<number | null>(null);

  const load = () => {
    listMyRestaurants()
      .then(setRestaurants)
      .catch(() => {})
      .finally(() => setLoading(false));
  };

  useEffect(load, []);

  const handleCreate = async (e: Event) => {
    e.preventDefault();
    if (!newName.trim() || creating) return;
    setCreating(true);
    try {
      await createRestaurant({ name: newName, address: newAddress });
      setNewName('');
      setNewAddress('');
      setShowCreate(false);
      load();
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (id: number) => {
    if (deletingId !== null) return;
    if (!confirm('確定刪除？')) return;
    setDeletingId(id);
    try {
      await deleteRestaurant(id);
      load();
    } finally {
      setDeletingId(null);
    }
  };

  const inputClass =
    'w-full bg-white border border-slate-200 rounded-lg px-4 py-2.5 text-sm text-slate-800 placeholder:text-slate-400 focus:outline-none focus:ring-2 focus:ring-amber-500/20 focus:border-amber-500 transition-colors';

  const publishedCount = restaurants.filter((r) => r.is_published).length;

  return (
    <div class="max-w-5xl mx-auto">
      {/* Header */}
      <div class="flex items-center justify-between mb-6">
        <h1 class="text-2xl font-bold text-slate-800 tracking-tight">我的餐廳</h1>
        <button
          onClick={() => setShowCreate(true)}
          class="bg-amber-600 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-amber-700 transition-colors"
        >
          + 新增餐廳
        </button>
      </div>

      {/* Stats */}
      {!loading && restaurants.length > 0 && (
        <div class="grid grid-cols-2 gap-4 mb-6">
          <div class="bg-white rounded-xl p-4 shadow-sm border border-slate-100">
            <p class="text-xs text-slate-400 uppercase tracking-wide">餐廳數量</p>
            <p class="text-2xl font-bold text-slate-800 mt-1">{restaurants.length}</p>
          </div>
          <div class="bg-white rounded-xl p-4 shadow-sm border border-slate-100">
            <p class="text-xs text-slate-400 uppercase tracking-wide">已發布</p>
            <p class="text-2xl font-bold text-emerald-600 mt-1">{publishedCount}</p>
          </div>
        </div>
      )}

      {/* Restaurant cards */}
      {loading ? (
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {Array.from({ length: 3 }, (_, i) => <SkeletonCard key={i} />)}
        </div>
      ) : restaurants.length === 0 ? (
        <div class="text-center py-16">
          <p class="text-slate-400 mb-4">還沒有餐廳，點擊上方按鈕建立</p>
        </div>
      ) : (
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {restaurants.map((r) => (
            <div key={r.id} class="bg-white rounded-xl shadow-sm border border-slate-100 overflow-hidden hover:shadow-md transition-shadow">
              {/* Amber accent strip */}
              <div class="h-1.5 bg-amber-500" />
              <div class="p-4">
                <div class="flex items-start justify-between mb-2">
                  <h2
                    class="font-semibold text-slate-800 cursor-pointer hover:text-amber-600 transition-colors"
                    onClick={() => route(`/app/restaurants/${r.id}`)}
                  >
                    {r.name}
                  </h2>
                  <span
                    class={`text-xs font-medium px-2 py-0.5 rounded-full ${
                      r.is_published
                        ? 'bg-emerald-100 text-emerald-700'
                        : 'bg-slate-100 text-slate-500'
                    }`}
                  >
                    {r.is_published ? '已發布' : '草稿'}
                  </span>
                </div>
                <p class="text-sm text-slate-500 mb-3">{r.address || '未設定地址'}</p>
              </div>
              {/* Card footer */}
              <div class="border-t border-slate-100 px-4 py-3 flex items-center justify-between">
                <div class="flex gap-4">
                  <button onClick={() => route(`/app/restaurants/${r.id}`)} class="text-xs text-slate-500 hover:text-amber-600 font-medium transition-colors">設定</button>
                  <button onClick={() => route(`/app/restaurants/${r.id}/menu`)} class="text-xs text-slate-500 hover:text-amber-600 font-medium transition-colors">菜單</button>
                  <button onClick={() => route(`/app/restaurants/${r.id}/orders`)} class="text-xs text-slate-500 hover:text-amber-600 font-medium transition-colors">訂單</button>
                </div>
                <button
                  onClick={() => handleDelete(r.id)}
                  disabled={deletingId === r.id}
                  class="text-xs text-slate-400 hover:text-red-500 disabled:opacity-50 transition-colors"
                >
                  {deletingId === r.id ? '...' : '刪除'}
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Create modal */}
      <Modal open={showCreate} onClose={() => setShowCreate(false)} title="新增餐廳">
        <form onSubmit={handleCreate} class="space-y-4">
          <input type="text" placeholder="餐廳名稱" value={newName} onInput={(e) => setNewName((e.target as HTMLInputElement).value)} class={inputClass} required />
          <input type="text" placeholder="地址（選填）" value={newAddress} onInput={(e) => setNewAddress((e.target as HTMLInputElement).value)} class={inputClass} />
          <div class="flex gap-2 justify-end">
            <button type="button" onClick={() => setShowCreate(false)} class="border border-slate-200 px-4 py-2 rounded-lg text-sm text-slate-600 hover:bg-slate-50 transition-colors">取消</button>
            <button type="submit" disabled={creating} class="bg-amber-600 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-amber-700 disabled:opacity-50 transition-colors">{creating ? '建立中...' : '建立'}</button>
          </div>
        </form>
      </Modal>
    </div>
  );
}
```

**Step 2: Verify build**

Run: `cd frontend && npm run build`
Expected: Build succeeds.

**Step 3: Commit**

```bash
git add frontend/src/pages/Dashboard.tsx
git commit -m "feat: redesign dashboard with card grid, stats, and create modal"
```

---

### Task 6: Redesign Restaurant Settings Page

**Files:**
- Modify: `frontend/src/pages/RestaurantEdit.tsx`

**Step 1: Rewrite RestaurantEdit.tsx**

Replace entire `frontend/src/pages/RestaurantEdit.tsx` with:

```tsx
import { useState, useEffect } from 'preact/hooks';
import { route } from 'preact-router';
import { getRestaurant, updateRestaurant, publishRestaurant, getQRUrl } from '../lib/api';
import type { Restaurant } from '../lib/api';
import type { RoutableProps } from '../lib/route';
import Toggle from '../components/Toggle';
import { SkeletonList } from '../components/Skeleton';

export default function RestaurantEdit({ id = '' }: RoutableProps & { id?: string }) {
  const rid = parseInt(id);
  const [rest, setRest] = useState<Restaurant | null>(null);
  const [form, setForm] = useState({
    name: '', address: '', phone_number: '', website: '',
    dine_in: true, takeout: false, delivery: false, minimum_spend: 0,
  });
  const [saving, setSaving] = useState(false);
  const [publishing, setPublishing] = useState(false);

  useEffect(() => {
    getRestaurant(rid).then((r) => {
      setRest(r);
      setForm({
        name: r.name, address: r.address || '', phone_number: r.phone_number || '',
        website: r.website || '', dine_in: r.dine_in, takeout: r.takeout,
        delivery: r.delivery, minimum_spend: r.minimum_spend || 0,
      });
    });
  }, [rid]);

  const save = async (e: Event) => {
    e.preventDefault();
    if (saving) return;
    setSaving(true);
    try {
      const updated = await updateRestaurant(rid, form);
      setRest(updated);
    } catch {}
    setSaving(false);
  };

  const togglePublish = async () => {
    if (!rest || publishing) return;
    setPublishing(true);
    try {
      const updated = await publishRestaurant(rid, !rest.is_published);
      setRest(updated);
    } finally {
      setPublishing(false);
    }
  };

  const inputClass =
    'w-full bg-white border border-slate-200 rounded-lg px-4 py-2.5 text-sm text-slate-800 placeholder:text-slate-400 focus:outline-none focus:ring-2 focus:ring-amber-500/20 focus:border-amber-500 transition-colors';

  if (!rest) return <div class="max-w-3xl mx-auto"><SkeletonList rows={4} /></div>;

  return (
    <div class="max-w-3xl mx-auto space-y-6">
      {/* Page header */}
      <h1 class="text-2xl font-bold text-slate-800 tracking-tight">{rest.name}</h1>

      {/* Publish banner */}
      <div class={`rounded-xl p-4 flex items-center justify-between ${
        rest.is_published
          ? 'bg-emerald-50 border border-emerald-200'
          : 'bg-slate-50 border border-slate-200'
      }`}>
        <div>
          <p class="font-medium text-slate-800">{rest.is_published ? '餐廳已上線' : '餐廳尚未發布'}</p>
          <p class="text-sm text-slate-500">{rest.is_published ? '顧客可以透過 QR Code 點餐' : '發布後顧客即可開始點餐'}</p>
        </div>
        <button
          onClick={togglePublish}
          disabled={publishing}
          class={`px-4 py-2 rounded-lg text-sm font-medium transition-colors disabled:opacity-50 ${
            rest.is_published
              ? 'bg-red-100 text-red-700 hover:bg-red-200'
              : 'bg-emerald-600 text-white hover:bg-emerald-700'
          }`}
        >
          {publishing ? '處理中...' : rest.is_published ? '取消發布' : '發布餐廳'}
        </button>
      </div>

      {/* QR Code */}
      {rest.is_published && (
        <div class="bg-white rounded-xl shadow-sm border border-slate-100 p-6 flex flex-col sm:flex-row items-center gap-6">
          <img src={getQRUrl(rid)} alt="QR Code" class="rounded-lg border border-slate-200" width={160} height={160} />
          <div class="text-center sm:text-left">
            <h3 class="font-semibold text-slate-800">QR Code</h3>
            <p class="text-sm text-slate-500 mt-1 mb-3">掃描即可查看菜單並下單</p>
            <div class="flex gap-2">
              <a href={getQRUrl(rid)} download={`qr-${rest.slug}.png`} class="bg-amber-600 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-amber-700 transition-colors inline-block">下載</a>
              <a href={`/r/${rest.slug}`} target="_blank" class="border border-slate-200 px-4 py-2 rounded-lg text-sm text-slate-600 hover:bg-slate-50 transition-colors inline-block">公開頁面</a>
            </div>
          </div>
        </div>
      )}

      {/* Settings form */}
      <form onSubmit={save} class="space-y-6">
        {/* Basic info */}
        <div class="bg-white rounded-xl shadow-sm border border-slate-100 p-6 space-y-4">
          <h3 class="text-base font-semibold text-slate-800">基本資訊</h3>
          <div>
            <label class="block text-sm font-medium text-slate-600 mb-1.5">名稱</label>
            <input type="text" value={form.name} onInput={(e) => setForm({ ...form, name: (e.target as HTMLInputElement).value })} class={inputClass} required />
          </div>
          <div>
            <label class="block text-sm font-medium text-slate-600 mb-1.5">地址</label>
            <input type="text" value={form.address} onInput={(e) => setForm({ ...form, address: (e.target as HTMLInputElement).value })} class={inputClass} />
          </div>
        </div>

        {/* Contact */}
        <div class="bg-white rounded-xl shadow-sm border border-slate-100 p-6 space-y-4">
          <h3 class="text-base font-semibold text-slate-800">聯絡方式</h3>
          <div>
            <label class="block text-sm font-medium text-slate-600 mb-1.5">電話</label>
            <input type="text" value={form.phone_number} onInput={(e) => setForm({ ...form, phone_number: (e.target as HTMLInputElement).value })} class={inputClass} />
          </div>
          <div>
            <label class="block text-sm font-medium text-slate-600 mb-1.5">網站</label>
            <input type="text" value={form.website} onInput={(e) => setForm({ ...form, website: (e.target as HTMLInputElement).value })} class={inputClass} />
          </div>
        </div>

        {/* Service type */}
        <div class="bg-white rounded-xl shadow-sm border border-slate-100 p-6 space-y-4">
          <h3 class="text-base font-semibold text-slate-800">服務類型</h3>
          <div class="space-y-3">
            <Toggle label="內用" enabled={form.dine_in} onChange={(v) => setForm({ ...form, dine_in: v })} />
            <Toggle label="外帶" enabled={form.takeout} onChange={(v) => setForm({ ...form, takeout: v })} />
            <Toggle label="外送" enabled={form.delivery} onChange={(v) => setForm({ ...form, delivery: v })} />
          </div>
        </div>

        {/* Minimum spend */}
        <div class="bg-white rounded-xl shadow-sm border border-slate-100 p-6 space-y-4">
          <h3 class="text-base font-semibold text-slate-800">消費設定</h3>
          <div>
            <label class="block text-sm font-medium text-slate-600 mb-1.5">最低消費 (元)</label>
            <input type="number" value={form.minimum_spend} onInput={(e) => setForm({ ...form, minimum_spend: parseInt((e.target as HTMLInputElement).value) || 0 })} class={inputClass} min={0} />
          </div>
        </div>

        <button type="submit" disabled={saving} class="bg-amber-600 text-white px-6 py-2.5 rounded-lg font-medium hover:bg-amber-700 disabled:opacity-50 transition-colors">
          {saving ? '儲存中...' : '儲存設定'}
        </button>
      </form>
    </div>
  );
}
```

**Step 2: Verify build**

Run: `cd frontend && npm run build`
Expected: Build succeeds.

**Step 3: Commit**

```bash
git add frontend/src/pages/RestaurantEdit.tsx
git commit -m "feat: redesign settings with sections, toggles, publish banner, and QR card"
```

---

### Task 7: Redesign Menu Editor

**Files:**
- Modify: `frontend/src/pages/MenuEditor.tsx`

**Step 1: Rewrite MenuEditor.tsx**

Replace entire `frontend/src/pages/MenuEditor.tsx` with:

```tsx
import { useState, useEffect, useRef, useCallback } from 'preact/hooks';
import { getMenu, saveMenu, uploadPhotos, triggerOCR } from '../lib/api';
import type { MenuData, MenuCategory, MenuItem } from '../lib/api';
import type { RoutableProps } from '../lib/route';
import Sortable from 'sortablejs';
import { SkeletonList } from '../components/Skeleton';

export default function MenuEditor({ id = '' }: RoutableProps & { id?: string }) {
  const rid = parseInt(id);
  const [menu, setMenu] = useState<MenuData>({ categories: [], combos: [] });
  const [savedMenu, setSavedMenu] = useState<MenuData>({ categories: [], combos: [] });
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [ocrRunning, setOcrRunning] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [msg, setMsg] = useState('');
  const [collapsed, setCollapsed] = useState<Set<number>>(new Set());
  const [editingItem, setEditingItem] = useState<string | null>(null); // "catIdx-itemIdx"
  const categoriesRef = useRef<HTMLDivElement>(null);
  const sortableRefs = useRef<Map<number, Sortable>>(new Map());

  const isDirty = JSON.stringify(menu) !== JSON.stringify(savedMenu);

  useEffect(() => {
    getMenu(rid)
      .then((m) => { setMenu(m); setSavedMenu(m); })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, [rid]);

  // Category drag-and-drop
  useEffect(() => {
    if (!categoriesRef.current || loading) return;
    const s = Sortable.create(categoriesRef.current, {
      handle: '.cat-handle',
      animation: 150,
      onEnd: (evt) => {
        if (evt.oldIndex == null || evt.newIndex == null) return;
        setMenu(prev => {
          const cats = [...prev.categories];
          const [moved] = cats.splice(evt.oldIndex!, 1);
          cats.splice(evt.newIndex!, 0, moved);
          return { ...prev, categories: cats };
        });
      },
    });
    return () => s.destroy();
  }, [loading, menu.categories.length]);

  // Item drag-and-drop per category
  const initItemSortable = useCallback((el: HTMLElement | null, catIdx: number) => {
    if (!el) {
      sortableRefs.current.get(catIdx)?.destroy();
      sortableRefs.current.delete(catIdx);
      return;
    }
    if (sortableRefs.current.has(catIdx)) return;
    const s = Sortable.create(el, {
      handle: '.item-handle',
      animation: 150,
      onEnd: (evt) => {
        if (evt.oldIndex == null || evt.newIndex == null) return;
        setMenu(prev => {
          const cats = [...prev.categories];
          const items = [...cats[catIdx].items];
          const [moved] = items.splice(evt.oldIndex!, 1);
          items.splice(evt.newIndex!, 0, moved);
          cats[catIdx] = { ...cats[catIdx], items };
          return { ...prev, categories: cats };
        });
      },
    });
    sortableRefs.current.set(catIdx, s);
  }, []);

  const handleSave = async () => {
    setSaving(true);
    try {
      await saveMenu(rid, menu);
      setSavedMenu(menu);
      setMsg('已儲存');
      setTimeout(() => setMsg(''), 2000);
    } catch (err: any) {
      setMsg('儲存失敗: ' + err.message);
    }
    setSaving(false);
  };

  const handleDiscard = () => {
    setMenu(savedMenu);
    setEditingItem(null);
  };

  const handleUpload = async (e: Event) => {
    const input = e.target as HTMLInputElement;
    if (!input.files?.length) return;
    setUploading(true);
    try {
      await uploadPhotos(rid, input.files);
      setMsg('照片上傳成功');
      setTimeout(() => setMsg(''), 3000);
    } catch (err: any) {
      setMsg('上傳失敗: ' + err.message);
    }
    setUploading(false);
    input.value = '';
  };

  const handleOCR = async () => {
    setOcrRunning(true);
    setMsg('OCR 辨識中，請稍候...');
    try {
      const result = await triggerOCR(rid);
      setMenu(result);
      setSavedMenu(result);
      setMsg('OCR 完成！請檢查並修正菜單內容');
    } catch (err: any) {
      setMsg('OCR 失敗: ' + err.message);
    }
    setOcrRunning(false);
  };

  const addCategory = () => {
    setMenu(prev => ({
      ...prev,
      categories: [...prev.categories, { id: 0, name: '新分類', sort_order: prev.categories.length + 1, items: [] }],
    }));
  };

  const updateCategory = (idx: number, update: Partial<MenuCategory>) => {
    setMenu(prev => {
      const cats = [...prev.categories];
      cats[idx] = { ...cats[idx], ...update };
      return { ...prev, categories: cats };
    });
  };

  const removeCategory = (idx: number) => {
    setMenu(prev => ({ ...prev, categories: prev.categories.filter((_, i) => i !== idx) }));
  };

  const addItem = (catIdx: number) => {
    setMenu(prev => {
      const cats = [...prev.categories];
      cats[catIdx] = {
        ...cats[catIdx],
        items: [...cats[catIdx].items, { id: 0, name: '新品項', description: '', price: 0, is_available: true, category_id: 0 }],
      };
      return { ...prev, categories: cats };
    });
    // Auto-expand and edit new item
    setCollapsed(prev => { const n = new Set(prev); n.delete(catIdx); return n; });
    const newIdx = menu.categories[catIdx]?.items.length || 0;
    setEditingItem(`${catIdx}-${newIdx}`);
  };

  const updateItem = (catIdx: number, itemIdx: number, update: Partial<MenuItem>) => {
    setMenu(prev => {
      const cats = [...prev.categories];
      const items = [...cats[catIdx].items];
      items[itemIdx] = { ...items[itemIdx], ...update };
      cats[catIdx] = { ...cats[catIdx], items };
      return { ...prev, categories: cats };
    });
  };

  const removeItem = (catIdx: number, itemIdx: number) => {
    setMenu(prev => {
      const cats = [...prev.categories];
      cats[catIdx] = { ...cats[catIdx], items: cats[catIdx].items.filter((_, i) => i !== itemIdx) };
      return { ...prev, categories: cats };
    });
  };

  const toggleCollapsed = (idx: number) => {
    setCollapsed(prev => {
      const n = new Set(prev);
      if (n.has(idx)) n.delete(idx);
      else n.add(idx);
      return n;
    });
  };

  const inputClass =
    'w-full bg-white border border-slate-200 rounded-lg px-3 py-2 text-sm text-slate-800 placeholder:text-slate-400 focus:outline-none focus:ring-2 focus:ring-amber-500/20 focus:border-amber-500 transition-colors';

  if (loading) return <div class="max-w-3xl mx-auto"><SkeletonList rows={4} /></div>;

  return (
    <div class="max-w-3xl mx-auto pb-20">
      <h1 class="text-2xl font-bold text-slate-800 tracking-tight mb-6">菜單編輯</h1>

      {/* Photo upload + OCR */}
      <div class="bg-white rounded-xl shadow-sm border border-slate-100 p-6 mb-6">
        <h2 class="font-semibold text-slate-800 mb-3">照片辨識菜單</h2>
        <label class={`block border-2 border-dashed rounded-xl p-8 text-center transition-colors ${uploading ? 'border-amber-300 bg-amber-50/50' : 'border-slate-200 hover:border-amber-400 hover:bg-amber-50/30 cursor-pointer'}`}>
          <div class="text-3xl mb-2">📷</div>
          <p class="font-medium text-slate-700 text-sm">{uploading ? '上傳中...' : '點擊選擇菜單照片'}</p>
          <p class="text-xs text-slate-400 mt-1">支援 JPG、PNG，可多選</p>
          <input type="file" accept="image/*" multiple onChange={handleUpload} disabled={uploading} class="hidden" />
        </label>
        <button
          onClick={handleOCR}
          disabled={ocrRunning}
          class="mt-3 w-full bg-slate-800 text-white px-4 py-2.5 rounded-lg text-sm font-medium hover:bg-slate-700 disabled:opacity-50 transition-colors"
        >
          {ocrRunning ? '辨識中...' : '開始 OCR 辨識'}
        </button>
      </div>

      {msg && (
        <div class="mb-4 text-sm bg-amber-50 border border-amber-200 text-amber-800 rounded-lg px-4 py-2.5">
          {msg}
        </div>
      )}

      {/* Categories */}
      <div ref={categoriesRef} class="space-y-4">
        {menu.categories.map((cat, ci) => (
          <div key={cat.id || ci} class="bg-white rounded-xl shadow-sm border border-slate-100 overflow-hidden">
            {/* Category header */}
            <div
              class="flex items-center gap-3 px-4 py-3 bg-slate-50 cursor-pointer select-none"
              onClick={() => toggleCollapsed(ci)}
            >
              <span class="cat-handle text-slate-300 cursor-grab text-lg" onClick={(e) => e.stopPropagation()}>⠿</span>
              <input
                type="text"
                value={cat.name}
                onInput={(e) => { e.stopPropagation(); updateCategory(ci, { name: (e.target as HTMLInputElement).value }); }}
                onClick={(e) => e.stopPropagation()}
                class="font-semibold text-slate-800 bg-transparent border-none focus:outline-none focus:ring-0 flex-1 min-w-0"
              />
              <span class="text-xs text-slate-400 font-medium">{cat.items.length} 項</span>
              <button onClick={(e) => { e.stopPropagation(); removeCategory(ci); }} class="text-slate-400 hover:text-red-500 text-sm transition-colors">刪除</button>
              <span class={`text-slate-400 transition-transform ${collapsed.has(ci) ? '-rotate-90' : ''}`}>▾</span>
            </div>

            {/* Items */}
            {!collapsed.has(ci) && (
              <div ref={(el) => initItemSortable(el, ci)}>
                {cat.items.map((item, ii) => {
                  const isEditing = editingItem === `${ci}-${ii}`;
                  return (
                    <div key={item.id || ii} class="border-t border-slate-50">
                      {isEditing ? (
                        /* Edit mode */
                        <div class="px-4 py-3 space-y-2 bg-amber-50/30">
                          <input type="text" value={item.name} onInput={(e) => updateItem(ci, ii, { name: (e.target as HTMLInputElement).value })} class={inputClass} placeholder="品名" />
                          <input type="text" value={item.description || ''} onInput={(e) => updateItem(ci, ii, { description: (e.target as HTMLInputElement).value })} class={inputClass} placeholder="描述（選填）" />
                          <div class="flex gap-2 items-center">
                            <input type="number" value={item.price} onInput={(e) => updateItem(ci, ii, { price: parseInt((e.target as HTMLInputElement).value) || 0 })} class={`${inputClass} w-28`} min={0} />
                            <span class="text-sm text-slate-400">元</span>
                            <div class="flex-1" />
                            <button onClick={() => setEditingItem(null)} class="text-xs text-amber-600 font-medium hover:text-amber-700">完成</button>
                            <button onClick={() => removeItem(ci, ii)} class="text-xs text-red-500 hover:text-red-600">刪除</button>
                          </div>
                        </div>
                      ) : (
                        /* Read mode */
                        <div
                          class="flex items-center gap-3 px-4 py-2.5 hover:bg-slate-50/50 group cursor-pointer"
                          onClick={() => setEditingItem(`${ci}-${ii}`)}
                        >
                          <span class="item-handle text-slate-200 cursor-grab opacity-0 group-hover:opacity-100 transition-opacity" onClick={(e) => e.stopPropagation()}>⠿</span>
                          <div class="flex-1 min-w-0">
                            <p class="text-sm font-medium text-slate-800 truncate">{item.name}</p>
                            {item.description && <p class="text-xs text-slate-400 truncate">{item.description}</p>}
                          </div>
                          <span class="text-sm font-medium text-slate-700 tabular-nums">${item.price}</span>
                          <button
                            onClick={(e) => { e.stopPropagation(); removeItem(ci, ii); }}
                            class="text-slate-300 hover:text-red-500 opacity-0 group-hover:opacity-100 transition-all text-sm"
                          >
                            ✕
                          </button>
                        </div>
                      )}
                    </div>
                  );
                })}
                <button onClick={() => addItem(ci)} class="w-full text-left px-4 py-2.5 text-sm text-amber-600 hover:bg-amber-50/50 transition-colors font-medium">
                  + 新增品項
                </button>
              </div>
            )}
          </div>
        ))}
      </div>

      <button
        onClick={addCategory}
        class="mt-4 border-2 border-dashed border-slate-200 rounded-xl px-4 py-3 w-full text-sm text-slate-500 hover:border-amber-400 hover:text-amber-600 transition-colors font-medium"
      >
        + 新增分類
      </button>

      {/* Floating save bar */}
      {isDirty && (
        <div class="fixed bottom-0 left-0 lg:left-60 right-0 bg-white border-t border-slate-200 px-6 py-3 flex items-center justify-between shadow-lg z-40">
          <p class="text-sm text-slate-500">有未儲存的變更</p>
          <div class="flex gap-2">
            <button onClick={handleDiscard} class="border border-slate-200 px-4 py-2 rounded-lg text-sm text-slate-600 hover:bg-slate-50 transition-colors">捨棄</button>
            <button onClick={handleSave} disabled={saving} class="bg-amber-600 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-amber-700 disabled:opacity-50 transition-colors">{saving ? '儲存中...' : '儲存菜單'}</button>
          </div>
        </div>
      )}
    </div>
  );
}
```

**Step 2: Verify build**

Run: `cd frontend && npm run build`
Expected: Build succeeds.

**Step 3: Commit**

```bash
git add frontend/src/pages/MenuEditor.tsx
git commit -m "feat: redesign menu editor with accordion, inline edit, drag-and-drop, floating save"
```

---

### Task 8: Redesign Orders Page with Kanban Board

**Files:**
- Modify: `frontend/src/pages/Orders.tsx`

**Step 1: Rewrite Orders.tsx**

Replace entire `frontend/src/pages/Orders.tsx` with:

```tsx
import { useState, useEffect, useRef } from 'preact/hooks';
import { listOrders, updateOrderStatus } from '../lib/api';
import type { Order } from '../lib/api';
import type { RoutableProps } from '../lib/route';
import { SkeletonList } from '../components/Skeleton';

const STATUS_LABELS: Record<string, string> = {
  pending: '待確認', confirmed: '已確認', preparing: '準備中', completed: '已完成', cancelled: '已取消',
};

const STATUS_COLORS: Record<string, string> = {
  pending: 'border-l-yellow-400', confirmed: 'border-l-blue-400',
  preparing: 'border-l-orange-400', completed: 'border-l-emerald-400',
};

const COLUMN_BADGE: Record<string, string> = {
  pending: 'bg-yellow-100 text-yellow-800', confirmed: 'bg-blue-100 text-blue-800',
  preparing: 'bg-orange-100 text-orange-800', completed: 'bg-emerald-100 text-emerald-800',
};

const ACTION_COLORS: Record<string, string> = {
  pending: 'bg-amber-600 hover:bg-amber-700', confirmed: 'bg-blue-600 hover:bg-blue-700',
  preparing: 'bg-emerald-600 hover:bg-emerald-700',
};

const NEXT_STATUS: Record<string, string> = {
  pending: 'confirmed', confirmed: 'preparing', preparing: 'completed',
};

const KANBAN_COLUMNS = ['pending', 'confirmed', 'preparing', 'completed'] as const;

function relativeTime(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return '剛剛';
  if (mins < 60) return `${mins} 分鐘前`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours} 小時前`;
  return `${Math.floor(hours / 24)} 天前`;
}

export default function Orders({ id = '' }: RoutableProps & { id?: string }) {
  const rid = parseInt(id);
  const [orders, setOrders] = useState<Order[]>([]);
  const [filter, setFilter] = useState('');
  const [loading, setLoading] = useState(true);
  const [updatingId, setUpdatingId] = useState<number | null>(null);
  const timerRef = useRef<ReturnType<typeof setInterval>>();

  const load = () => {
    listOrders(rid)
      .then(setOrders)
      .catch(() => {})
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    load();
    timerRef.current = setInterval(load, 10000);
    return () => clearInterval(timerRef.current);
  }, [rid]);

  const advance = async (order: Order) => {
    const next = NEXT_STATUS[order.status];
    if (!next || updatingId !== null) return;
    setUpdatingId(order.id);
    try {
      await updateOrderStatus(rid, order.id, next);
      load();
    } finally {
      setUpdatingId(null);
    }
  };

  const cancel = async (order: Order) => {
    if (updatingId !== null) return;
    if (!confirm('確定取消此訂單？')) return;
    setUpdatingId(order.id);
    try {
      await updateOrderStatus(rid, order.id, 'cancelled');
      load();
    } finally {
      setUpdatingId(null);
    }
  };

  const ordersByStatus = (status: string) => orders.filter((o) => o.status === status);

  // Mobile: filtered list
  const filteredOrders = filter ? orders.filter((o) => o.status === filter) : orders;

  const renderOrderCard = (o: Order, compact = false) => (
    <div key={o.id} class={`bg-white rounded-lg shadow-sm border-l-4 ${STATUS_COLORS[o.status] || 'border-l-slate-200'} border border-slate-100 p-3`}>
      <div class="flex items-center justify-between mb-2">
        <span class="font-mono text-xs text-slate-400">#{o.id}</span>
        <span class="text-xs text-slate-400">{relativeTime(o.created_at)}</span>
      </div>
      {o.table_label && <p class="text-sm font-medium text-slate-800 mb-1">桌號 {o.table_label}</p>}
      <p class={`font-bold text-slate-800 ${compact ? 'text-base' : 'text-lg'}`}>${o.total_amount} 元</p>
      {!compact && o.status !== 'completed' && o.status !== 'cancelled' && (
        <div class="flex gap-2 mt-3">
          {NEXT_STATUS[o.status] && (
            <button
              onClick={() => advance(o)}
              disabled={updatingId === o.id}
              class={`flex-1 text-white text-xs py-1.5 rounded-lg font-medium disabled:opacity-50 transition-colors ${ACTION_COLORS[o.status]}`}
            >
              {updatingId === o.id ? '...' : STATUS_LABELS[NEXT_STATUS[o.status]]}
            </button>
          )}
          <button
            onClick={() => cancel(o)}
            disabled={updatingId === o.id}
            class="text-xs text-red-500 hover:text-red-600 px-2 disabled:opacity-50"
          >
            取消
          </button>
        </div>
      )}
    </div>
  );

  if (loading) return <div class="max-w-5xl mx-auto"><SkeletonList rows={4} /></div>;

  return (
    <div class="max-w-5xl mx-auto">
      <div class="flex items-center gap-3 mb-6">
        <h1 class="text-2xl font-bold text-slate-800 tracking-tight">訂單管理</h1>
        {/* Pulsing live indicator */}
        <span class="relative flex h-2.5 w-2.5">
          <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-amber-400 opacity-75" />
          <span class="relative inline-flex rounded-full h-2.5 w-2.5 bg-amber-500" />
        </span>
        <span class="text-xs text-slate-400">即時更新</span>
      </div>

      {/* Desktop: Kanban board */}
      <div class="hidden lg:grid lg:grid-cols-4 gap-4" style="height: calc(100vh - 10rem)">
        {KANBAN_COLUMNS.map((status) => {
          const col = ordersByStatus(status);
          return (
            <div key={status} class="flex flex-col bg-slate-50 rounded-xl overflow-hidden">
              <div class="px-4 py-3 border-b border-slate-200 flex items-center justify-between">
                <h3 class="font-semibold text-sm text-slate-700">{STATUS_LABELS[status]}</h3>
                <span class={`text-xs font-bold px-2 py-0.5 rounded-full ${COLUMN_BADGE[status]}`}>{col.length}</span>
              </div>
              <div class="flex-1 overflow-y-auto p-3 space-y-2">
                {col.length === 0 && <p class="text-xs text-slate-400 text-center py-4">沒有訂單</p>}
                {col.map((o) => renderOrderCard(o))}
              </div>
            </div>
          );
        })}
      </div>

      {/* Mobile: filter tabs + list */}
      <div class="lg:hidden">
        <div class="flex gap-2 mb-4 flex-wrap">
          {['', ...KANBAN_COLUMNS, 'cancelled'].map((s) => (
            <button
              key={s}
              onClick={() => setFilter(s)}
              class={`px-3 py-1.5 rounded-lg text-sm font-medium transition-colors ${
                filter === s ? 'bg-amber-600 text-white' : 'bg-white border border-slate-200 text-slate-600 hover:bg-slate-50'
              }`}
            >
              {s === '' ? '全部' : STATUS_LABELS[s]}
            </button>
          ))}
        </div>
        {filteredOrders.length === 0 ? (
          <p class="text-slate-400 text-center py-8">沒有訂單</p>
        ) : (
          <div class="space-y-2">
            {filteredOrders.map((o) => renderOrderCard(o))}
          </div>
        )}
      </div>
    </div>
  );
}
```

**Step 2: Verify build**

Run: `cd frontend && npm run build`
Expected: Build succeeds.

**Step 3: Commit**

```bash
git add frontend/src/pages/Orders.tsx
git commit -m "feat: redesign orders with kanban board, relative timestamps, live indicator"
```

---

### Task 9: Build Frontend and Update Embedded Dist

**Files:**
- Modify: `frontend/dist/*` (rebuilt output)

**Step 1: Build production frontend**

Run:
```bash
cd frontend && npm run build
```

**Step 2: Verify Go build still works**

Run:
```bash
cd /home/pgi/query && go build ./cmd/server
```
Expected: Build succeeds — embedded `frontend/dist/` is up to date.

**Step 3: Commit built output**

```bash
git add frontend/dist/
git commit -m "chore: rebuild frontend dist with UI redesign"
```

---

### Task 10: Manual Smoke Test

**Step 1: Restart backend + frontend dev servers**

```bash
tmux send-keys -t query:backend C-c && sleep 1 && tmux send-keys -t query:backend './server --db "postgres://query:query@localhost:5432/query?sslmode=disable"' Enter
tmux send-keys -t query:frontend C-c && sleep 1 && tmux send-keys -t query:frontend 'npm run dev' Enter
```

**Step 2: Visual checks**

Open `http://localhost:5173/app/` and verify:
- [ ] Login: split layout with amber gradient on left, form card on right
- [ ] Dashboard: card grid, amber accent strip, stats bar, modal for create
- [ ] Settings: sectioned cards, toggle switches, publish banner, QR card
- [ ] Menu Editor: accordion categories, inline editing, drag-and-drop handles, floating save bar
- [ ] Orders: kanban board on desktop, filter tabs on mobile, pulsing live indicator
- [ ] Sidebar: dark slate sidebar, active nav with amber accent, mobile drawer
- [ ] All loading states show skeleton screens instead of text
