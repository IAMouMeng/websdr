<script setup>
import { computed, ref, watch, onMounted, onUnmounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useMeteorListen } from './composables/useMeteorListen.js';
import { useSatelliteRecord } from './composables/useSatelliteRecord.js';
import MeteorDecoder from './components/MeteorDecoder.vue';
import PassList from './components/PassList.vue';
import PassElevationChart from './components/PassElevationChart.vue';
import SatelliteHeader from './components/SatelliteHeader.vue';
import RecordPanel from './components/RecordPanel.vue';
import SqSlider from '@/views/radio/components/SqSlider.vue';
import { SPEC_MAX_RANGE, SPEC_MIN_RANGE, ZOOM_RANGE, snapVal, fmtZoom } from '@/utils/constants.js';
import { fetchTLE, fetchCatalog } from '@/api/satellite.js';
import {
  loadObserver,
  saveObserver,
  observerGeodetic,
  findPasses,
  passElevationCurve,
  geoElevationCurve,
  geoStatus,
  satrecFromTLE,
  dopplerHz,
  geometryNow,
  PASS_DAYS,
  fmtFreqMHz,
  isGeoSatellite,
  fmtClockUTC,
  fmtClockBeijing,
  fmtCountdown,
  fmtTime,
  currentPass,
  nextPass,
  passLeg,
  passLegLabel,
} from '@/utils/satellitePass.js';

const route = useRoute();
const router = useRouter();

const freqHz = computed(() => {
  const v = Number(route.query.freqHz || route.query.freq || 0);
  return Number.isFinite(v) && v > 0 ? v : 137_900_000;
});
const noradQuery = computed(() => {
  const v = Number(route.query.norad || 0);
  return Number.isFinite(v) && v > 0 ? v : 40069;
});

const observer = ref(loadObserver());
const catalog = ref([]);
const tleMap = ref({});
const passes = ref([]);
const selectedPassIdx = ref(0);
const tleLoading = ref(true);
const autoDoppler = ref(true);
const satrec = ref(null);
const selectedChannel = ref(0);
const geoLoading = ref(false);
const geoError = ref('');
const nowMs = ref(Date.now());
let clockTimer = null;

const iqBridge = { push: () => {} };

const {
  meteor,
  statusText,
  connected,
  gain,
  agc,
  displayState,
  initDisplay,
  setGain,
  setAgc,
  setFilterBW,
  startListen,
  resetDisplay,
  sendCmd,
  setDeviceSampleRate,
  onCanvasDown,
  onCanvasMove,
  onCanvasUp,
  onCanvasWheel,
  onCanvasHover,
} = useMeteorListen({
  freqHz: freqHz.value,
  norad: noradQuery.value,
  autoDoppler: autoDoppler.value,
  onTrackTick,
  onIQ: (frame) => iqBridge.push(frame),
});

const {
  RATE_OPTIONS: recRateOptions,
  sampleRate: recSampleRate,
  channels: recChannels,
  state: recState,
  elapsedStr: recElapsedStr,
  sizeStr: recSizeStr,
  canDownload: recCanDownload,
  autoPassRecord: recAutoPass,
  preRollSec: recPreRoll,
  postRollSec: recPostRoll,
  autoPassStatus: recAutoPassStatus,
  pushIQ: recPushIQ,
  start: recStart,
  pause: recPause,
  resume: recResume,
  stop: recStop,
  download: recDownload,
  resetAutoPassState,
} = useSatelliteRecord({
  sendCmd,
  setSampleRate: setDeviceSampleRate,
  getAutoPassCtx: () => ({
    passes: passes.value,
    isGeo: isGeo.value,
    nowMs: nowMs.value,
    satName: selectedSat.value?.name || 'sat',
  }),
});
iqBridge.push = recPushIQ;

const waterfallRef = ref(null);
const spectrumRef = ref(null);
const waveformRef = ref(null);

const selectedSat = computed(() =>
  catalog.value.find((s) => s.norad === noradQuery.value) || catalog.value[0] || null,
);
const isGeo = computed(() => isGeoSatellite(selectedSat.value, noradQuery.value));

function catalogEntry(norad) {
  return catalog.value.find((s) => s.norad === norad);
}
const geoInfo = computed(() => {
  if (!isGeo.value || !satrec.value) return null;
  return geoStatus(satrec.value, observerGeodetic(observer.value));
});

