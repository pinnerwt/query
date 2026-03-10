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
