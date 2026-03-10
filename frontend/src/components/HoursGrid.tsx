import { useState, useEffect, useRef, useCallback } from 'preact/hooks';
import type { RestaurantHour } from '../lib/api';

const DAY_LABELS = ['週一', '週二', '週三', '週四', '週五', '週六', '週日'];

interface Props {
  hours: RestaurantHour[];
  dirty: boolean;
  onGridChange: (hours: RestaurantHour[]) => void;
  onSave: () => void;
  saving: boolean;
  message: string;
}

function timeToSlot(time: string, halfHour: boolean): number {
  const [h, m] = time.split(':').map(Number);
  return halfHour ? h * 2 + (m >= 30 ? 1 : 0) : h;
}

function slotToTime(slot: number, halfHour: boolean): string {
  if (halfHour) {
    const h = Math.floor(slot / 2);
    const m = slot % 2 === 0 ? '00' : '30';
    return `${String(h).padStart(2, '0')}:${m}`;
  }
  return `${String(slot).padStart(2, '0')}:00`;
}

function hoursToGrid(hours: RestaurantHour[], slots: number, halfHour: boolean): boolean[][] {
  const grid: boolean[][] = Array.from({ length: 7 }, () => Array(slots).fill(false));
  for (const h of hours) {
    const start = timeToSlot(h.open_time, halfHour);
    const end = timeToSlot(h.close_time, halfHour);
    const endSlot = end === 0 ? slots : end;
    for (let s = start; s < endSlot; s++) {
      grid[h.day_of_week][s] = true;
    }
  }
  return grid;
}

function gridToHours(grid: boolean[][], halfHour: boolean): RestaurantHour[] {
  const hours: RestaurantHour[] = [];
  for (let day = 0; day < 7; day++) {
    let inRange = false;
    let start = 0;
    for (let s = 0; s < grid[day].length; s++) {
      if (grid[day][s] && !inRange) {
        inRange = true;
        start = s;
      } else if (!grid[day][s] && inRange) {
        inRange = false;
        hours.push({
          day_of_week: day,
          open_time: slotToTime(start, halfHour),
          close_time: slotToTime(s, halfHour),
        });
      }
    }
    if (inRange) {
      hours.push({
        day_of_week: day,
        open_time: slotToTime(start, halfHour),
        close_time: slotToTime(0, halfHour),
      });
    }
  }
  return hours;
}

// Fill all cells on the line between two points
function fillLine(
  grid: boolean[][],
  from: { day: number; slot: number },
  to: { day: number; slot: number },
  value: boolean,
): boolean[][] {
  const next = grid.map((row) => [...row]);
  // Bresenham-style interpolation
  let d0 = from.day, s0 = from.slot;
  const d1 = to.day, s1 = to.slot;
  const dd = Math.abs(d1 - d0), ds = Math.abs(s1 - s0);
  const sd = d0 < d1 ? 1 : -1, ss = s0 < s1 ? 1 : -1;
  let err = dd - ds;
  while (true) {
    next[d0][s0] = value;
    if (d0 === d1 && s0 === s1) break;
    const e2 = 2 * err;
    if (e2 > -ds) { err -= ds; d0 += sd; }
    if (e2 < dd) { err += dd; s0 += ss; }
  }
  return next;
}