function onTrackTick(sendTrack) {
  if (!satrec.value) return;
  const obs = observerGeodetic(observer.value);
  const entry = selectedSat.value;
  const nominal = entry?.downlinkHz || freqHz.value;
  const geo = geometryNow(satrec.value, obs);
  const dop = autoDoppler.value && !isGeo.value ? dopplerHz(satrec.value, obs, nominal) : 0;
  sendTrack({ dopplerHz: dop, elevation: geo.elevation, azimuth: geo.azimuth });
}

const utcStr = computed(() => fmtClockUTC(new Date(nowMs.value)));
const beijingStr = computed(() => fmtClockBeijing(new Date(nowMs.value)));

const currentFreqStr = computed(() => {
  if (meteor.value.freq) return meteor.value.freq;
  return fmtFreqMHz(meteor.value.freqHz || displayState.tuneFreq || freqHz.value);
});

const passHighlight = computed(() => {
  if (isGeo.value) {
    if (geoInfo.value?.visible) return 'GEO 可见 · 常驻接收';
    return 'GEO · 低于地平线';
  }
  const cur = currentPass(passes.value, nowMs.value);
  if (cur) {
    const left = cur.los.getTime() - nowMs.value;
    return `正在过境 · 剩余 ${fmtCountdown(left)}`;
  }
  const nxt = nextPass(passes.value, nowMs.value);
  if (nxt) {
    const wait = nxt.aos.getTime() - nowMs.value;
    return `下次过境 ${fmtCountdown(wait)}`;
  }
  return `未来 ${PASS_DAYS} 天无过境`;
});

const passDetail = computed(() => {
  const el = meteor.value.elevation || geoInfo.value?.elevation || 0;
  if (isGeo.value && el > 0) return `仰角 ${el.toFixed(1)}°`;
  const cur = currentPass(passes.value, nowMs.value);
  if (cur && satrec.value) {
    const leg = passLegLabel(passLeg(satrec.value, observerGeodetic(observer.value)));
    return `仰角 ${el.toFixed(1)}° · ${leg}`;
  }
  const nxt = nextPass(passes.value, nowMs.value);
  if (nxt) return `峰值 ${nxt.maxEl.toFixed(0)}° · ${fmtTime(nxt.aos)}`;
  return '';
});

function tleLines(norad) {
  const map = tleMap.value;
  return map[norad] || map[String(norad)];
}

function updateSatrecAndPasses(norad) {
  const lines = tleLines(norad);
  if (!lines) {
    satrec.value = null;
    passes.value = [];
    return;
  }
  satrec.value = satrecFromTLE(lines[0], lines[1]);
  const entry = catalogEntry(norad);
  if (isGeoSatellite(entry, norad)) {
    passes.value = [];
  } else {
    passes.value = findPasses(satrec.value, observerGeodetic(observer.value), PASS_DAYS, 10);
  }
  selectedPassIdx.value = 0;
}

let cleanupDisplay = null;
let tuningBusy = false;

function applySatTuning() {
  const entry = selectedSat.value;
  if (!entry || tuningBusy) return;
  tuningBusy = true;
  try {
    displayState.centerFreq = entry.centerHz || entry.downlinkHz;
    displayState.tuneFreq = entry.downlinkHz;
    displayState.sampleRate = entry.sampleRate || displayState.sampleRate;
    const bw = entry.modulation === 'LRIT' ? 800_000 : 150_000;
    setFilterBW(bw);
    resetDisplay?.();
    if (isGeoSatellite(entry, entry.norad)) autoDoppler.value = false;
    startListen({
      freqHz: entry.downlinkHz,
      norad: entry.norad,
      autoDoppler: autoDoppler.value,
    });
  } finally {
    tuningBusy = false;
  }
}

async function loadPasses() {
  tleLoading.value = true;
  try {
    const [cat, tle] = await Promise.all([fetchCatalog(), fetchTLE()]);
    catalog.value = cat.satellites || [];
    tleMap.value = tle;
    updateSatrecAndPasses(noradQuery.value);
    applySatTuning();
  } catch (e) {
    console.warn('TLE:', e);
  } finally {
    tleLoading.value = false;
  }
}

const elevationCurve = computed(() => {
  if (!satrec.value) return [];
  if (isGeo.value) {
    return geoElevationCurve(satrec.value, observerGeodetic(observer.value));
  }
  const p = passes.value[selectedPassIdx.value];
  if (!p) return [];
  return passElevationCurve(satrec.value, observerGeodetic(observer.value), p.aos, p.los);
});

