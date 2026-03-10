import { useState, useEffect, useRef } from 'preact/hooks';
import { route } from 'preact-router';
import { listOrders, updateOrderStatus } from '../lib/api';
import type { Order } from '../lib/api';
import type { RoutableProps } from '../lib/route';

const STATUS_LABELS: Record<string, string> = {
  pending: '待確認',
  confirmed: '已確認',
  preparing: '準備中',
  completed: '已完成',
  cancelled: '已取消',
};

const STATUS_COLORS: Record<string, string> = {
  pending: 'bg-yellow-100 text-yellow-800',
  confirmed: 'bg-blue-100 text-blue-800',
  preparing: 'bg-orange-100 text-orange-800',
  completed: 'bg-green-100 text-green-800',
  cancelled: 'bg-red-100 text-red-800',
};

const NEXT_STATUS: Record<string, string> = {
  pending: 'confirmed',
  confirmed: 'preparing',
  preparing: 'completed',
};

export default function Orders({ id = '' }: RoutableProps & { id?: string }) {
  const rid = parseInt(id);
  const [orders, setOrders] = useState<Order[]>([]);
  const [filter, setFilter] = useState('');
  const [loading, setLoading] = useState(true);
  const timerRef = useRef<ReturnType<typeof setInterval>>();

  const load = () => {
    listOrders(rid, filter)
      .then(setOrders)
      .catch(() => {})
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    load();
    timerRef.current = setInterval(load, 10000);
    return () => clearInterval(timerRef.current);
  }, [rid, filter]);

  const advance = async (order: Order) => {
    const next = NEXT_STATUS[order.status];
    if (!next) return;
    await updateOrderStatus(rid, order.id, next);
    load();
  };

  const cancel = async (order: Order) => {
    if (!confirm('確定取消此訂單？')) return;
    await updateOrderStatus(rid, order.id, 'cancelled');
    load();
  };

  return (
    <div class="max-w-2xl mx-auto p-4">
      <button
        onClick={() => route(`/app/restaurants/${rid}`)}
        class="text-blue-600 text-sm mb-4 inline-block"
      >
        &larr; 返回
      </button>

      <h1 class="text-2xl font-bold mb-4">訂單管理</h1>

      <div class="flex gap-2 mb-4 flex-wrap">
        {['', 'pending', 'confirmed', 'preparing', 'completed', 'cancelled'].map((s) => (
          <button
            key={s}
            onClick={() => setFilter(s)}
            class={`px-3 py-1 rounded text-sm ${
              filter === s ? 'bg-blue-600 text-white' : 'bg-gray-100 hover:bg-gray-200'
            }`}
          >
            {s === '' ? '全部' : STATUS_LABELS[s]}
          </button>
        ))}
      </div>

      {loading ? (
        <p class="text-gray-500">載入中...</p>
      ) : orders.length === 0 ? (
        <p class="text-gray-400">沒有訂單</p>
      ) : (
        <div class="space-y-3">
          {orders.map((o) => (
            <div key={o.id} class="border rounded p-4">
              <div class="flex justify-between items-start mb-2">
                <div>
                  <span class="font-mono text-sm text-gray-500">#{o.id}</span>
                  {o.table_label && (
                    <span class="ml-2 bg-gray-100 px-2 py-0.5 rounded text-sm">
                      {o.table_label}
                    </span>
                  )}
                </div>
                <span
                  class={`px-2 py-0.5 rounded text-xs font-medium ${
                    STATUS_COLORS[o.status] || ''
                  }`}
                >
                  {STATUS_LABELS[o.status] || o.status}
                </span>
              </div>
              <p class="text-lg font-bold mb-1">${o.total_amount} 元</p>
              <p class="text-xs text-gray-400 mb-2">
                {new Date(o.created_at).toLocaleString('zh-TW')}
              </p>
              <div class="flex gap-2">
                {NEXT_STATUS[o.status] && (
                  <button
                    onClick={() => advance(o)}
                    class="bg-blue-600 text-white px-3 py-1 rounded text-sm hover:bg-blue-700"
                  >
                    {STATUS_LABELS[NEXT_STATUS[o.status]]}
                  </button>
                )}
                {o.status !== 'completed' && o.status !== 'cancelled' && (
                  <button
                    onClick={() => cancel(o)}
                    class="text-red-600 border border-red-300 px-3 py-1 rounded text-sm hover:bg-red-50"
                  >
                    取消
                  </button>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
