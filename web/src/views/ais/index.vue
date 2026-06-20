<script setup>
import { ref, computed, onMounted } from 'vue';
import L from 'leaflet';
import BaseMap from '@/components/map/BaseMap.vue';
import DecodeNotifyList from '@/components/decode/DecodeNotifyList.vue';
import { useDecodeService } from '@/composables/useDecodeService.js';
import { useDecodeMapMarkers, fitMapToMarkers } from '@/composables/useDecodeMapMarkers.js';
import { hasValidPos } from '@/utils/decodeItem.js';
import '@/assets/css/decode-panel.css';

const { items, statusText, connected } = useDecodeService({
  service: 'ais',
  messageType: 'ais',
  field: 'vessels',
  idKey: 'mmsi',
});

const mapRef = ref(null);
const sorted = computed(() =>
  [...items.value].sort((a, b) => String(a.mmsi).localeCompare(String(b.mmsi))),
);

const { selected, withPos, onMapReady, selectItem, bindSync } = useDecodeMapMarkers(
  mapRef,
  items,
  (v) => v.mmsi,
);

function label(v) {
  return v.name || String(v.mmsi);
}

function shipIcon(v, active) {
  const hasHeading = v.cog >= 0 || v.heading >= 0;
  const deg = v.cog >= 0 ? v.cog : v.heading;
  const color = active ? '#ffd54a' : '#5fd0a0';
  const inner = hasHeading
    ? `<path fill="${color}" stroke="#06120d" stroke-width="0.6" transform="rotate(${deg} 12 12)" d="M12 3l5 16-5-3-5 3z"/>`
    : `<circle cx="12" cy="12" r="5" fill="${color}" stroke="#06120d" stroke-width="0.6"/>`;
  return L.divIcon({
    className: 'ship-icon',
    iconSize: [24, 24],
    iconAnchor: [12, 12],
    html: `<svg width="24" height="24" viewBox="0 0 24 24">${inner}</svg>`,
  });
}

function popupHtml(v) {
  const rows = [
    ['MMSI', v.mmsi],
    ['呼号', v.callsign || '—'],
    ['航速', v.sog >= 0 ? `${v.sog.toFixed(1)} kn` : '—'],
    ['航向', v.cog >= 0 ? `${Math.round(v.cog)}°` : '—'],
    ['船首向', v.heading >= 0 ? `${v.heading}°` : '—'],
  ];
  return `<div class="ac-popup"><b>${label(v)}</b>${rows.map(([k, val]) => `<div><span>${k}</span>${val}</div>`).join('')}</div>`;
}

const runSync = bindSync(shipIcon, popupHtml, { minZoom: 10 });

function onSelect(v) {
  selectItem(v, 10);
  runSync();
}

function onMapReadyFit(map) {
  onMapReady();
  runSync();
  fitMapToMarkers(map, withPos.value.map((v) => [v.lat, v.lon]), 10);
}

onMounted(() => {
  setTimeout(runSync, 300);
});
</script>

<template>
  <div class="map-page">
    <BaseMap ref="mapRef" :center="[30.5, 122.2]" :zoom="7" @ready="onMapReadyFit" />
    <DecodeNotifyList
      :items="sorted"
      item-key="mmsi"
      :selected-id="selected"
      :status-text="statusText"
      :connected="connected"
      :is-dim="(v) => !hasValidPos(v)"
      @select="onSelect"
    >
      <template #item="{ item: v }">
        <div class="notify-main">
          <span class="notify-title">{{ label(v) }}</span>
          <span class="notify-badge">{{ v.sog >= 0 ? `${v.sog.toFixed(1)} kn` : '—' }}</span>
        </div>
        <div class="notify-sub">
          <span v-if="!hasValidPos(v)" class="notify-pending">暂无位置数据</span>
          <span>{{ v.mmsi }}</span>
          <span v-if="v.cog >= 0">{{ Math.round(v.cog) }}°</span>
          <span class="notify-time">{{ Math.round(v.seen) }}s</span>
        </div>
      </template>
    </DecodeNotifyList>
  </div>
</template>