const inPassNow = computed(() => {
  if (isGeo.value) return geoInfo.value?.visible ?? false;
  const p = passes.value[selectedPassIdx.value];
  if (!p) return false;
  const now = Date.now();
  return now >= p.aos.getTime() && now <= p.los.getTime();
});

const mainImage = computed(() => {
  const ch = meteor.value.channels?.[selectedChannel.value];
  if (ch?.image) return ch.image;
  return meteor.value.image || '';
});

const metaLine = computed(() => {
  if (!connected.value) return statusText.value;
  const entry = selectedSat.value;
  const parts = [statusText.value, meteor.value.satellite || entry?.name || '卫星'];
  parts.push(meteor.value.freq || fmtFreqMHz(entry?.downlinkHz));
  if (meteor.value.autoDoppler && meteor.value.dopplerHz) {
    parts.push(`Δf ${meteor.value.dopplerHz >= 0 ? '+' : ''}${Math.round(meteor.value.dopplerHz)} Hz`);
  }
  if (meteor.value.elevation > 0) parts.push(`仰角 ${meteor.value.elevation.toFixed(1)}°`);
  if (isGeo.value) parts.push('GEO');
  if (meteor.value.synced) parts.push('帧同步');
  else if (meteor.value.locked) parts.push('载波锁定');
  else parts.push('搜锁中');
  if (meteor.value.lines) parts.push(`${meteor.value.lines} 行`);
  if (meteor.value.metric) parts.push(meteor.value.metric);
  return parts.join(' · ');
});

function onObserverChange() {
  resetAutoPassState();
  saveObserver(observer.value);
  loadPasses();
}

function onSatChange(e) {
  const n = Number(e.target.value);
  if (!Number.isFinite(n) || n === noradQuery.value) return;
  const entry = catalogEntry(n);
  const f = entry?.downlinkHz || freqHz.value;
  selectedChannel.value = 0;
  resetDisplay?.();
  router.replace({ query: { ...route.query, norad: String(n), freqHz: String(f) } });
}

function useMyLocation() {
  if (!navigator.geolocation) {
    geoError.value = '浏览器不支持定位';
    return;
  }
  geoLoading.value = true;
  geoError.value = '';
  navigator.geolocation.getCurrentPosition(
    (pos) => {
      observer.value.lat = Math.round(pos.coords.latitude * 10000) / 10000;
      observer.value.lon = Math.round(pos.coords.longitude * 10000) / 10000;
      if (pos.coords.altitude != null && Number.isFinite(pos.coords.altitude)) {
        observer.value.alt = Math.round(pos.coords.altitude);
      }
      geoLoading.value = false;
      onObserverChange();
    },
    (err) => {
      geoLoading.value = false;
      if (err.code === 1) geoError.value = '定位被拒绝，请在浏览器中允许';
      else if (err.code === 3) geoError.value = '定位超时';
      else geoError.value = '定位失败';
    },
    { enableHighAccuracy: true, timeout: 15000, maximumAge: 60000 },
  );
}

watch(noradQuery, (n) => {
  resetAutoPassState();
  updateSatrecAndPasses(n);
  applySatTuning();
});

watch(autoDoppler, (v) => {
  if (tuningBusy) return;
  startListen({ autoDoppler: v });
});

onMounted(() => {
  cleanupDisplay = initDisplay(waterfallRef.value, spectrumRef.value, waveformRef.value);
  clockTimer = setInterval(() => { nowMs.value = Date.now(); }, 1000);
  window.addEventListener('mousemove', onGlobalMove);
  window.addEventListener('mouseup', onCanvasUp);
  window.addEventListener('touchend', onCanvasUp);
  loadPasses();
});

function onCanvasInteract(e, canvas) {
  onCanvasDown(e.clientX, canvas);
}

function onTouchStart(e, canvas) {
  e.preventDefault();
  onCanvasInteract(e, canvas);
}

function onTouchMove(e) {
  if (displayState.dragging) {
    e.preventDefault();
    onCanvasMove(e.touches[0].clientX);
  }
}

function onGlobalMove(e) {
  onCanvasMove(e.clientX);
}

onUnmounted(() => {
  cleanupDisplay?.();
  if (clockTimer) clearInterval(clockTimer);
  window.removeEventListener('mousemove', onGlobalMove);
  window.removeEventListener('mouseup', onCanvasUp);
  window.removeEventListener('touchend', onCanvasUp);
  recStop();
  resetAutoPassState();
});
</script>