export default function HoursGrid({ hours, dirty, onGridChange, onSave, saving, message }: Props) {
  const [halfHour, setHalfHour] = useState(false);
  const slots = halfHour ? 48 : 24;
  const [grid, setGrid] = useState(() => hoursToGrid(hours, slots, halfHour));
  const dragging = useRef(false);
  const dragValue = useRef(false);
  const lastCell = useRef<{ day: number; slot: number } | null>(null);
  const gridRef = useRef<HTMLDivElement>(null);

  // Sync grid when hours prop changes (from server load)
  useEffect(() => {
    setGrid(hoursToGrid(hours, slots, halfHour));
  }, [hours, halfHour]);

  const emitChange = useCallback((g: boolean[][]) => {
    onGridChange(gridToHours(g, halfHour));
  }, [halfHour, onGridChange]);

  const handleMouseDown = useCallback((day: number, slot: number) => {
    const newVal = !grid[day][slot];
    dragValue.current = newVal;
    lastCell.current = { day, slot };
    dragging.current = true;
    setGrid((prev) => {
      const next = prev.map((row) => [...row]);
      next[day][slot] = newVal;
      emitChange(next);
      return next;
    });
  }, [grid, emitChange]);

  const handleMouseEnter = useCallback((day: number, slot: number) => {
    if (!dragging.current) return;
    const from = lastCell.current;
    lastCell.current = { day, slot };
    setGrid((prev) => {
      const next = from
        ? fillLine(prev, from, { day, slot }, dragValue.current)
        : prev.map((row) => [...row]);
      if (!from) next[day][slot] = dragValue.current;
      emitChange(next);
      return next;
    });
  }, [emitChange]);

  const handleMouseUp = useCallback(() => {
    dragging.current = false;
    lastCell.current = null;
  }, []);

  useEffect(() => {
    window.addEventListener('mouseup', handleMouseUp);
    window.addEventListener('touchend', handleMouseUp);
    return () => {
      window.removeEventListener('mouseup', handleMouseUp);
      window.removeEventListener('touchend', handleMouseUp);
    };
  }, [handleMouseUp]);

  const handleTouchMove = useCallback((e: TouchEvent) => {
    if (!dragging.current || !gridRef.current) return;
    const touch = e.touches[0];
    const el = document.elementFromPoint(touch.clientX, touch.clientY) as HTMLElement | null;
    if (el && el.dataset.day && el.dataset.slot) {
      const day = parseInt(el.dataset.day);
      const slot = parseInt(el.dataset.slot);
      handleMouseEnter(day, slot);
    }
  }, [handleMouseEnter]);

  const toggleGranularity = () => {
    const nextHalf = !halfHour;
    const nextSlots = nextHalf ? 48 : 24;
    const currentHours = gridToHours(grid, halfHour);
    setHalfHour(nextHalf);
    const nextGrid = hoursToGrid(currentHours, nextSlots, nextHalf);
    setGrid(nextGrid);
    onGridChange(gridToHours(nextGrid, nextHalf));
  };

  const headerSlots = Array.from({ length: 24 }, (_, i) => i);

  return (
    <div>
      <div class="flex items-center justify-between mb-3">
        <h3 class="text-base font-semibold text-slate-800">營業時間</h3>
        <button
          type="button"
          onClick={toggleGranularity}
          class="text-xs text-amber-600 hover:text-amber-700 font-medium"
        >
          {halfHour ? '切換為整點' : '切換為半小時'}
        </button>
      </div>
      <p class="text-xs text-slate-400 mb-3">點擊或拖曳選取營業時段</p>
      <div class="overflow-x-auto -mx-6 px-6" ref={gridRef} onTouchMove={handleTouchMove}>
        <div class="inline-block min-w-full select-none">
          {/* Hour labels */}
          <div class="flex" style={{ paddingLeft: '2.5rem' }}>
            {headerSlots.map((h) => (
              <div
                key={h}
                class="text-[10px] text-slate-400 text-center flex-shrink-0"
                style={{ width: halfHour ? '2rem' : '1rem', minWidth: halfHour ? '2rem' : '1rem' }}
              >
                {h % (halfHour ? 3 : 6) === 0 ? h : ''}
              </div>
            ))}
          </div>
          {/* Grid rows */}
          {DAY_LABELS.map((label, day) => (
            <div key={day} class="flex items-center gap-1 mb-0.5">
              <span class="text-xs text-slate-500 w-9 text-right flex-shrink-0 font-medium">{label}</span>
              <div class="flex">
                {Array.from({ length: slots }, (_, slot) => (
                  <div
                    key={slot}
                    data-day={day}
                    data-slot={slot}
                    onMouseDown={() => handleMouseDown(day, slot)}
                    onMouseEnter={() => handleMouseEnter(day, slot)}
                    onTouchStart={() => handleMouseDown(day, slot)}
                    class={`flex-shrink-0 cursor-pointer transition-colors w-[0.9375rem] h-6 ${
                      grid[day][slot]
                        ? 'bg-amber-500 hover:bg-amber-600'
                        : 'bg-slate-100 hover:bg-slate-200'
                    } ${
                      slot === 0 ? 'rounded-l' : slot === slots - 1 ? 'rounded-r' : ''
                    }`}
                    style={{
                      minWidth: '0.9375rem',
                      borderRight: (halfHour ? slot % 2 === 1 : true) && slot < slots - 1 ? '1px solid rgba(255,255,255,0.3)' : 'none',
                    }}
                  />
                ))}
              </div>
            </div>
          ))}
        </div>
      </div>
      {/* Summary */}
      {hours.length > 0 && (() => {
        const byDay = new Map<number, RestaurantHour[]>();
        for (const h of hours) {
          const arr = byDay.get(h.day_of_week);
          if (arr) arr.push(h); else byDay.set(h.day_of_week, [h]);
        }
        return (
          <div class="mt-3 space-y-1">
            {DAY_LABELS.map((label, day) => {
              const dayHours = byDay.get(day);
              if (!dayHours) return null;
              return (
                <div key={day} class="flex items-center gap-2 text-xs text-slate-600">
                  <span class="font-medium w-8">{label}</span>
                  <span>{dayHours.map((h) => `${h.open_time}–${h.close_time === '00:00' ? '24:00' : h.close_time}`).join(', ')}</span>
                </div>
              );
            })}
          </div>
        );
      })()}
      {/* Save bar */}
      <div class="flex items-center justify-between mt-4 h-8">
        <span class="text-xs text-amber-600">{message}</span>
        {dirty && (
          <button
            type="button"
            onClick={onSave}
            disabled={saving}
            class="bg-amber-600 text-white px-4 py-1.5 rounded-lg text-sm font-medium hover:bg-amber-700 disabled:opacity-50 transition-colors"
          >
            {saving ? '儲存中...' : '儲存營業時間'}
          </button>
        )}
      </div>
    </div>
  );
}
