import { useEffect, useRef } from 'preact/hooks';
import L from 'leaflet';
import 'leaflet/dist/leaflet.css';

// Fix default marker icon (Leaflet CSS expects image paths)
const DefaultIcon = L.icon({
  iconUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon.png',
  iconRetinaUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon-2x.png',
  shadowUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-shadow.png',
  iconSize: [25, 41],
  iconAnchor: [12, 41],
  popupAnchor: [1, -34],
  shadowSize: [41, 41],
});

interface Props {
  latitude: number | null;
  longitude: number | null;
  dirty: boolean;
  onChange: (lat: number, lng: number) => void;
  onSave: () => void;
  saving: boolean;
  message: string;
}

// Default center: Taiwan
const DEFAULT_LAT = 25.033;
const DEFAULT_LNG = 121.565;
const DEFAULT_ZOOM = 13;

export default function MapPicker({ latitude, longitude, dirty, onChange, onSave, saving, message }: Props) {
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<L.Map | null>(null);
  const markerRef = useRef<L.Marker | null>(null);
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  useEffect(() => {
    if (!containerRef.current || mapRef.current) return;

    const lat = latitude ?? DEFAULT_LAT;
    const lng = longitude ?? DEFAULT_LNG;
    const zoom = latitude != null ? 16 : DEFAULT_ZOOM;

    const map = L.map(containerRef.current).setView([lat, lng], zoom);
    L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
      attribution: '&copy; OpenStreetMap',
      maxZoom: 19,
    }).addTo(map);

    const marker = L.marker([lat, lng], { icon: DefaultIcon, draggable: true }).addTo(map);
    marker.on('dragend', () => {
      const pos = marker.getLatLng();
      onChangeRef.current(pos.lat, pos.lng);
    });

    map.on('click', (e: L.LeafletMouseEvent) => {
      marker.setLatLng(e.latlng);
      onChangeRef.current(e.latlng.lat, e.latlng.lng);
    });

    mapRef.current = map;
    markerRef.current = marker;

    setTimeout(() => map.invalidateSize(), 100);

    return () => {
      map.remove();
      mapRef.current = null;
      markerRef.current = null;
    };
  }, []);

  // Update marker when saved lat/lng loads after mount
  useEffect(() => {
    if (!markerRef.current || !mapRef.current || latitude == null || longitude == null) return;
    markerRef.current.setLatLng([latitude, longitude]);
    mapRef.current.setView([latitude, longitude], 16);
  }, [latitude, longitude]);

  return (
    <div>
      <h3 class="text-base font-semibold text-slate-800 mb-2">餐廳位置</h3>
      <p class="text-xs text-slate-400 mb-3">點擊地圖或拖曳標記設定位置</p>
      <div ref={containerRef} class="rounded-lg border border-slate-200 overflow-hidden" style={{ height: '300px' }} />
      <div class="flex items-center justify-between mt-3 h-8">
        <div>
          {latitude != null && longitude != null && (
            <span class="text-xs text-slate-400">{latitude.toFixed(6)}, {longitude.toFixed(6)}</span>
          )}
          {message && <span class="text-xs text-amber-600 ml-2">{message}</span>}
        </div>
        {dirty && (
          <button
            type="button"
            onClick={onSave}
            disabled={saving}
            class="bg-amber-600 text-white px-4 py-1.5 rounded-lg text-sm font-medium hover:bg-amber-700 disabled:opacity-50 transition-colors"
          >
            {saving ? '儲存中...' : '儲存位置'}
          </button>
        )}
      </div>
    </div>
  );
}
