<script setup>
import { ref, watch, nextTick, onMounted, onUnmounted } from 'vue';
import { useProtocolScan } from '../composables/useProtocolScan.js';

const {
  scanBand,
  scanProgress,
  bindFullScanDisplay,
  showScanPanel,
} = useProtocolScan();

const waterfallRef = ref(null);
const spectrumRef = ref(null);
const waveformRef = ref(null);
let cleanup = null;

function mountDisplay() {
  if (!waterfallRef.value || !spectrumRef.value || !waveformRef.value) return;
  cleanup?.();
  cleanup = bindFullScanDisplay(waterfallRef.value, spectrumRef.value, waveformRef.value);
}

watch(showScanPanel, async (show) => {
  if (!show) return;
  await nextTick();
  mountDisplay();
});

onMounted(async () => {
  await nextTick();
  mountDisplay();
});

onUnmounted(() => {
  cleanup?.();
});
</script>

<template>
  <aside v-if="showScanPanel" class="scan-display">
    <div class="scan-display-head">
      <div>
        <h3>全频扫描</h3>
        <p class="scan-display-meta">
          <span v-if="scanProgress">
            {{ scanProgress.phase === 'analyzing' ? '分析中…' : '当前段频谱 · 连续瀑布（换段不断层清零）' }}
            · {{ Math.round(scanProgress.pct || 0) }}%
            <template v-if="scanBand"> · {{ scanBand }}</template>
          </span>
        </p>
      </div>
      <div v-if="scanProgress" class="progress-wrap">
        <div class="progress-bar">
          <div class="progress-fill" :style="{ width: `${Math.min(100, scanProgress.pct || 0)}%` }" />
        </div>
        <span class="progress-text">
          {{ (scanProgress.bandIdx ?? 0) + 1 }} / {{ scanProgress.bandTotal || '—' }}
        </span>
      </div>
    </div>

    <div class="display-stack">
      <div class="spectrum-wrap">
        <canvas ref="spectrumRef" class="spectrum-canvas" />
      </div>
      <div class="waterfall-wrap">
        <canvas ref="waterfallRef" class="waterfall-canvas" />
      </div>
      <div class="waveform-wrap">
        <span class="wave-label">音频波形</span>
        <canvas ref="waveformRef" class="waveform-canvas" />
      </div>
    </div>
  </aside>
</template>

<style scoped>
.scan-display {
  width: min(44vw, 480px);
  min-width: 300px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  background: #050505;
  min-height: 0;
}

.scan-display-head {
  flex-shrink: 0;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 14px 16px 10px;
  border-bottom: 1px solid #141414;
}

.scan-display-head h3 {
  font-size: 13px;
  font-weight: 600;
  color: #ddd;
  margin-bottom: 2px;
}

.scan-display-meta {
  font-size: 11px;
  color: #666;
  line-height: 1.45;
}

.progress-wrap {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 4px;
  min-width: 100px;
}

.progress-bar {
  width: 100%;
  height: 4px;
  background: #1a1a1a;
  border-radius: 2px;
  overflow: hidden;
}

.progress-fill {
  height: 100%;
  background: linear-gradient(90deg, #3a7, #6c6);
  transition: width 0.15s linear;
}

.progress-text {
  font-size: 10px;
  color: #666;
  font-family: "SF Mono", Menlo, monospace;
  white-space: nowrap;
}

.display-stack {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 10px 12px 14px;
}

.spectrum-wrap {
  flex: 0 0 88px;
  position: relative;
  border: 1px solid #1a1a1a;
  border-radius: 4px;
  background: #000;
}

.spectrum-canvas {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
}

.waterfall-wrap {
  flex: 1;
  min-height: 0;
  position: relative;
  border: 1px solid #1a1a1a;
  border-radius: 4px;
  background: #000;
}

.waterfall-canvas {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
}

.waveform-wrap {
  flex: 0 0 72px;
  position: relative;
  border: 1px solid #1a1a1a;
  border-radius: 4px;
  background: #000;
}

.wave-label {
  position: absolute;
  top: 4px;
  left: 8px;
  font-size: 10px;
  color: #555;
  z-index: 1;
  pointer-events: none;
}

.waveform-canvas {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
}
</style>
