import { useState, useEffect } from 'preact/hooks';
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
