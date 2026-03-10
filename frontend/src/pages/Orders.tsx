import { useState, useEffect, useRef, useCallback } from 'preact/hooks';
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

interface OrderCardProps {
  order: Order;
  updatingId: number | null;
  onAdvance: (o: Order) => void;
  onCancel: (o: Order) => void;
}

function OrderCard({ order: o, updatingId, onAdvance, onCancel }: OrderCardProps) {
  return (
    <div class={`bg-white rounded-lg shadow-sm border-l-4 ${STATUS_COLORS[o.status] || 'border-l-slate-200'} border border-slate-100 p-3`}>
      <div class="flex items-center justify-between mb-2">
        <span class="font-mono text-xs text-slate-400">#{o.id}</span>
        <span class="text-xs text-slate-400">{relativeTime(o.created_at)}</span>
      </div>
      {o.table_label && <p class="text-sm font-medium text-slate-800 mb-1">桌號 {o.table_label}</p>}
      <p class="font-bold text-slate-800 text-lg">${o.total_amount} 元</p>
      {o.status !== 'completed' && o.status !== 'cancelled' && (
        <div class="flex gap-2 mt-3">
          {NEXT_STATUS[o.status] && (
            <button
              onClick={() => onAdvance(o)}
              disabled={updatingId === o.id}
              class={`flex-1 text-white text-xs py-1.5 rounded-lg font-medium disabled:opacity-50 transition-colors ${ACTION_COLORS[o.status]}`}
            >
              {updatingId === o.id ? '...' : STATUS_LABELS[NEXT_STATUS[o.status]]}
            </button>
          )}
          <button
            onClick={() => onCancel(o)}
            disabled={updatingId === o.id}
            class="text-xs text-red-500 hover:text-red-600 px-2 disabled:opacity-50"
          >
            取消
          </button>
        </div>
      )}
    </div>
  );
}

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

  const load = useCallback(() => {
    listOrders(rid)
      .then(setOrders)
      .catch(() => {})
      .finally(() => setLoading(false));
  }, [rid]);

  useEffect(() => {
    load();
    timerRef.current = setInterval(load, 10000);
    return () => clearInterval(timerRef.current);
  }, [load]);

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
                {col.map((o) => <OrderCard key={o.id} order={o} updatingId={updatingId} onAdvance={advance} onCancel={cancel} />)}
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
            {filteredOrders.map((o) => <OrderCard key={o.id} order={o} updatingId={updatingId} onAdvance={advance} onCancel={cancel} />)}
          </div>
        )}
      </div>
    </div>
  );
}
