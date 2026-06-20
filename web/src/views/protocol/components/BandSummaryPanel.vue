<script setup>
import { ref } from 'vue';
import { useProtocolScan } from '../composables/useProtocolScan.js';
import { TYPE_LABELS } from '../constants.js';

const { bandSummaries, fullScanPhase } = useProtocolScan();
const openBands = ref(new Set());

function toggleBand(name) {
  const next = new Set(openBands.value);
  if (next.has(name)) next.delete(name);
  else next.add(name);
  openBands.value = next;
}

function typeLabel(t) {
  return TYPE_LABELS[t]?.split(' ')[0] || t;
}
</script>

<template>
  <section v-if="fullScanPhase === 'done' && bandSummaries.length" class="band-summary">
    <header class="band-summary-head">
      <h3>全频扫描报告</h3>
      <p>按 50 MHz 分段汇总；0–24 MHz 超出 RTL-SDR 直扫范围</p>
    </header>

    <div class="band-list">
      <div
        v-for="band in bandSummaries.filter((b) => b.name !== '扫描说明')"
        :key="band.name"
        class="band-card"
      >
        <button type="button" class="band-card-head" @click="toggleBand(band.name)">
          <span class="expand" :class="{ open: openBands.has(band.name) }">›</span>
          <span class="band-name">{{ band.name }}</span>
          <span class="band-freq">{{ band.centerMHz }} MHz</span>
          <span class="band-count" :class="{ empty: band.signalCount === 0 }">
            {{ band.signalCount }} 信号
          </span>
        </button>
        <p class="band-summary-text">{{ band.summary }}</p>
        <div v-if="openBands.has(band.name) && band.signals?.length" class="band-signals">
          <div v-for="sig in band.signals" :key="sig.id" class="band-sig-row">
            <span class="sig-label">{{ sig.label }}</span>
            <span class="sig-type">{{ typeLabel(sig.type) }}</span>
            <span class="sig-strength">{{ sig.strength }} dBm</span>
            <span v-if="sig.decoded" class="sig-decoded">已解调</span>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.band-summary {
  border-bottom: 1px solid #1a1a1a;
  padding: 14px 16px;
  background: #080808;
}

.band-summary-head h3 {
  font-size: 13px;
  font-weight: 600;
  color: #ddd;
  margin-bottom: 4px;
}

.band-summary-head p {
  font-size: 11px;
  color: #555;
  margin-bottom: 12px;
}

.band-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 320px;
  overflow-y: auto;
}

.band-card {
  border: 1px solid #1a1a1a;
  border-radius: 4px;
  background: #0a0a0a;
  overflow: hidden;
}

.band-card-head {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 10px 12px;
  border: none;
  background: transparent;
  color: inherit;
  cursor: pointer;
  text-align: left;
}

.band-card-head:hover {
  background: rgba(255, 255, 255, 0.02);
}

.expand {
  color: #555;
  font-size: 14px;
  transition: transform 0.12s;
}

.expand.open {
  transform: rotate(90deg);
  color: #9cf;
}

.band-name {
  font-size: 12px;
  font-weight: 600;
  color: #ccc;
  flex: 1;
}

.band-freq {
  font-size: 10px;
  color: #666;
  font-family: "SF Mono", Menlo, monospace;
}

.band-count {
  font-size: 10px;
  color: #6a6;
  padding: 2px 6px;
  border-radius: 3px;
  background: rgba(74, 120, 74, 0.2);
}

.band-count.empty {
  color: #666;
  background: #151515;
}

.band-summary-text {
  padding: 0 12px 10px 36px;
  font-size: 11px;
  color: #777;
  line-height: 1.45;
}

.band-signals {
  border-top: 1px solid #141414;
  padding: 6px 12px 8px 36px;
}

.band-sig-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 5px 0;
  font-size: 11px;
  border-bottom: 1px solid #111;
}

.band-sig-row:last-child {
  border-bottom: none;
}

.sig-label {
  flex: 1;
  color: #bbb;
}

.sig-type {
  color: #666;
}

.sig-strength {
  font-family: "SF Mono", Menlo, monospace;
  color: #888;
}

.sig-decoded {
  color: #8ecf8e;
  font-size: 10px;
}
</style>
