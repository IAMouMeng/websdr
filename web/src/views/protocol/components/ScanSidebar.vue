<script setup>
import { useProtocolScan } from '../composables/useProtocolScan.js';
import { TYPE_LABELS } from '../constants.js';

const {
  listening,
  fullScanning,
  fullScanPhase,
  connected,
  statusText,
  scanBand,
  scanProgress,
  filters,
  toggleListen,
  startFullScan,
  stopFullScan,
} = useProtocolScan();

const scanBusy = () => listening.value || fullScanning.value;

function onFullScanClick() {
  if (fullScanning.value) stopFullScan();
  else startFullScan();
}
</script>

<template>
  <aside class="sidebar">
    <div class="sidebar-head">
      <h1>协议分析</h1>
      <div class="status" :class="{ ok: scanBusy() && connected }">
        <template v-if="fullScanning">
          {{ connected ? '全频扫描中…' : statusText }}
          <span v-if="connected && scanProgress" class="scan-band">
            · {{ Math.round(scanProgress.pct || 0) }}%
          </span>
        </template>
        <template v-else-if="listening">
          {{ connected ? '监听中…' : statusText }}
          <span v-if="connected && scanBand" class="scan-band"> · {{ scanBand }}</span>
        </template>
        <template v-else-if="fullScanPhase === 'done'">全频扫描已完成</template>
        <template v-else>已停止</template>
      </div>
    </div>

    <div class="section">
      <div class="section-title">扫描</div>
      <button
        class="fullscan-btn"
        :class="{ active: fullScanning }"
        :disabled="listening"
        @click="onFullScanClick"
      >
        {{ fullScanning ? '停止全频扫描' : '一键扫全频' }}
      </button>

      <button
        class="listen-btn"
        :class="{ active: listening }"
        :disabled="fullScanning"
        @click="toggleListen"
      >
        {{ listening ? '停止监听' : '持续监听' }}
      </button>
    </div>

    <div class="section section-filters">
      <div class="section-title">显示筛选</div>
      <label v-for="(label, key) in TYPE_LABELS" :key="key" class="check-row">
        <input v-model="filters[key]" type="checkbox">
        {{ label }}
      </label>
    </div>
  </aside>
</template>

<style scoped>
.sidebar-head {
  padding: 16px 16px 12px;
}

.sidebar-head h1 {
  font-size: 15px;
  font-weight: 600;
  color: #fff;
  letter-spacing: 0.05em;
}

.status {
  font-size: 11px;
  color: #555;
  margin-top: 4px;
}

.status.ok {
  color: #4a4;
}

.scan-band {
  color: #666;
}

.fullscan-btn {
  width: 100%;
  padding: 9px 0;
  margin-bottom: 8px;
  border: none;
  border-radius: 3px;
  background: #2d5a3d;
  color: #e8ffe8;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}

.fullscan-btn:hover:not(:disabled) {
  background: #3a7049;
}

.fullscan-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.fullscan-btn.active {
  background: #1a2e22;
  color: #9c9;
  box-shadow: inset 0 0 0 1px #3a5;
}

.listen-btn {
  width: 100%;
  padding: 9px 0;
  border: none;
  border-radius: 3px;
  background: #fff;
  color: #000;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}

.listen-btn:hover {
  background: #ddd;
}

.listen-btn.active {
  background: #2a2a2a;
  color: #ccc;
  box-shadow: inset 0 0 0 1px #444;
}

.listen-btn.active:hover {
  background: #333;
  color: #eee;
}

.listen-hint {
  margin-top: 6px;
}

.listen-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.scan-hint {
  margin-top: 8px;
  font-size: 11px;
  color: #555;
  line-height: 1.5;
}

.section-filters {
  max-height: calc(100vh - 220px);
  overflow-y: auto;
  scrollbar-width: thin;
  scrollbar-color: #333 transparent;
}

.check-row {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: #888;
  cursor: pointer;
  margin-bottom: 6px;
}

.check-row:last-child {
  margin-bottom: 0;
}
</style>
