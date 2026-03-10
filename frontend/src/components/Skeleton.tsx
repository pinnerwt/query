export function SkeletonCard() {
  return (
    <div class="bg-white rounded-xl shadow-sm border border-slate-100 p-4 animate-pulse">
      <div class="h-1.5 bg-amber-200 rounded -mx-4 -mt-4 mb-4" />
      <div class="h-5 bg-slate-200 rounded w-3/4 mb-3" />
      <div class="h-4 bg-slate-100 rounded w-1/2 mb-4" />
      <div class="border-t border-slate-100 -mx-4 px-4 pt-3 mt-3 flex gap-4">
        <div class="h-4 bg-slate-100 rounded w-12" />
        <div class="h-4 bg-slate-100 rounded w-12" />
        <div class="h-4 bg-slate-100 rounded w-12" />
      </div>
    </div>
  );
}

export function SkeletonList({ rows = 3 }: { rows?: number }) {
  return (
    <div class="space-y-3 animate-pulse">
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} class="bg-white rounded-xl shadow-sm border border-slate-100 p-4">
          <div class="h-4 bg-slate-200 rounded w-2/3 mb-2" />
          <div class="h-3 bg-slate-100 rounded w-1/3" />
        </div>
      ))}
    </div>
  );
}
