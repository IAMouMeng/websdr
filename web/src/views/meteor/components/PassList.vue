<script setup>
import { fmtTime, fmtDur, PASS_DAYS, PASS_MIN_EL } from '@/utils/satellitePass.js';

defineProps({
  passes: { type: Array, default: () => [] },
  selectedIdx: { type: Number, default: -1 },
  loading: { type: Boolean, default: false },
  isGeo: { type: Boolean, default: false },
  geoStatus: { type: Object, default: null },
});

defineEmits(['select']);
</script>

<template>
  <div class="pass-list">
    <div class="pass-head">
      <template v-if="isGeo">地球同步 · 实时仰角</template>
      <template v-else>未来 {{ PASS_DAYS }} 天过境 · 仰角 ≥{{ PASS_MIN_EL }}°</template>
    </div>
    <div v-if="loading" class="pass-empty">加载 TLE…</div>
    <div v-else-if="isGeo && geoStatus" class="geo-card">
      <div class="geo-row">
        <span class="geo-el" :class="{ ok: geoStatus.visible }">{{ geoStatus.elevation.toFixed(1) }}°</span>
        <span class="geo-label">{{ geoStatus.visible ? '可见' : '低于地平线' }}</span>
      </div>
      <div class="geo-sub">
        方位 {{ geoStatus.azimuth.toFixed(0) }}° · 距离 {{ (geoStatus.rangeKm).toFixed(0) }} km
      </div>
      <p class="geo-note">GEO 卫星无 LEO 式过境，仰角随观测站纬度变化。</p>
    </div>
    <div v-else-if="!passes.length" class="pass-empty">未来 {{ PASS_DAYS }} 天暂无过境</div>
    <ul v-else class="pass-items">
      <li
        v-for="(p, i) in passes"
        :key="i"
        class="pass-item"
        :class="{ active: i === selectedIdx }"
        @click="$emit('select', i)"
      >
        <div class="pass-row">
          <span class="pass-max">{{ p.maxEl.toFixed(0) }}°</span>
          <span class="pass-when">{{ fmtTime(p.aos) }}</span>
        </div>
        <div class="pass-sub">
          持续 {{ fmtDur(p.aos, p.los) }} · 峰值 {{ fmtTime(p.maxElTime) }}
        </div>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.pass-list {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  border: 1px solid #1a1a1a;
  border-radius: 6px;
  background: #0a0a0a;
  overflow: hidden;
}

.pass-head {
  flex-shrink: 0;
  padding: 8px 10px;
  font-size: 11px;
  color: #666;
  border-bottom: 1px solid #141414;
}

.pass-empty {
  padding: 16px;
  font-size: 12px;
  color: #555;
  text-align: center;
}

.geo-card {
  padding: 12px 10px;
}

.geo-row {
  display: flex;
  align-items: baseline;
  gap: 8px;
}

.geo-el {
  font-family: "SF Mono", Menlo, monospace;
  font-size: 18px;
  color: #888;
}

.geo-el.ok { color: #9cf; }

.geo-label {
  font-size: 12px;
  color: #aaa;
}

.geo-sub {
  margin-top: 4px;
  font-size: 11px;
  color: #666;
}

.geo-note {
  margin-top: 8px;
  font-size: 10px;
  color: #555;
  line-height: 1.4;
}

.pass-items {
  list-style: none;
  overflow-y: auto;
  flex: 1;
}

.pass-item {
  padding: 8px 10px;
  border-bottom: 1px solid #111;
  cursor: pointer;
}

.pass-item:hover {
  background: rgba(255, 255, 255, 0.03);
}

.pass-item.active {
  background: rgba(102, 204, 255, 0.08);
}

.pass-row {
  display: flex;
  align-items: baseline;
  gap: 8px;
}

.pass-max {
  font-family: "SF Mono", Menlo, monospace;
  font-size: 13px;
  color: #9cf;
  min-width: 32px;
}

.pass-when {
  font-size: 12px;
  color: #ccc;
}

.pass-sub {
  margin-top: 2px;
  font-size: 10px;
  color: #555;
}
</style>
