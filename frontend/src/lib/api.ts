const BASE = '/api';

async function request<T>(path: string, opts?: RequestInit): Promise<T> {
  const headers: Record<string, string> = {
    ...(opts?.headers as Record<string, string>),
  };
  if (opts?.body && typeof opts.body === 'string') {
    headers['Content-Type'] = 'application/json';
  }
  const res = await fetch(`${BASE}${path}`, { ...opts, headers, credentials: 'include' });
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
  option_groups?: OptionGroup[];
}

export interface PriceTier {
  label: string;
  quantity: number;
  price: number;
}

export interface OptionGroup {
  name: string;
  min_choices: number;
  max_choices: number;
  options: OptionChoice[];
}

export interface OptionChoice {
  name: string;
  price_adjustment: number;
}

export interface MenuData {
  categories: MenuCategory[];
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
  request<{ owner: Owner }>('/auth/register', {
    method: 'POST',
    body: JSON.stringify({ email, password, name }),
  });

export const login = (email: string, password: string) =>
  request<{ owner: Owner }>('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  });

export const getMe = () => request<Owner>('/auth/me');

export const logout = () =>
  request<void>('/auth/logout', { method: 'POST' });

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
export const getRestaurantHours = (id: number) =>
  request<{ hours: RestaurantHour[] }>(`/restaurants/${id}/hours`).then((r) => r.hours);

export const setRestaurantHours = (id: number, hours: RestaurantHour[]) =>
  request<void>(`/restaurants/${id}/hours`, {
    method: 'PUT',
    body: JSON.stringify({ hours }),
  });

// Location
export interface RestaurantLocation {
  latitude: number | null;
  longitude: number | null;
}

export const getRestaurantLocation = (id: number) =>
  request<RestaurantLocation>(`/restaurants/${id}/location`);

export const setRestaurantLocation = (id: number, latitude: number, longitude: number) =>
  request<void>(`/restaurants/${id}/location`, {
    method: 'PUT',
    body: JSON.stringify({ latitude, longitude }),
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
export const uploadPhotos = (
  id: number,
  files: FileList | File[],
  onProgress?: (pct: number) => void,
): Promise<unknown> => {
  const form = new FormData();
  for (let i = 0; i < files.length; i++) form.append('photos', files[i]);
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open('POST', `${BASE}/restaurants/${id}/menu-photos`);
    xhr.withCredentials = true;
    if (onProgress) {
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable) onProgress(Math.round((e.loaded / e.total) * 100));
      };
    }
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve(JSON.parse(xhr.responseText));
      } else {
        reject(new Error(xhr.responseText || xhr.statusText));
      }
    };
    xhr.onerror = () => reject(new Error('網路錯誤'));
    xhr.send(form);
  });
};

export interface MenuPhoto {
  id: number;
  file_name: string;
  url: string;
}

export const listMenuPhotos = (id: number) =>
  request<{ photos: MenuPhoto[] }>(`/restaurants/${id}/menu-photos`).then((r) => r.photos);

export const deleteMenuPhoto = (restaurantId: number, photoId: number) =>
  request<void>(`/restaurants/${restaurantId}/menu-photos/${photoId}`, { method: 'DELETE' });

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
