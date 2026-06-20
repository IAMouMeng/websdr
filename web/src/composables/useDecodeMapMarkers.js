import { ref, computed, watch, onUnmounted } from 'vue';
import L from 'leaflet';
import { hasValidPos } from '@/utils/decodeItem.js';

// Keeps Leaflet markers in sync with decode snapshots. IDs are normalized to
// strings so MMSI (numeric in JSON) and selection state stay consistent.
export function useDecodeMapMarkers(mapRef, items, getId) {
  const markers = new Map();
  const selected = ref(null);
  const mapReady = ref(false);
  const pannedFor = ref(null);

  const withPos = computed(() => items.value.filter(hasValidPos));

  function idOf(item) {
    return String(getId(item));
  }

  function getMap() {
    return mapRef.value?.getMap?.() ?? null;
  }

  function clearMarkers() {
    const map = getMap();
    for (const [, m] of markers) {
      map?.removeLayer(m);
    }
    markers.clear();
  }

  function syncMarkers(createIcon, popupHtml) {
    const map = getMap();
    if (!map) return;

    const alive = new Set();
    const sel = selected.value;

    for (const item of withPos.value) {
      const id = idOf(item);
      alive.add(id);
      const isActive = sel === id;
      let m = markers.get(id);
      if (m) {
        m.setLatLng([item.lat, item.lon]);
        m.setIcon(createIcon(item, isActive));
        m.setPopupContent(popupHtml(item));
      } else {
        m = L.marker([item.lat, item.lon], { icon: createIcon(item, isActive) });
        m.bindPopup(popupHtml(item));
        m.on('click', () => {
          selected.value = id;
        });
        m.addTo(map);
        markers.set(id, m);
      }
    }

    for (const [id, m] of markers) {
      if (!alive.has(id)) {
        map.removeLayer(m);
        markers.delete(id);
      }
    }
  }

  function selectItem(item, minZoom = 9) {
    const id = idOf(item);
    selected.value = id;
    const map = getMap();
    if (map && hasValidPos(item)) {
      map.setView([item.lat, item.lon], Math.max(map.getZoom(), minZoom), { animate: true });
      pannedFor.value = id;
    }
  }

  function onMapReady() {
    mapReady.value = true;
  }

  // Prefer targets with a fix. Upgrade away from a no-position selection when
  // a positioned target appears (common for ADS-B before CPR pairs).
  function ensureSelection(minZoom = 9) {
    const positioned = withPos.value;
    const sel = selected.value;

    if (sel != null) {
      const cur = items.value.find((i) => idOf(i) === sel);
      if (!cur) {
        selected.value = null;
        pannedFor.value = null;
      } else if (!hasValidPos(cur) && positioned.length > 0) {
        selectItem(positioned[0], minZoom);
        return;
      } else if (hasValidPos(cur) && pannedFor.value !== sel) {
        selectItem(cur, minZoom);
        return;
      } else {
        return;
      }
    }

    if (positioned.length > 0) {
      selectItem(positioned[0], minZoom);
    } else if (items.value.length > 0) {
      selected.value = idOf(items.value[0]);
    }
  }

  function bindSync(createIcon, popupHtml, { minZoom = 9 } = {}) {
    const run = () => {
      if (!mapReady.value) return;
      ensureSelection(minZoom);
      syncMarkers(createIcon, popupHtml);
    };
    watch([items, withPos, mapReady, selected], run, { deep: true });
    return run;
  }

  onUnmounted(clearMarkers);

  return { selected, withPos, mapReady, onMapReady, syncMarkers, selectItem, bindSync, idOf };
}

export function fitMapToMarkers(map, latlngs, minZoom = 9) {
  if (!map || latlngs.length === 0) return;
  if (latlngs.length === 1) {
    map.setView(latlngs[0], Math.max(map.getZoom(), minZoom));
    return;
  }
  map.fitBounds(latlngs, { padding: [48, 48], maxZoom: 12 });
}
