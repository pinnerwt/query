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
      .then((m) => {
        const safe = { categories: m?.categories || [], combos: m?.combos || [] };
        setMenu(safe);
        setSavedMenu(safe);
      })
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
      const safe = { categories: result?.categories || [], combos: result?.combos || [] };
      setMenu(safe);
      setSavedMenu(safe);
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