<template>
  <div class="meteor-page">
    <SatelliteHeader
      :utc="utcStr"
      :beijing="beijingStr"
      :pass-highlight="passHighlight"
      :pass-detail="passDetail"
      :freq="currentFreqStr"
      link-label="下行"
    />

    <div class="meteor-body">
      <aside class="meteor-side">
        <div class="obs-card">
          <div class="obs-title">观测站</div>
          <label class="obs-row">
            <span>纬度</span>
            <input v-model.number="observer.lat" type="number" step="0.01" @change="onObserverChange">
          </label>
          <label class="obs-row">
            <span>经度</span>
            <input v-model.number="observer.lon" type="number" step="0.01" @change="onObserverChange">
          </label>
          <button
            type="button"
            class="geo-btn"
            :disabled="geoLoading"
            @click="useMyLocation"
          >
            {{ geoLoading ? '定位中…' : '使用我的位置' }}
          </button>
          <p v-if="geoError" class="geo-err">{{ geoError }}</p>
          <label class="obs-row">
            <span>卫星</span>
            <select :value="noradQuery" @change="onSatChange">
              <option v-for="s in catalog" :key="s.norad" :value="s.norad">{{ s.name }}</option>
            </select>
          </label>
          <label class="obs-check">
            <input v-model="autoDoppler" type="checkbox" :disabled="isGeo">
            <span>自动多普勒补偿</span>
          </label>
          <p v-if="isGeo" class="geo-hint">GEO 卫星无需多普勒补偿</p>
        </div>
        <PassList
          :passes="passes"
          :selected-idx="selectedPassIdx"
          :loading="tleLoading"
          :is-geo="isGeo"
          :geo-status="geoInfo"
          @select="(i) => { selectedPassIdx = i; }"
        />
        <PassElevationChart :curve="elevationCurve" :active="inPassNow" />
      </aside>

      <main class="meteor-main">
        <div class="rf-bar">
          <label class="rf-item">
            <span>增益</span>
            <input type="range" min="0" max="50" :value="gain" @input="setGain(Number($event.target.value))">
            <span>{{ gain }} dB</span>
          </label>
          <label class="rf-item">
            <input type="checkbox" :checked="agc" @change="setAgc($event.target.checked)">
            <span>AGC</span>
          </label>
          <label class="rf-item">
            <span>带宽</span>
            <input
              type="range"
              :min="isGeo ? 200000 : 80000"
              :max="isGeo ? 2000000 : 250000"
              step="5000"
              :value="displayState.filterBW"
              @input="setFilterBW(Number($event.target.value))"
            >
            <span>{{ (displayState.filterBW / 1000).toFixed(0) }} kHz</span>
          </label>
          <label class="rf-item tune-hint">
            <span>调频</span>
            <span class="tune-tip">点击频谱/瀑布拖动 · 滚轮微调</span>
            <span v-if="meteor.manualOffsetHz" class="tune-off">
              手动 Δf {{ meteor.manualOffsetHz >= 0 ? '+' : '' }}{{ Math.round(meteor.manualOffsetHz) }} Hz
            </span>
          </label>
        </div>
        <div class="display-body meteor-rf">
          <div class="displays">
            <div class="display-spectrum">
              <canvas
                ref="spectrumRef"
                class="spectrum-canvas"
                @mousedown="(e) => onCanvasInteract(e, e.currentTarget)"
                @touchstart="(e) => onTouchStart(e, e.currentTarget)"
                @mousemove="(e) => onCanvasHover(e.clientX, e.currentTarget)"
                @touchmove="onTouchMove"
                @wheel="onCanvasWheel"
              />
            </div>
            <div class="display-waterfall">
              <canvas
                ref="waterfallRef"
                class="waterfall-canvas"
                @mousedown="(e) => onCanvasInteract(e, e.currentTarget)"
                @touchstart="(e) => onTouchStart(e, e.currentTarget)"
                @mousemove="(e) => onCanvasHover(e.clientX, e.currentTarget)"
                @touchmove="onTouchMove"
                @wheel="onCanvasWheel"
              />
            </div>
            <div class="display-waveform">
              <span class="wave-label">USB 波形</span>
              <canvas ref="waveformRef" class="waveform-canvas" @wheel="onCanvasWheel" />
            </div>
          </div>
          <div class="right-panel">
            <SqSlider label="Max" :model-value="displayState.specMax" :range="SPEC_MAX_RANGE" @update:model-value="(v) => { displayState.specMax = snapVal(v, SPEC_MAX_RANGE); }" />
            <SqSlider label="Min" :model-value="displayState.specMin" :range="SPEC_MIN_RANGE" @update:model-value="(v) => { displayState.specMin = snapVal(v, SPEC_MIN_RANGE); }" />
            <SqSlider label="Zoom" :model-value="displayState.zoom" :range="ZOOM_RANGE" :formatter="fmtZoom" @update:model-value="(v) => { displayState.zoom = snapVal(v, ZOOM_RANGE); }" />
          </div>
        </div>
        <MeteorDecoder
          :connected="connected"
          :meteor="meteor"
          :main-image="mainImage"
          :selected-channel="selectedChannel"
          @select-channel="(i) => { selectedChannel = i; }"
        />
      </main>

      <RecordPanel
        :sample-rate="recSampleRate"
        :channels="recChannels"
        :state="recState"
        :elapsed-str="recElapsedStr"
        :size-str="recSizeStr"
        :can-download="recCanDownload"
        :rate-options="recRateOptions"
        :auto-pass="recAutoPass"
        :pre-roll-sec="recPreRoll"
        :post-roll-sec="recPostRoll"
        :auto-pass-status="recAutoPassStatus"
        :is-geo="isGeo"
        @update:sample-rate="(v) => { recSampleRate = v; }"
        @update:channels="(v) => { recChannels = v; }"
        @update:auto-pass="(v) => { recAutoPass = v; }"
        @update:pre-roll-sec="(v) => { recPreRoll = v; }"
        @update:post-roll-sec="(v) => { recPostRoll = v; }"
        @start="recStart"
        @pause="recPause"
        @resume="recResume"
        @stop="recStop"
        @download="() => recDownload(selectedSat?.name || 'sat')"
      />
    </div>
  </div>
