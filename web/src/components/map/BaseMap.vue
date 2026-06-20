<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue';
import L from 'leaflet';
import 'leaflet/dist/leaflet.css';

const emit = defineEmits(['ready']);

const props = defineProps({
  center: {
    type: Array,
    default: () => [31.23, 121.47],
  },
  zoom: {
    type: Number,
    default: 8,
  },
});

const mapEl = ref(null);
const loading = ref(true);
let map = null;
let tileLayer = null;
let ro = null;
let loadTimer = null;

function onTilesLoaded() {
  loading.value = false;
  if (loadTimer) {
    clearTimeout(loadTimer);
    loadTimer = null;
  }
}

function initMap() {
  if (!mapEl.value || map) return;

  map = L.map(mapEl.value, {
    center: props.center,
    zoom: props.zoom,
    zoomControl: true,
    attributionControl: true,
  });

  tileLayer = L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png', {
    attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OSM</a> &copy; <a href="https://carto.com/">CARTO</a>',
    subdomains: 'abcd',
    maxZoom: 20,
  });
  tileLayer.on('load', onTilesLoaded);
  tileLayer.addTo(map);

  loadTimer = setTimeout(onTilesLoaded, 15000);

  ro = new ResizeObserver(() => map?.invalidateSize());
  ro.observe(mapEl.value);
  requestAnimationFrame(() => {
    map?.invalidateSize();
    emit('ready', map);
  });
}

onMounted(initMap);

onUnmounted(() => {
  if (loadTimer) clearTimeout(loadTimer);
  tileLayer?.off('load', onTilesLoaded);
  ro?.disconnect();
  map?.remove();
  map = null;
  tileLayer = null;
});

watch(
  () => props.center,
  (c) => {
    if (map && c?.length === 2) map.setView(c, map.getZoom());
  },
);

watch(
  () => props.zoom,
  (z) => {
    if (map) map.setZoom(z);
  },
);

defineExpose({
  getMap: () => map,
});
</script>

<template>
  <div class="base-map-wrap">
    <div ref="mapEl" class="base-map" />
    <div v-show="loading" class="map-loading">
      <div class="map-loading-spinner" />
      <span>地图加载中...</span>
    </div>
  </div>
</template>

<style scoped>
.base-map-wrap {
  position: relative;
  width: 100%;
  height: 100%;
}

.base-map {
  width: 100%;
  height: 100%;
  background: #000;
}

/* 暗色底图提亮：保留深色风格，避免 pure dark 过黑 */
.base-map :deep(.leaflet-tile-pane) {
  filter: brightness(1.45) contrast(0.92) saturate(0.95);
}

.map-loading {
  position: absolute;
  inset: 0;
  z-index: 1000;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 14px;
  background: #000;
  color: #888;
  font-size: 13px;
  user-select: none;
}

.map-loading-spinner {
  width: 32px;
  height: 32px;
  border: 3px solid #333;
  border-top-color: #7eb8e8;
  border-radius: 50%;
  animation: map-spin 0.75s linear infinite;
}

@keyframes map-spin {
  to { transform: rotate(360deg); }
}

.base-map :deep(.leaflet-control-zoom a) {
  background: #353d4f;
  color: #c8d0dc;
  border-color: #4a5568;
}

.base-map :deep(.leaflet-control-zoom a:hover) {
  background: #424b5e;
  color: #eef2f7;
}

.base-map :deep(.leaflet-control-attribution) {
  background: rgba(42, 49, 64, 0.88);
  color: #8a93a8;
  font-size: 10px;
}

.base-map :deep(.leaflet-control-attribution a) {
  color: #9cb8d8;
}
</style>
