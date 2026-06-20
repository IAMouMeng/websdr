<script setup>
import { computed, ref, onMounted, onUnmounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useLRPTListen } from './composables/useLRPTListen.js';
import SqSlider from '@/views/radio/components/SqSlider.vue';
import { SPEC_MAX_RANGE, SPEC_MIN_RANGE, ZOOM_RANGE, snapVal, fmtZoom } from '@/utils/constants.js';

const route = useRoute();
const router = useRouter();

const freqHz = computed(() => {
  const v = Number(route.query.freqHz || route.query.freq || 0);
  return Number.isFinite(v) && v > 0 ? v : 137_900_000;
});

const waterfallRef = ref(null);
const spectrumRef = ref(null);
const waveformRef = ref(null);

const {
  lrpt,
  statusText,
  connected,
  gain,
  agc,
  displayState,
  stop,
  initDisplay,
  setGain,
  setAgc,
  setFilterBW,
} = useLRPTListen(freqHz.value);

let cleanupDisplay = null;

onMounted(() => {
  cleanupDisplay = initDisplay(waterfallRef.value, spectrumRef.value, waveformRef.value);
});

onUnmounted(() => {
  cleanupDisplay?.();
});

const metaLine = computed(() => {
  if (!connected.value) return statusText.value;
  const parts = [statusText.value, lrpt.value.freq || '137.900 MHz'];
  if (lrpt.value.locked) parts.push('已锁定');
  else parts.push('搜锁中');
  if (lrpt.value.metric) parts.push(lrpt.value.metric);
  if (lrpt.value.strength != null) parts.push(`${lrpt.value.strength} dBm`);
  return parts.join(' · ');
});

function goBack() {
  stop();
  router.push('/protocol');
}
</script>

<template>
  <div class="lrpt-page">
    <header class="lrpt-head">
      <div>
        <h2>LRPT 数字下行</h2>
        <p class="lrpt-meta" :class="{ ok: connected && lrpt.listening && lrpt.locked }">{{ metaLine }}</p>
        <p class="lrpt-hint">Meteor-M 等 OQPSK 链路 · 完整云图解码后续支持</p>
      </div>
      <p class="lrpt-nav">
        <a href="#" class="lrpt-text-link" @click.prevent="goBack">返回扫频并停止</a>
      </p>
    </header>

    <div class="lrpt-rf">
      <label class="lrpt-rf-item">
        <span>增益</span>
        <input type="range" min="0" max="50" :value="gain" @input="setGain(Number($event.target.value))">
        <span class="lrpt-rf-v">{{ gain }} dB</span>
      </label>
      <label class="lrpt-rf-item">
        <input type="checkbox" :checked="agc" @change="setAgc($event.target.checked)">
        <span>AGC</span>
      </label>
      <label class="lrpt-rf-item">
        <span>带宽</span>
        <input
          type="range"
          min="80000"
          max="250000"
          step="5000"
          :value="displayState.filterBW"
          @input="setFilterBW(Number($event.target.value))"
        >
        <span class="lrpt-rf-v">{{ (displayState.filterBW / 1000).toFixed(0) }} kHz</span>
      </label>
    </div>

    <div class="display-body lrpt-display-body">
      <div class="displays">
        <div class="display-spectrum">
          <canvas ref="spectrumRef" class="spectrum-canvas" />
        </div>
        <div class="display-waterfall">
          <canvas ref="waterfallRef" class="waterfall-canvas" />
        </div>
        <div class="display-waveform">
          <span class="lrpt-wave-label">USB 监听</span>
          <canvas ref="waveformRef" class="waveform-canvas" />
        </div>
      </div>
      <div class="right-panel">
        <SqSlider
          label="Max"
          :model-value="displayState.specMax"
          :range="SPEC_MAX_RANGE"
          @update:model-value="(v) => { displayState.specMax = snapVal(v, SPEC_MAX_RANGE); }"
        />
        <SqSlider
          label="Min"
          :model-value="displayState.specMin"
          :range="SPEC_MIN_RANGE"
          @update:model-value="(v) => { displayState.specMin = snapVal(v, SPEC_MIN_RANGE); }"
        />
        <SqSlider
          label="Zoom"
          :model-value="displayState.zoom"
          :range="ZOOM_RANGE"
          :formatter="fmtZoom"
          @update:model-value="(v) => { displayState.zoom = snapVal(v, ZOOM_RANGE); }"
        />
      </div>
    </div>

    <div v-if="!connected" class="lrpt-empty">正在连接接收机…</div>
    <div v-else-if="!lrpt.locked" class="lrpt-status">等待 LRPT 数字信号… 请对准 Meteor 过境方向</div>
  </div>
</template>

<style scoped>
.lrpt-page {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: #000;
  overflow: hidden;
}

.lrpt-head {
  flex-shrink: 0;
  display: flex;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 20px 8px;
}

.lrpt-head h2 {
  font-size: 15px;
  font-weight: 600;
  color: #eee;
  margin-bottom: 4px;
}

.lrpt-meta {
  font-size: 12px;
  color: #666;
}

.lrpt-meta.ok {
  color: #6a9f6a;
}

.lrpt-hint {
  font-size: 11px;
  color: #444;
  margin-top: 4px;
}

.lrpt-nav {
  padding-top: 2px;
}

.lrpt-text-link {
  font-size: 12px;
  color: #666;
  text-decoration: none;
}

.lrpt-text-link:hover {
  color: #9cf;
}

.lrpt-rf {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 4px 12px;
  border-bottom: 1px solid #141414;
  background: #050505;
}

.lrpt-rf-item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: #666;
}

.lrpt-rf-item input[type='range'] {
  width: 72px;
  accent-color: #69a;
}

.lrpt-rf-v {
  font-family: "SF Mono", "Menlo", monospace;
  font-size: 10px;
  color: #777;
  min-width: 44px;
}

.lrpt-display-body {
  flex: 1;
  min-height: 0;
}

.display-waveform {
  flex: 0 0 64px;
  min-height: 48px;
  position: relative;
  border-top: 1px solid #141414;
}

.waveform-canvas {
  display: block;
  width: 100%;
  height: 100%;
}

.lrpt-wave-label {
  position: absolute;
  left: 8px;
  top: 4px;
  font-size: 10px;
  color: #444;
  pointer-events: none;
}

.lrpt-empty,
.lrpt-status {
  flex-shrink: 0;
  padding: 10px 20px 16px;
  font-size: 12px;
  color: #555;
  text-align: center;
}
</style>
