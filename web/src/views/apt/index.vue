<script setup>
import { computed, ref, onMounted, onUnmounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useAPTListen } from './composables/useAPTListen.js';
import AptControls from './components/AptControls.vue';
import SqSlider from '@/views/radio/components/SqSlider.vue';
import { SPEC_MAX_RANGE, SPEC_MIN_RANGE, ZOOM_RANGE, snapVal, fmtZoom } from '@/utils/constants.js';

const route = useRoute();
const router = useRouter();

const freqHz = computed(() => {
  const v = Number(route.query.freqHz || route.query.freq || 0);
  return Number.isFinite(v) && v > 0 ? v : 137_100_000;
});

const waterfallRef = ref(null);
const spectrumRef = ref(null);
const waveformRef = ref(null);

const {
  apt,
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
  setSpecMax,
  setSpecMin,
  setZoom,
} = useAPTListen(freqHz.value);

let cleanupDisplay = null;

onMounted(() => {
  cleanupDisplay = initDisplay(waterfallRef.value, spectrumRef.value, waveformRef.value);
});

onUnmounted(() => {
  cleanupDisplay?.();
});

const elapsedText = computed(() => {
  const s = Math.floor(apt.value.elapsedSec || 0);
  const m = Math.floor(s / 60);
  const r = s % 60;
  return `${m}:${String(r).padStart(2, '0')}`;
});

const metaLine = computed(() => {
  if (!connected.value) return statusText.value;
  const parts = [statusText.value];
  if (apt.value.listening) {
    parts.push(apt.value.freq || '137.100 MHz');
    parts.push(elapsedText.value);
    parts.push(`${apt.value.lines} 行`);
    if (apt.value.strength != null) parts.push(`${apt.value.strength} dBm`);
    if (apt.value.metric) parts.push(apt.value.metric);
  }
  return parts.join(' · ');
});

function goBack() {
  stop();
  router.push('/protocol');
}
</script>

<template>
  <div class="apt-page">
    <header class="apt-head">
      <div>
        <h2>APT 气象云图</h2>
        <p class="apt-meta" :class="{ ok: connected && apt.listening }">{{ metaLine }}</p>
      </div>
      <p class="apt-nav">
        <a href="#" class="apt-text-link" @click.prevent="goBack">返回扫频并停止</a>
      </p>
    </header>

    <div class="apt-main">
      <section class="apt-left">
        <div v-if="!connected" class="apt-empty">正在连接接收机…</div>
        <template v-else>
          <div v-if="apt.image" class="apt-image-wrap">
            <p class="apt-image-hint">可见光 · 红外</p>
            <img :src="apt.image" class="apt-image" alt="APT 云图">
          </div>
          <div v-else class="apt-wait">
            <p v-if="apt.lines >= 2">已收到 {{ apt.lines }} 行，正在合成图像…</p>
            <p v-else>正在累积信号，云图将自动出现</p>
            <p class="apt-wait-sub">完整帧约 128 行 / 60 秒 · 请保持天线指向卫星过境方向</p>
          </div>
        </template>
      </section>

      <section class="apt-right">
        <AptControls
          :gain="gain"
          :agc="agc"
          :filter-b-w="displayState.filterBW"
          @update:gain="setGain"
          @update:agc="setAgc"
          @update:filter-b-w="setFilterBW"
        />
        <div class="display-body apt-display-body">
          <div class="displays">
            <div class="display-spectrum">
              <canvas ref="spectrumRef" class="spectrum-canvas" />
            </div>
            <div class="display-waterfall">
              <canvas ref="waterfallRef" class="waterfall-canvas" />
            </div>
            <div class="display-waveform">
              <span class="apt-wave-label">音频波形</span>
              <canvas ref="waveformRef" class="waveform-canvas" />
            </div>
          </div>
          <div class="right-panel">
            <SqSlider
              label="Max"
              :model-value="displayState.specMax"
              :range="SPEC_MAX_RANGE"
              @update:model-value="(v) => setSpecMax(snapVal(v, SPEC_MAX_RANGE))"
            />
            <SqSlider
              label="Min"
              :model-value="displayState.specMin"
              :range="SPEC_MIN_RANGE"
              @update:model-value="(v) => setSpecMin(snapVal(v, SPEC_MIN_RANGE))"
            />
            <SqSlider
              label="Zoom"
              :model-value="displayState.zoom"
              :range="ZOOM_RANGE"
              :formatter="fmtZoom"
              @update:model-value="(v) => setZoom(snapVal(v, ZOOM_RANGE))"
            />
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.apt-page {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: #000;
  overflow: hidden;
}

.apt-head {
  flex-shrink: 0;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 20px 10px;
}

.apt-head h2 {
  font-size: 15px;
  font-weight: 600;
  color: #eee;
  margin-bottom: 4px;
}

.apt-meta {
  font-size: 12px;
  color: #666;
}

.apt-meta.ok {
  color: #6a9f6a;
}

.apt-nav {
  flex-shrink: 0;
  padding-top: 2px;
}

.apt-text-link {
  font-size: 12px;
  color: #666;
  text-decoration: none;
}

.apt-text-link:hover {
  color: #9cf;
}

.apt-main {
  flex: 1;
  min-height: 0;
  display: flex;
  border-top: 1px solid #141414;
}

.apt-left {
  flex: 1;
  min-width: 0;
  overflow-y: auto;
  padding: 14px 16px 20px;
  border-right: 1px solid #141414;
}

.apt-right {
  width: min(48vw, 560px);
  min-width: 320px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.apt-display-body {
  flex: 1;
  min-height: 0;
}

.apt-display-body .displays {
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

.apt-wave-label {
  position: absolute;
  left: 8px;
  top: 4px;
  font-size: 10px;
  color: #444;
  pointer-events: none;
  z-index: 1;
}

.apt-empty,
.apt-wait {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 240px;
  color: #555;
  font-size: 13px;
}

.apt-wait-sub {
  font-size: 11px;
  color: #444;
  text-align: center;
}

.apt-image-wrap {
  max-width: 100%;
}

.apt-image-hint {
  font-size: 10px;
  color: #6a806a;
  margin-bottom: 4px;
}

.apt-image {
  display: block;
  width: 100%;
  max-width: 720px;
  height: auto;
  border-radius: 4px;
  background: #0a0a0a;
}
</style>
