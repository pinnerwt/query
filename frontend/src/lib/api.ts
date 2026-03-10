const BASE = '/api';

async function request<T>(path: string, opts?: RequestInit): Promise<T> {
  const token = localStorage.getItem('token');
  const headers: Record<string, string> = {
    ...(opts?.headers as Record<string, string>),
  };
  if (token) headers['Authorization'] = `Bearer ${token}`;
  if (opts?.body && typeof opts.body === 'string') {
    headers['Content-Type'] = 'application/json';
  }
  const res = await fetch(`${BASE}${path}`, { ...opts, headers });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || res.statusText);
  }
  const ct = res.headers.get('content-type');
  if (ct && ct.includes('application/json')) {
    return res.json();
  }
  return undefined as unknown as T;
}

export interface Owner {
  id: number;
  email: string;
  name: string;
}

export interface Restaurant {
  id: number;
  owner_id: number;
  name: string;
  slug: string;
  address: string;
  phone_number: string;
  website: string;
  dine_in: boolean;
  takeout: boolean;
  delivery: boolean;
  minimum_spend: number;
  is_published: boolean;
}

export interface MenuCategory {
  id: number;
  name: string;
  sort_order: number;
  items: MenuItem[];
}

export interface MenuItem {
  id: number;
  name: string;
  description: string;
  price: number;
  is_available: boolean;
  category_id: number;
  price_tiers?: PriceTier[];
}

export interface PriceTier {
  label: string;
  quantity: number;
  price: number;
}

export interface ComboMeal {
  id: number;
  name: string;
  description: string;
  price: number;
  groups: ComboGroup[];
}

export interface ComboGroup {
  id: number;
  name: string;
  min_choices: number;
  max_choices: number;
  options: ComboOption[];
}

export interface ComboOption {
  id: number;
  item_name: string;
  price_adjustment: number;
}

export interface MenuData {
  categories: MenuCategory[];
  combos: ComboMeal[];
}

export interface RestaurantHour {
  day_of_week: number;
  open_time: string;
  close_time: string;
}

export interface Order {
  id: number;
  restaurant_id: number;
  status: string;
  table_label: string;
  total_amount: number;
  created_at: string;
  items?: OrderItem[];
}

export interface OrderItem {
  id: number;
  item_name: string;
  quantity: number;
  unit_price: number;
  notes: string;
}

// Auth
export const register = (email: string, password: string, name: string) =>
  request<{ token: string; owner: Owner }>('/auth/register', {
    method: 'POST',
    body: JSON.stringify({ email, password, name }),
  });

export const login = (email: string, password: string) =>
  request<{ token: string; owner: Owner }>('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  });

export const getMe = () => request<Owner>('/auth/me');

// Restaurants
export const createRestaurant = (data: Partial<Restaurant>) =>
  request<Restaurant>('/restaurants', {
    method: 'POST',
    body: JSON.stringify(data),
  });

export const listMyRestaurants = () =>
  request<Restaurant[]>('/restaurants/mine');

export const getRestaurant = (id: number) =>
  request<Restaurant>(`/restaurants/${id}`);

export const updateRestaurant = (id: number, data: Partial<Restaurant>) =>
  request<Restaurant>(`/restaurants/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });

export const deleteRestaurant = (id: number) =>
  request<void>(`/restaurants/${id}`, { method: 'DELETE' });

export const publishRestaurant = (id: number, is_published: boolean) =>
  request<Restaurant>(`/restaurants/${id}/publish`, {
    method: 'PUT',
    body: JSON.stringify({ is_published }),
  });

// Hours
export const setRestaurantHours = (id: number, hours: RestaurantHour[]) =>
  request<void>(`/restaurants/${id}/hours`, {
    method: 'PUT',
    body: JSON.stringify({ hours }),
  });

// Menu
export const getMenu = (id: number) =>
  request<MenuData>(`/restaurants/${id}/menu`);

export const saveMenu = (id: number, menu: MenuData) =>
  request<void>(`/restaurants/${id}/menu`, {
    method: 'PUT',
    body: JSON.stringify(menu),
  });

// Photos / OCR
export const uploadPhotos = (id: number, files: FileList) => {
  const form = new FormData();
  for (let i = 0; i < files.length; i++) form.append('photos', files[i]);
  const token = localStorage.getItem('token');
  return fetch(`${BASE}/restaurants/${id}/menu-photos`, {
    method: 'POST',
    headers: token ? { Authorization: `Bearer ${token}` } : {},
    body: form,
  }).then(async (r) => {
    if (!r.ok) throw new Error(await r.text());
    return r.json();
  });
};

export const triggerOCR = (id: number) =>
  request<MenuData>(`/restaurants/${id}/ocr`, { method: 'POST' });

// QR
export const getQRUrl = (id: number) => `${BASE}/restaurants/${id}/qr`;

// Orders
export const listOrders = (id: number, status?: string) =>
  request<Order[]>(`/restaurants/${id}/orders${status ? `?status=${status}` : ''}`);

export const updateOrderStatus = (restaurantId: number, orderId: number, status: string) =>
  request<Order>(`/restaurants/${restaurantId}/orders/${orderId}/status`, {
    method: 'PUT',
    body: JSON.stringify({ status }),
  });
