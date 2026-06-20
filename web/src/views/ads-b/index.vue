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
  service: 'adsb',
  messageType: 'adsb',
  field: 'aircraft',
  idKey: 'icao',
  soundInitialWithoutPos: true,
});

const mapRef = ref(null);
const sorted = computed(() =>
  [...items.value].sort((a, b) => String(a.icao).localeCompare(String(b.icao))),
);

const { selected, withPos, onMapReady, selectItem, bindSync } = useDecodeMapMarkers(
  mapRef,
  items,
  (ac) => ac.icao,
);

function label(ac) {
  return ac.callsign || ac.icao;
}

function planeIcon(ac, active) {
  const deg = ac.heading || 0;
  const color = active ? '#ffd54a' : '#7ec8ff';
  return L.divIcon({
    className: 'plane-icon',
    iconSize: [26, 26],
    iconAnchor: [13, 13],
    html: `<svg width="26" height="26" viewBox="0 0 24 24" style="transform:rotate(${deg}deg)">
      <path fill="${color}" stroke="#0b0f17" stroke-width="0.6"
        d="M12 2l1.4 1.4v6.3l7.6 4.4v1.9l-7.6-2.3v4.3l2.1 1.5v1.4L12 21.6l-3.5 1.1v-1.4l2.1-1.5v-4.3L3 17.9V16l7.6-4.4V5.4z"/>
    </svg>`,
  });
}

function popupHtml(ac) {
  const rows = [
    ['呼号', ac.callsign || '—'],
    ['ICAO', ac.icao],
    ['高度', ac.hasAlt ? `${ac.alt} ft` : '—'],
    ['地速', ac.hasVel ? `${Math.round(ac.speed)} kt` : '—'],
    ['航向', ac.hasVel ? `${Math.round(ac.heading)}°` : '—'],
    ['爬升', ac.hasVel ? `${ac.vs} ft/min` : '—'],
  ];
  return `<div class="ac-popup"><b>${label(ac)}</b>${rows.map(([k, v]) => `<div><span>${k}</span>${v}</div>`).join('')}</div>`;
}

const runSync = bindSync(planeIcon, popupHtml, { minZoom: 9 });

function onSelect(ac) {
  selectItem(ac, 9);
  runSync();
}

function onMapReadyFit(map) {
  onMapReady();
  runSync();
  fitMapToMarkers(map, withPos.value.map((a) => [a.lat, a.lon]));
}

onMounted(() => {
  setTimeout(runSync, 300);
});
</script>

<template>
  <div class="map-page">
    <BaseMap ref="mapRef" :center="[31.23, 121.47]" :zoom="7" @ready="onMapReadyFit" />
    <DecodeNotifyList
      :items="sorted"
      item-key="icao"
      :selected-id="selected"
      :status-text="statusText"
      :connected="connected"
      :is-dim="(ac) => !hasValidPos(ac)"
      @select="onSelect"
    >
      <template #item="{ item: ac }">
        <div class="notify-main">
          <span class="notify-title">{{ label(ac) }}</span>
          <span class="notify-badge">{{ ac.hasAlt ? `${ac.alt} ft` : '—' }}</span>
        </div>
        <div class="notify-sub">
          <span v-if="!hasValidPos(ac)" class="notify-pending">暂无位置数据</span>
          <span v-if="ac.hasVel">{{ Math.round(ac.speed) }} kt</span>
          <span v-if="ac.hasVel">{{ Math.round(ac.heading) }}°</span>
          <span class="notify-time">{{ Math.round(ac.seen) }}s</span>
        </div>
      </template>
    </DecodeNotifyList>
  </div>
</template>
