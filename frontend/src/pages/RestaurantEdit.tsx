import { useState, useEffect } from 'preact/hooks';
import { route } from 'preact-router';
import {
  getRestaurant,
  updateRestaurant,
  publishRestaurant,
  getQRUrl,
} from '../lib/api';
import type { Restaurant } from '../lib/api';
import type { RoutableProps } from '../lib/route';

export default function RestaurantEdit({ id = '' }: RoutableProps & { id?: string }) {
  const rid = parseInt(id);
  const [rest, setRest] = useState<Restaurant | null>(null);
  const [form, setForm] = useState({
    name: '',
    address: '',
    phone_number: '',
    website: '',
    dine_in: true,
    takeout: false,
    delivery: false,
    minimum_spend: 0,
  });
  const [saving, setSaving] = useState(false);
  const [publishing, setPublishing] = useState(false);

  useEffect(() => {
    getRestaurant(rid).then((r) => {
      setRest(r);
      setForm({
        name: r.name,
        address: r.address || '',
        phone_number: r.phone_number || '',
        website: r.website || '',
        dine_in: r.dine_in,
        takeout: r.takeout,
        delivery: r.delivery,
        minimum_spend: r.minimum_spend || 0,
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

  if (!rest) return <p class="p-4 text-gray-500">載入中...</p>;

  return (
    <div class="max-w-2xl mx-auto p-4">
      <button onClick={() => route('/app/')} class="text-blue-600 text-sm mb-4 inline-block">
        &larr; 返回
      </button>

      <div class="flex justify-between items-center mb-4">
        <h1 class="text-2xl font-bold">{rest.name}</h1>
        <button
          onClick={togglePublish}
          disabled={publishing}
          class={`px-3 py-1 rounded text-sm disabled:opacity-50 ${
            rest.is_published
              ? 'bg-red-100 text-red-700 hover:bg-red-200'
              : 'bg-green-100 text-green-700 hover:bg-green-200'
          }`}
        >
          {publishing ? '處理中...' : rest.is_published ? '取消發布' : '發布'}
        </button>
      </div>

      {/* Navigation tabs */}
      <div class="flex gap-2 mb-6 border-b pb-2">
        <button
          onClick={() => route(`/app/restaurants/${rid}/menu`)}
          class="px-3 py-1 rounded bg-gray-100 hover:bg-gray-200 text-sm"
        >
          菜單
        </button>
        <button
          onClick={() => route(`/app/restaurants/${rid}/orders`)}
          class="px-3 py-1 rounded bg-gray-100 hover:bg-gray-200 text-sm"
        >
          訂單
        </button>
        {rest.is_published && (
          <a
            href={`/r/${rest.slug}`}
            target="_blank"
            class="px-3 py-1 rounded bg-gray-100 hover:bg-gray-200 text-sm"
          >
            公開頁面
          </a>
        )}
      </div>

      {/* QR Code */}
      {rest.is_published && (
        <div class="mb-6 text-center">
          <h2 class="font-semibold mb-2">QR Code</h2>
          <img
            src={getQRUrl(rid)}
            alt="QR Code"
            class="mx-auto border rounded"
            width={200}
            height={200}
          />
          <a
            href={getQRUrl(rid)}
            download={`qr-${rest.slug}.png`}
            class="text-blue-600 text-sm underline mt-2 inline-block"
          >
            下載 QR Code
          </a>
        </div>
      )}

      <form onSubmit={save} class="space-y-4">
        <div>
          <label class="block text-sm font-medium mb-1">名稱</label>
          <input
            type="text"
            value={form.name}
            onInput={(e) => setForm({ ...form, name: (e.target as HTMLInputElement).value })}
            class="w-full border rounded px-3 py-2"
            required
          />
        </div>
        <div>
          <label class="block text-sm font-medium mb-1">地址</label>
          <input
            type="text"
            value={form.address}
            onInput={(e) => setForm({ ...form, address: (e.target as HTMLInputElement).value })}
            class="w-full border rounded px-3 py-2"
          />
        </div>
        <div>
          <label class="block text-sm font-medium mb-1">電話</label>
          <input
            type="text"
            value={form.phone_number}
            onInput={(e) =>
              setForm({ ...form, phone_number: (e.target as HTMLInputElement).value })
            }
            class="w-full border rounded px-3 py-2"
          />
        </div>
        <div>
          <label class="block text-sm font-medium mb-1">網站</label>
          <input
            type="text"
            value={form.website}
            onInput={(e) => setForm({ ...form, website: (e.target as HTMLInputElement).value })}
            class="w-full border rounded px-3 py-2"
          />
        </div>
        <div class="flex gap-4">
          <label class="flex items-center gap-1">
            <input
              type="checkbox"
              checked={form.dine_in}
              onChange={(e) =>
                setForm({ ...form, dine_in: (e.target as HTMLInputElement).checked })
              }
            />
            內用
          </label>
          <label class="flex items-center gap-1">
            <input
              type="checkbox"
              checked={form.takeout}
              onChange={(e) =>
                setForm({ ...form, takeout: (e.target as HTMLInputElement).checked })
              }
            />
            外帶
          </label>
          <label class="flex items-center gap-1">
            <input
              type="checkbox"
              checked={form.delivery}
              onChange={(e) =>
                setForm({ ...form, delivery: (e.target as HTMLInputElement).checked })
              }
            />
            外送
          </label>
        </div>
        <div>
          <label class="block text-sm font-medium mb-1">最低消費 (元)</label>
          <input
            type="number"
            value={form.minimum_spend}
            onInput={(e) =>
              setForm({ ...form, minimum_spend: parseInt((e.target as HTMLInputElement).value) || 0 })
            }
            class="w-full border rounded px-3 py-2"
            min={0}
          />
        </div>
        <button
          type="submit"
          disabled={saving}
          class="bg-blue-600 text-white px-4 py-2 rounded hover:bg-blue-700 disabled:opacity-50"
        >
          {saving ? '儲存中...' : '儲存'}
        </button>
      </form>
    </div>
  );
}
