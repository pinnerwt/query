import { useState, useEffect, useRef, useCallback } from 'preact/hooks';
import { getMenu, saveMenu, uploadPhotos, triggerOCR, listMenuPhotos, deleteMenuPhoto } from '../lib/api';
import type { MenuData, MenuCategory, MenuItem, MenuPhoto } from '../lib/api';
import type { RoutableProps } from '../lib/route';
import Sortable from 'sortablejs';
import { SkeletonList } from '../components/Skeleton';

export default function MenuEditor({ id = '' }: RoutableProps & { id?: string }) {
  const rid = parseInt(id);
  const [menu, setMenu] = useState<MenuData>({ categories: [] });
  const [savedMenu, setSavedMenu] = useState<MenuData>({ categories: [] });
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [ocrRunning, setOcrRunning] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [uploadPct, setUploadPct] = useState(0);
  const [dragOver, setDragOver] = useState(false);
  const [photos, setPhotos] = useState<MenuPhoto[]>([]);
  const [deletingPhotoId, setDeletingPhotoId] = useState<number | null>(null);
  const [msg, setMsg] = useState('');
  const [collapsed, setCollapsed] = useState<Set<number>>(new Set());
  const [editingItem, setEditingItem] = useState<string | null>(null); // "catIdx-itemIdx"
  const batchOGDefault = { cat: null as number | null, name: '', options: '', min: 1, max: 1 };
  const [batchOG, setBatchOG] = useState(batchOGDefault);
  const categoriesRef = useRef<HTMLDivElement>(null);
  const sortableRefs = useRef<Map<number, Sortable>>(new Map());

  const isDirty = JSON.stringify(menu) !== JSON.stringify(savedMenu);

  useEffect(() => {
    getMenu(rid)
      .then((m) => {
        const safe = { categories: m?.categories || [] };
        setMenu(safe);
        setSavedMenu(safe);
      })
      .catch(() => {})
      .finally(() => setLoading(false));
    listMenuPhotos(rid).then(setPhotos).catch(() => {});
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

  const doUpload = async (files: FileList | File[]) => {
    if (!files.length || uploading) return;
    setUploading(true);
    setUploadPct(0);
    try {
      await uploadPhotos(rid, files, setUploadPct);
      listMenuPhotos(rid).then(setPhotos).catch(() => {});
      setMsg('照片上傳成功');
      setTimeout(() => setMsg(''), 3000);
    } catch (err: any) {
      setMsg('上傳失敗: ' + err.message);
    }
    setUploading(false);
    setUploadPct(0);
  };

  const handleUpload = (e: Event) => {
    const input = e.target as HTMLInputElement;
    const files = Array.from(input.files || []).filter((f) => f.type.startsWith('image/'));
    if (files.length) doUpload(files);
    input.value = '';
  };

  const handleDrop = (e: DragEvent) => {
    e.preventDefault();
    setDragOver(false);
    const files = Array.from(e.dataTransfer?.files || []).filter((f) => f.type.startsWith('image/'));
    if (files.length) doUpload(files);
  };

  const handleOCR = async () => {
    setOcrRunning(true);
    setMsg('OCR 辨識中，請稍候...');
    try {
      const result = await triggerOCR(rid);
      const safe = { categories: result?.categories || [] };
      setMenu(safe);
      setSavedMenu(safe);
      setMsg('OCR 完成！請檢查並修正菜單內容');
    } catch (err: any) {
      setMsg('OCR 失敗: ' + err.message);
    }
    setOcrRunning(false);
  };

  const handleDeletePhoto = async (photoId: number) => {
    if (!confirm('確定要刪除這張照片嗎？')) return;
    setDeletingPhotoId(photoId);
    try {
      await deleteMenuPhoto(rid, photoId);
      setPhotos(prev => prev.filter(p => p.id !== photoId));
    } catch (err: any) {
      setMsg('刪除失敗: ' + err.message);
    }
    setDeletingPhotoId(null);
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

  const updateOption = (ci: number, ii: number, ogIdx: number, optIdx: number, update: Partial<{ name: string; price_adjustment: number }>) => {
    setMenu((prev) => {
      const cats = [...prev.categories];
      const items = [...cats[ci].items];
      const groups = [...(items[ii].option_groups || [])];
      const options = [...groups[ogIdx].options];
      options[optIdx] = { ...options[optIdx], ...update };
      groups[ogIdx] = { ...groups[ogIdx], options };
      items[ii] = { ...items[ii], option_groups: groups };
      cats[ci] = { ...cats[ci], items };
      return { ...prev, categories: cats };
    });
  };

  const applyBatchOG = (catIdx: number) => {
    const opts = batchOG.options.split(',').map(s => s.trim()).filter(Boolean).map(s => {
      const m = s.match(/^(.+?)\+(\d+)$/);
      return m
        ? { name: m[1].trim(), price_adjustment: parseInt(m[2]) }
        : { name: s, price_adjustment: 0 };
    });
    if (!batchOG.name || !opts.length) return;
    const group = { name: batchOG.name, min_choices: batchOG.min, max_choices: batchOG.max, options: opts };
    setMenu(prev => {
      const cats = [...prev.categories];
      const items = cats[catIdx].items.map(item => ({
        ...item,
        option_groups: [...(item.option_groups || []), group],
      }));
      cats[catIdx] = { ...cats[catIdx], items };
      return { ...prev, categories: cats };
    });
    setBatchOG(batchOGDefault);
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
        <label
          class={`block border-2 border-dashed rounded-xl p-8 text-center transition-colors ${
            dragOver ? 'border-amber-500 bg-amber-50/50' :
            uploading ? 'border-amber-300 bg-amber-50/50' :
            'border-slate-200 hover:border-amber-400 hover:bg-amber-50/30 cursor-pointer'
          }`}
          onDragOver={(e) => { e.preventDefault(); setDragOver(true); }}
          onDragEnter={(e) => { e.preventDefault(); setDragOver(true); }}
          onDragLeave={() => setDragOver(false)}
          onDrop={handleDrop}
        >
          {uploading ? (
            <>
              <div class="w-48 mx-auto bg-slate-200 rounded-full h-2 mb-3">
                <div class="bg-amber-500 h-2 rounded-full transition-all" style={{ width: `${uploadPct}%` }} />
              </div>
              <p class="font-medium text-slate-700 text-sm">上傳中 {uploadPct}%</p>
            </>
          ) : (
            <>
              <div class="text-3xl mb-2">{dragOver ? '📥' : '📷'}</div>
              <p class="font-medium text-slate-700 text-sm">{dragOver ? '放開以上傳' : '點擊選擇或拖曳照片至此'}</p>
              <p class="text-xs text-slate-400 mt-1">支援 JPG、PNG，可多選</p>
            </>
          )}
          <input type="file" multiple onChange={handleUpload} disabled={uploading} class="hidden" />
        </label>
        {/* Thumbnails */}
        {photos.length > 0 && (
          <div class="mt-4 grid grid-cols-4 sm:grid-cols-6 gap-2">
            {photos.map((p) => (
              <div key={p.id} class="relative group aspect-square rounded-lg overflow-hidden border border-slate-200 hover:border-amber-400 transition-colors">
                <a href={p.url} target="_blank" class="block w-full h-full">
                  <img src={p.url} alt={p.file_name} class="w-full h-full object-cover" loading="lazy" />
                </a>
                <button
                  onClick={() => handleDeletePhoto(p.id)}
                  disabled={deletingPhotoId === p.id}
                  class="absolute top-1 right-1 w-6 h-6 bg-black/60 hover:bg-red-600 text-white rounded-full text-xs flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity disabled:opacity-50"
                >
                  {deletingPhotoId === p.id ? '...' : '✕'}
                </button>
              </div>
            ))}
          </div>
        )}
        <button
          onClick={handleOCR}
          disabled={ocrRunning || photos.length === 0}
          class="mt-3 w-full bg-slate-800 text-white px-4 py-2.5 rounded-lg text-sm font-medium hover:bg-slate-700 disabled:opacity-50 transition-colors"
        >
          {ocrRunning ? '辨識中...' : `開始 OCR 辨識${photos.length > 0 ? ` (${photos.length} 張)` : ''}`}
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
              <button onClick={(e) => { e.stopPropagation(); setBatchOG(prev => prev.cat === ci ? batchOGDefault : { ...batchOGDefault, cat: ci }); }} class="text-slate-400 hover:text-amber-600 text-xs transition-colors">批次選項群組</button>
              <button onClick={(e) => { e.stopPropagation(); removeCategory(ci); }} class="text-slate-400 hover:text-red-500 text-sm transition-colors">刪除</button>
              <span class={`text-slate-400 transition-transform ${collapsed.has(ci) ? '-rotate-90' : ''}`}>▾</span>
            </div>

            {/* Batch option group form */}
            {batchOG.cat === ci && (
              <div class="px-4 py-3 bg-amber-50/50 border-b border-slate-100 space-y-2">
                <p class="text-sm font-medium text-slate-700">批次新增選項群組</p>
                <input
                  class={inputClass}
                  placeholder="群組名稱（如：辣度）"
                  value={batchOG.name}
                  onInput={(e) => setBatchOG(prev => ({ ...prev, name: (e.target as HTMLInputElement).value }))}
                />
                <input
                  class={inputClass}
                  placeholder="選項，逗號分隔（如：小辣, 中辣, 大辣+10）"
                  value={batchOG.options}
                  onInput={(e) => setBatchOG(prev => ({ ...prev, options: (e.target as HTMLInputElement).value }))}
                />
                <div class="flex gap-2 items-center">
                  <label class="text-xs text-slate-500 flex items-center gap-1">最少 <input type="number" min={0} value={batchOG.min} onInput={(e) => setBatchOG(prev => ({ ...prev, min: parseInt((e.target as HTMLInputElement).value) || 0 }))} class={`${inputClass} w-16`} /></label>
                  <label class="text-xs text-slate-500 flex items-center gap-1">最多 <input type="number" min={1} value={batchOG.max} onInput={(e) => setBatchOG(prev => ({ ...prev, max: parseInt((e.target as HTMLInputElement).value) || 1 }))} class={`${inputClass} w-16`} /></label>
                  <div class="flex-1" />
                  <button onClick={() => setBatchOG(batchOGDefault)} class="text-xs text-slate-400 hover:text-slate-600">取消</button>
                  <button onClick={() => applyBatchOG(ci)} class="text-xs bg-amber-600 text-white px-3 py-1.5 rounded-lg font-medium hover:bg-amber-700 transition-colors">套用到所有品項</button>
                </div>
              </div>
            )}

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
                            <select
                              value={item.price < 0 ? String(item.price) : 'custom'}
                              onChange={(e) => {
                                const v = (e.target as HTMLSelectElement).value;
                                updateItem(ci, ii, { price: v === 'custom' ? 0 : parseInt(v) });
                              }}
                              class={`${inputClass} w-24`}
                            >
                              <option value="custom">自訂</option>
                              <option value="-1">未知</option>
                              <option value="-2">時價</option>
                            </select>
                            {item.price >= 0 && (
                              <>
                                <input type="number" value={item.price} onInput={(e) => updateItem(ci, ii, { price: parseInt((e.target as HTMLInputElement).value) || 0 })} class={`${inputClass} w-28`} min={0} />
                                <span class="text-sm text-slate-400">元</span>
                              </>
                            )}
                          </div>
                          {/* Option Groups */}
                          {item.option_groups?.map((og, ogIdx) => (
                            <div key={ogIdx} class="mt-3 p-3 bg-slate-50 rounded-lg border border-slate-200">
                              <div class="flex items-center gap-2 mb-2">
                                <input
                                  class={inputClass}
                                  value={og.name}
                                  placeholder="選項群組名稱"
                                  onInput={(e) => {
                                    const val = (e.target as HTMLInputElement).value;
                                    setMenu((prev) => {
                                      const cats = [...prev.categories];
                                      const items = [...cats[ci].items];
                                      const groups = [...(items[ii].option_groups || [])];
                                      groups[ogIdx] = { ...groups[ogIdx], name: val };
                                      items[ii] = { ...items[ii], option_groups: groups };
                                      cats[ci] = { ...cats[ci], items };
                                      return { ...prev, categories: cats };
                                    });
                                  }}
                                />
                                <label class="text-xs text-slate-400 flex items-center gap-0.5 whitespace-nowrap">
                                  <input type="number" min={0} value={og.min_choices} class="w-8 bg-transparent border-none text-xs text-slate-500 p-0 text-center focus:outline-none" onInput={(e) => {
                                    const val = parseInt((e.target as HTMLInputElement).value) || 0;
                                    setMenu((prev) => { const cats = [...prev.categories]; const items = [...cats[ci].items]; const groups = [...(items[ii].option_groups || [])]; groups[ogIdx] = { ...groups[ogIdx], min_choices: val }; items[ii] = { ...items[ii], option_groups: groups }; cats[ci] = { ...cats[ci], items }; return { ...prev, categories: cats }; });
                                  }} />-<input type="number" min={0} value={og.max_choices} class="w-8 bg-transparent border-none text-xs text-slate-500 p-0 text-center focus:outline-none" onInput={(e) => {
                                    const val = parseInt((e.target as HTMLInputElement).value) || 0;
                                    setMenu((prev) => { const cats = [...prev.categories]; const items = [...cats[ci].items]; const groups = [...(items[ii].option_groups || [])]; groups[ogIdx] = { ...groups[ogIdx], max_choices: val }; items[ii] = { ...items[ii], option_groups: groups }; cats[ci] = { ...cats[ci], items }; return { ...prev, categories: cats }; });
                                  }} />
                                </label>
                                <button
                                  class="text-red-400 hover:text-red-600 text-sm"
                                  onClick={() => {
                                    setMenu((prev) => {
                                      const cats = [...prev.categories];
                                      const items = [...cats[ci].items];
                                      const groups = (items[ii].option_groups || []).filter((_, i) => i !== ogIdx);
                                      items[ii] = { ...items[ii], option_groups: groups.length ? groups : undefined };
                                      cats[ci] = { ...cats[ci], items };
                                      return { ...prev, categories: cats };
                                    });
                                  }}
                                >✕</button>
                              </div>
                              <div class="flex flex-wrap gap-1">
                                {og.options.map((opt, optIdx) => (
                                  <span key={optIdx} class="inline-flex items-center gap-1 bg-white border border-slate-200 rounded px-2 py-0.5 text-sm">
                                    <input
                                      class="border-none bg-transparent text-sm w-20 p-0 focus:outline-none"
                                      value={opt.name}
                                      onInput={(e) => updateOption(ci, ii, ogIdx, optIdx, { name: (e.target as HTMLInputElement).value })}
                                    />
                                    <input
                                      type="number"
                                      class="border-none bg-transparent text-xs w-12 p-0 text-amber-600 focus:outline-none text-right"
                                      value={opt.price_adjustment}
                                      onInput={(e) => updateOption(ci, ii, ogIdx, optIdx, { price_adjustment: parseInt((e.target as HTMLInputElement).value) || 0 })}
                                      placeholder="±"
                                    />
                                    <button
                                      class="text-red-300 hover:text-red-500 text-xs"
                                      onClick={() => {
                                        setMenu((prev) => {
                                          const cats = [...prev.categories];
                                          const items = [...cats[ci].items];
                                          const groups = [...(items[ii].option_groups || [])];
                                          const options = groups[ogIdx].options.filter((_, i) => i !== optIdx);
                                          groups[ogIdx] = { ...groups[ogIdx], options };
                                          items[ii] = { ...items[ii], option_groups: groups };
                                          cats[ci] = { ...cats[ci], items };
                                          return { ...prev, categories: cats };
                                        });
                                      }}
                                    >✕</button>
                                  </span>
                                ))}
                                <button
                                  class="text-xs text-amber-600 hover:text-amber-700 px-2 py-0.5 border border-dashed border-amber-300 rounded"
                                  onClick={() => {
                                    setMenu((prev) => {
                                      const cats = [...prev.categories];
                                      const items = [...cats[ci].items];
                                      const groups = [...(items[ii].option_groups || [])];
                                      const options = [...groups[ogIdx].options, { name: "", price_adjustment: 0 }];
                                      groups[ogIdx] = { ...groups[ogIdx], options };
                                      items[ii] = { ...items[ii], option_groups: groups };
                                      cats[ci] = { ...cats[ci], items };
                                      return { ...prev, categories: cats };
                                    });
                                  }}
                                >+ 選項</button>
                              </div>
                            </div>
                          ))}
                          <button
                            class="text-xs text-slate-400 hover:text-slate-600 mt-2"
                            onClick={() => {
                              setMenu((prev) => {
                                const cats = [...prev.categories];
                                const items = [...cats[ci].items];
                                const groups = [...(items[ii].option_groups || []), {
                                  name: "",
                                  min_choices: 1,
                                  max_choices: 1,
                                  options: [{ name: "", price_adjustment: 0 }],
                                }];
                                items[ii] = { ...items[ii], option_groups: groups };
                                cats[ci] = { ...cats[ci], items };
                                return { ...prev, categories: cats };
                              });
                            }}
                          >+ 選項群組</button>
                          <div class="flex gap-2 justify-end mt-2 pt-2 border-t border-slate-100">
                            <button onClick={() => removeItem(ci, ii)} class="text-xs text-red-500 hover:text-red-600">刪除品項</button>
                            <button onClick={() => setEditingItem(null)} class="text-xs text-amber-600 font-medium hover:text-amber-700">完成</button>
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
                            <p class="text-sm font-medium text-slate-800 truncate">
                              {item.name}
                              {item.option_groups?.length ? <span class="text-xs text-amber-500 ml-2">({item.option_groups.length} 選項群組)</span> : null}
                            </p>
                            {item.description && <p class="text-xs text-slate-400 truncate">{item.description}</p>}
                          </div>
                          <span class="text-sm font-medium text-slate-700 tabular-nums">{item.price === -1 ? '未知' : item.price === -2 ? '時價' : `$${item.price}`}</span>
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
