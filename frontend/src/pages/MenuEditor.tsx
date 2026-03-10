import { useState, useEffect } from 'preact/hooks';
import { route } from 'preact-router';
import { getMenu, saveMenu, uploadPhotos, triggerOCR } from '../lib/api';
import type { MenuData, MenuCategory, MenuItem } from '../lib/api';
import type { RoutableProps } from '../lib/route';

export default function MenuEditor({ id = '' }: RoutableProps & { id?: string }) {
  const rid = parseInt(id);
  const [menu, setMenu] = useState<MenuData>({ categories: [], combos: [] });
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [ocrRunning, setOcrRunning] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [msg, setMsg] = useState('');

  useEffect(() => {
    getMenu(rid)
      .then(setMenu)
      .catch(() => {})
      .finally(() => setLoading(false));
  }, [rid]);

  const handleSave = async () => {
    setSaving(true);
    try {
      await saveMenu(rid, menu);
      setMsg('已儲存');
      setTimeout(() => setMsg(''), 2000);
    } catch (err: any) {
      setMsg('儲存失敗: ' + err.message);
    }
    setSaving(false);
  };

  const handleUpload = async (e: Event) => {
    const input = e.target as HTMLInputElement;
    if (!input.files?.length) return;
    setUploading(true);
    try {
      await uploadPhotos(rid, input.files);
      setMsg('照片上傳成功');
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
      setMsg('OCR 完成！請檢查並修正菜單內容');
    } catch (err: any) {
      setMsg('OCR 失敗: ' + err.message);
    }
    setOcrRunning(false);
  };

  const addCategory = () => {
    setMenu(prev => ({
      ...prev,
      categories: [
        ...prev.categories,
        { id: 0, name: '新分類', sort_order: prev.categories.length + 1, items: [] },
      ],
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
        items: [
          ...cats[catIdx].items,
          { id: 0, name: '新品項', description: '', price: 0, is_available: true, category_id: 0 },
        ],
      };
      return { ...prev, categories: cats };
    });
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
      cats[catIdx] = {
        ...cats[catIdx],
        items: cats[catIdx].items.filter((_, i) => i !== itemIdx),
      };
      return { ...prev, categories: cats };
    });
  };

  if (loading) return <p class="p-4 text-gray-500">載入中...</p>;

  return (
    <div class="max-w-3xl mx-auto p-4">
      <button
        onClick={() => route(`/app/restaurants/${rid}`)}
        class="text-blue-600 text-sm mb-4 inline-block"
      >
        &larr; 返回餐廳設定
      </button>

      <h1 class="text-2xl font-bold mb-4">菜單編輯</h1>

      {/* Photo upload + OCR */}
      <div class="border rounded p-4 mb-6 bg-gray-50 space-y-3">
        <h2 class="font-semibold">照片辨識菜單</h2>
        <div class="flex flex-wrap gap-2 items-center">
          <label class={`bg-white border px-3 py-1.5 rounded text-sm ${uploading ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer hover:bg-gray-100'}`}>
            {uploading ? '上傳中...' : '上傳照片'}
            <input
              type="file"
              accept="image/*"
              multiple
              onChange={handleUpload}
              disabled={uploading}
              class="hidden"
            />
          </label>
          <button
            onClick={handleOCR}
            disabled={ocrRunning}
            class="bg-purple-600 text-white px-3 py-1.5 rounded text-sm hover:bg-purple-700 disabled:opacity-50"
          >
            {ocrRunning ? '辨識中...' : '開始 OCR 辨識'}
          </button>
        </div>
        <p class="text-xs text-gray-500">
          上傳菜單照片後，點擊 OCR 辨識自動擷取菜單內容
        </p>
      </div>

      {msg && <p class="text-sm mb-4 text-blue-700 bg-blue-50 rounded px-3 py-2">{msg}</p>}

      {/* Categories + Items */}
      <div class="space-y-6">
        {menu.categories.map((cat, ci) => (
          <div key={ci} class="border rounded p-4">
            <div class="flex gap-2 items-center mb-3">
              <input
                type="text"
                value={cat.name}
                onInput={(e) =>
                  updateCategory(ci, { name: (e.target as HTMLInputElement).value })
                }
                class="font-semibold text-lg border-b border-gray-300 flex-1 focus:outline-none focus:border-blue-500"
              />
              <button
                onClick={() => removeCategory(ci)}
                class="text-red-500 text-sm hover:underline"
              >
                刪除分類
              </button>
            </div>

            <div class="space-y-2">
              {cat.items.map((item, ii) => (
                <div key={ii} class="flex gap-2 items-start border-b pb-2">
                  <div class="flex-1 space-y-1">
                    <input
                      type="text"
                      value={item.name}
                      onInput={(e) =>
                        updateItem(ci, ii, { name: (e.target as HTMLInputElement).value })
                      }
                      class="w-full border rounded px-2 py-1 text-sm"
                      placeholder="品名"
                    />
                    <input
                      type="text"
                      value={item.description || ''}
                      onInput={(e) =>
                        updateItem(ci, ii, {
                          description: (e.target as HTMLInputElement).value,
                        })
                      }
                      class="w-full border rounded px-2 py-1 text-sm text-gray-500"
                      placeholder="描述（選填）"
                    />
                  </div>
                  <input
                    type="number"
                    value={item.price}
                    onInput={(e) =>
                      updateItem(ci, ii, {
                        price: parseInt((e.target as HTMLInputElement).value) || 0,
                      })
                    }
                    class="w-20 border rounded px-2 py-1 text-sm text-right"
                    min={0}
                  />
                  <span class="text-sm text-gray-400 pt-1">元</span>
                  <button
                    onClick={() => removeItem(ci, ii)}
                    class="text-red-400 text-xs hover:underline pt-1"
                  >
                    刪除
                  </button>
                </div>
              ))}
            </div>

            <button
              onClick={() => addItem(ci)}
              class="text-blue-600 text-sm mt-2 hover:underline"
            >
              + 新增品項
            </button>
          </div>
        ))}
      </div>

      <div class="flex gap-3 mt-6">
        <button
          onClick={addCategory}
          class="border px-4 py-2 rounded hover:bg-gray-100 text-sm"
        >
          + 新增分類
        </button>
        <button
          onClick={handleSave}
          disabled={saving}
          class="bg-blue-600 text-white px-4 py-2 rounded hover:bg-blue-700 disabled:opacity-50"
        >
          {saving ? '儲存中...' : '儲存菜單'}
        </button>
      </div>
    </div>
  );
}