</template>

<style scoped>
.meteor-page {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: #000;
  overflow: hidden;
}

.meteor-head {
  flex-shrink: 0;
  padding: 10px 14px 6px;
}

.meteor-head h2 {
  font-size: 15px;
  font-weight: 600;
  color: #eee;
}

.meteor-meta {
  font-size: 12px;
  color: #666;
  margin-top: 4px;
}

.meteor-meta.ok { color: #6a9f6a; }

.meteor-body {
  flex: 1;
  min-height: 0;
  display: flex;
  gap: 8px;
  padding: 0 8px 8px;
}

.meteor-side {
  width: 220px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-height: 0;
}

.obs-card {
  flex-shrink: 0;
  padding: 8px;
  border: 1px solid #1a1a1a;
  border-radius: 6px;
  background: #0a0a0a;
}

.obs-title { font-size: 11px; color: #666; margin-bottom: 6px; }

.obs-row {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: #888;
  margin-bottom: 4px;
}

.obs-row input,
.obs-row select {
  flex: 1;
  background: #111;
  border: 1px solid #222;
  color: #ccc;
  border-radius: 4px;
  padding: 2px 6px;
  font-size: 11px;
}

.geo-btn {
  width: 100%;
  margin: 4px 0 6px;
  padding: 6px 0;
  border: 1px solid #2a3a2a;
  border-radius: 4px;
  background: #111a11;
  color: #9c9;
  font-size: 11px;
  cursor: pointer;
}

.geo-btn:hover:not(:disabled) {
  background: #152015;
  border-color: #3a5a3a;
}

.geo-btn:disabled {
  opacity: 0.5;
  cursor: default;
}

.geo-err {
  font-size: 10px;
  color: #a66;
  margin: -2px 0 6px;
  line-height: 1.4;
}

.obs-check {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: #888;
  margin-top: 4px;
}

.geo-hint {
  font-size: 10px;
  color: #555;
  margin-top: 4px;
  line-height: 1.3;
}

.meteor-main {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: #000;
}

.rf-bar {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 6px 12px;
  border-bottom: 1px solid #141414;
  background: #080808;
}

.rf-item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: #666;
}

.rf-item input[type='range'] { width: 72px; accent-color: #69a; }

.tune-hint {
  flex: 1;
  min-width: 0;
  flex-wrap: wrap;
}

.tune-tip {
  color: #555;
  font-size: 10px;
}

.tune-off {
  color: #ca6;
  font-family: "SF Mono", Menlo, monospace;
  font-size: 10px;
}

.meteor-rf {
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

.wave-label {
  position: absolute;
  left: 8px;
  top: 4px;
  font-size: 10px;
  color: #444;
  z-index: 1;
  pointer-events: none;
}
</style>
