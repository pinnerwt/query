import { useState, useEffect } from 'preact/hooks';
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
  const [url, setUrl] = useState(getCurrentUrl());

  useEffect(() => {
    const update = () => setUrl(getCurrentUrl());
    addEventListener('popstate', update);
    // Intercept pushState/replaceState so client-side route() triggers re-render
    const origPush = history.pushState.bind(history);
    const origReplace = history.replaceState.bind(history);
    history.pushState = (...args) => { origPush(...args); update(); };
    history.replaceState = (...args) => { origReplace(...args); update(); };
    return () => {
      removeEventListener('popstate', update);
      history.pushState = origPush;
      history.replaceState = origReplace;
    };
  }, []);

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
