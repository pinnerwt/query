import { useState, useEffect } from 'preact/hooks';
import { route } from 'preact-router';
import { listMyRestaurants, createRestaurant, deleteRestaurant } from '../lib/api';
import type { Restaurant } from '../lib/api';
import { useAuth } from '../lib/auth';
import type { RoutableProps } from '../lib/route';

export default function Dashboard(_props: RoutableProps) {
  const { owner, logout } = useAuth();
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

  return (
    <div class="max-w-2xl mx-auto p-4">
      <div class="flex justify-between items-center mb-6">
        <h1 class="text-2xl font-bold">我的餐廳</h1>
        <div class="flex items-center gap-3">
          <span class="text-sm text-gray-600">{owner?.name}</span>
          <button onClick={logout} class="text-sm text-red-600 underline">
            登出
          </button>
        </div>
      </div>

      {loading ? (
        <p class="text-gray-500">載入中...</p>
      ) : (
        <>
          <div class="space-y-3 mb-6">
            {restaurants.map((r) => (
              <div key={r.id} class="border rounded p-4 flex justify-between items-center">
                <div>
                  <h2
                    class="font-semibold text-lg cursor-pointer hover:text-blue-600"
                    onClick={() => route(`/app/restaurants/${r.id}`)}
                  >
                    {r.name}
                  </h2>
                  <p class="text-sm text-gray-500">{r.address || '未設定地址'}</p>
                  <span
                    class={`text-xs px-2 py-0.5 rounded ${
                      r.is_published
                        ? 'bg-green-100 text-green-700'
                        : 'bg-gray-100 text-gray-500'
                    }`}
                  >
                    {r.is_published ? '已發布' : '未發布'}
                  </span>
                </div>
                <button
                  onClick={() => handleDelete(r.id)}
                  disabled={deletingId === r.id}
                  class="text-red-500 text-sm hover:underline disabled:opacity-50"
                >
                  {deletingId === r.id ? '刪除中...' : '刪除'}
                </button>
              </div>
            ))}
            {restaurants.length === 0 && (
              <p class="text-gray-400">還沒有餐廳，點擊下方按鈕建立</p>
            )}
          </div>

          {showCreate ? (
            <form onSubmit={handleCreate} class="border rounded p-4 space-y-3">
              <input
                type="text"
                placeholder="餐廳名稱"
                value={newName}
                onInput={(e) => setNewName((e.target as HTMLInputElement).value)}
                class="w-full border rounded px-3 py-2"
                required
              />
              <input
                type="text"
                placeholder="地址（選填）"
                value={newAddress}
                onInput={(e) => setNewAddress((e.target as HTMLInputElement).value)}
                class="w-full border rounded px-3 py-2"
              />
              <div class="flex gap-2">
                <button
                  type="submit"
                  disabled={creating}
                  class="bg-blue-600 text-white px-4 py-2 rounded hover:bg-blue-700 disabled:opacity-50"
                >
                  {creating ? '建立中...' : '建立'}
                </button>
                <button
                  type="button"
                  onClick={() => setShowCreate(false)}
                  class="border px-4 py-2 rounded"
                >
                  取消
                </button>
              </div>
            </form>
          ) : (
            <button
              onClick={() => setShowCreate(true)}
              class="bg-blue-600 text-white px-4 py-2 rounded hover:bg-blue-700"
            >
              + 新增餐廳
            </button>
          )}
        </>
      )}
    </div>
  );
}
