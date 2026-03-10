# Frontend UI Redesign

## Goal

Transform the functional but plain Preact + Tailwind frontend into a polished restaurant management dashboard with warm branding, persistent navigation, and purpose-built layouts for menu editing and order management.

## Color System

| Role | Current | New | Class |
|------|---------|-----|-------|
| Primary CTA | `blue-600` | Amber | `bg-amber-600 hover:bg-amber-700` |
| Body background | `gray-50` | Warm off-white | `bg-stone-50` |
| Cards | `border rounded` | Elevated cards | `bg-white rounded-xl shadow-sm border border-slate-100` |
| Published/success | `green-100` | Emerald | `bg-emerald-100 text-emerald-700` |
| Inputs | `border rounded px-3 py-2` | Ring-focus | `border-slate-200 rounded-lg focus:ring-2 focus:ring-amber-500/20 focus:border-amber-500` |

## Architecture Changes

### 1. App Shell (Layout component)

New `Layout.tsx` wrapping all authenticated routes:
- **Sidebar** (`w-60 bg-slate-900`): Logo, nav links with inline SVG icons, active state with amber accent, logout at bottom. Collapses to slide-over drawer on mobile via hamburger in top bar.
- **Top bar** (`bg-white border-b`): Current restaurant name (when in restaurant context), owner name.
- **Main content**: `ml-60 min-h-screen bg-stone-50 p-6` (responsive: `ml-0` on mobile).

Navigation items adapt to context:
- Root: "Dashboard" only
- Inside restaurant: Settings, Menu, Orders, Public Page link

### 2. Login Page

Split layout on desktop: left 60% gradient brand panel (`from-amber-500 to-orange-600`), right 40% form card (`shadow-lg rounded-2xl`). Full-width form on mobile with brand header.

### 3. Dashboard

- Optional summary stats row (restaurant count, published count) in `grid-cols-2 lg:grid-cols-4`.
- Restaurant cards in `grid-cols-1 md:grid-cols-2 lg:grid-cols-3`. Each card: amber top accent strip, name, address, published badge (`rounded-full`), footer with quick-action links to Settings/Menu/Orders.
- Create restaurant: slide-up modal with backdrop instead of inline form.
- Delete: moved to "..." dropdown menu on card.

### 4. Restaurant Settings

- Publish state: prominent banner at top (emerald when published, slate when draft).
- Proper underline tabs for sub-nav (Settings/Menu/Orders) replacing plain buttons.
- Form grouped into sections with headers (Basic Info, Contact, Service Type).
- Toggle switches instead of checkboxes for dine-in/takeout/delivery.
- QR code: horizontal card layout with image + download/copy-link buttons.

### 5. Menu Editor

- Accordion categories: collapsible sections with category name, item count badge, expand/collapse chevron.
- Item rows: read-only display (name, description, price) with hover-reveal edit/delete actions. Click to expand inline editing.
- Photo upload: dashed-border drop zone (`border-2 border-dashed rounded-xl`) replacing hidden file input.
- Floating save bar: fixed to bottom of viewport, appears when changes exist, with discard/save buttons.
- Drag-and-drop reordering via SortableJS for both categories and items within categories.

### 6. Order Management

- **Desktop (lg+)**: Kanban board with 4 columns (pending/confirmed/preparing/completed). Each column has colored left border accent, count badge in header, scrollable card list.
- **Mobile**: Keep current filter-tab list view.
- Order cards: relative timestamps ("2 分鐘前"), table label, total, action button.
- Pulsing dot indicator for live polling status.

### 7. Global Improvements

- Loading states: skeleton screens with `animate-pulse` placeholder blocks instead of "載入中..." text.
- Input styling: consistent `rounded-lg` with amber focus ring across all forms.
- Typography: `tracking-tight` on headings, `text-slate-800` primary, `text-slate-500` secondary.

## New Dependencies

| Package | Purpose | Size |
|---------|---------|------|
| sortablejs | Drag-and-drop for menu reordering | ~5KB gz |
| @types/sortablejs | TypeScript types (dev only) | - |

No component library needed — all UI built with Tailwind utility classes using Flowbite/Heroicons patterns (copy-paste, zero runtime).

## Files Changed

- `src/app.tsx` — Add Layout wrapper with sidebar/topbar
- `src/components/Layout.tsx` — New: app shell (sidebar, topbar, mobile drawer)
- `src/components/Skeleton.tsx` — New: reusable skeleton loading components
- `src/components/Toggle.tsx` — New: toggle switch component
- `src/components/Modal.tsx` — New: modal/dialog component
- `src/index.css` — Font import, base styles
- `src/pages/Login.tsx` — Split layout redesign
- `src/pages/Dashboard.tsx` — Card grid, stats, modal create
- `src/pages/RestaurantEdit.tsx` — Sections, tabs, toggles, publish banner
- `src/pages/MenuEditor.tsx` — Accordion, inline edit, drag-and-drop, floating save, drop zone
- `src/pages/Orders.tsx` — Kanban board (desktop) + list (mobile)

## What We're NOT Doing

- No i18n system (strings stay hardcoded Chinese)
- No dark mode
- No animations library (Tailwind transitions only)
- No component library dependency (all hand-rolled with Tailwind)
- No router change (preact-router stays)
