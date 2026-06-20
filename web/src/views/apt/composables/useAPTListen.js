import { ref, reactive, onMounted, onUnmounted } from 'vue';
import { connect } from '@/utils/protocol.js';
import { Display } from '@/utils/display.js';
import { createWaveform } from '@/utils/waveform.js';
import { LS_KEY, SPEC_MAX_RANGE, SPEC_MIN_RANGE, ZOOM_RANGE, snapVal } from '@/utils/constants.js';

const APT_CENTER = 137_100_000;
const APT_SAMPLE_RATE = 1_024_000;
export const APT_FILTER_MIN = 20_000;
export const APT_FILTER_MAX = 80_000;
export const APT_DEFAULT_FILTER = 40_000;

function loadSettings() {
  try {
    const raw = localStorage.getItem(LS_KEY);
    return raw ? JSON.parse(raw) : null;
  } catch {
    return null;
  }
}

function saveSettings(data) {
  try {
    const prev = loadSettings() || {};
    localStorage.setItem(LS_KEY, JSON.stringify({ ...prev, ...data }));
  } catch { /* quota */ }
}

export function useAPTListen(freqHz) {
  const saved = loadSettings();

  const apt = ref({
    freqHz: freqHz || 0,
    freq: '',
    strength: null,
    lines: 0,
    image: '',
    decoded: false,
    metric: '',
    listening: false,
    elapsedSec: 0,
  });
  const statusText = ref('连接中…');
  const connected = ref(false);
  const gain = ref(saved?.gain ?? 20);
  const agc = ref(saved?.agc ?? false);

  const displayState = reactive({
    centerFreq: APT_CENTER,
    tuneFreq: freqHz || APT_CENTER,
    sampleRate: APT_SAMPLE_RATE,
    filterBW: saved?.aptFilterBW ?? APT_DEFAULT_FILTER,
    specMin: saved?.specMin ?? -110,
    specMax: saved?.specMax ?? -20,
    zoom: saved?.zoom ?? 1,
  });

  let conn = null;
  let display = null;
  let waveform = null;
  let saveTimer = null;

  function scheduleSave() {
    clearTimeout(saveTimer);
    saveTimer = setTimeout(() => {
      saveSettings({
        gain: gain.value,
        agc: agc.value,
        aptFilterBW: displayState.filterBW,
        specMin: displayState.specMin,
        specMax: displayState.specMax,
        zoom: displayState.zoom,
      });
    }, 150);
  }

  function pushRfSettings() {
    if (!conn) return;
    conn.send({ cmd: 'gain', gain: gain.value });
    conn.send({ cmd: 'agc', agc: agc.value });
    conn.send({ cmd: 'filter', filterBW: displayState.filterBW });
  }

  function stop() {
    if (!conn) return;
    if (connected.value) {
      conn.send({ cmd: 'aptListen', aptListen: false });
      conn.send({ cmd: 'service', service: 'radio' });
    }
    conn.close();
    conn = null;
    connected.value = false;
  }

  function initDisplay(waterfallEl, spectrumEl, waveformEl) {
    display = new Display(waterfallEl, spectrumEl, () => displayState);
    waveform = createWaveform(waveformEl);
    const onResize = () => {
      display.resize();
      waveform.resize();
    };
    window.addEventListener('resize', onResize);
    return () => window.removeEventListener('resize', onResize);
  }

  function setGain(v) {
    gain.value = v;
    conn?.send({ cmd: 'gain', gain: v });
    scheduleSave();
  }

  function setAgc(v) {
    agc.value = v;
    conn?.send({ cmd: 'agc', agc: v });
    scheduleSave();
  }

  function setFilterBW(hz) {
    const bw = Math.max(APT_FILTER_MIN, Math.min(APT_FILTER_MAX, Math.round(hz)));
    displayState.filterBW = bw;
    conn?.send({ cmd: 'filter', filterBW: bw });
    scheduleSave();
  }

  function setSpecMax(v) {
    displayState.specMax = snapVal(v, SPEC_MAX_RANGE);
    scheduleSave();
  }

  function setSpecMin(v) {
    displayState.specMin = snapVal(v, SPEC_MIN_RANGE);
    scheduleSave();
  }

  function setZoom(v) {
    displayState.zoom = snapVal(v, ZOOM_RANGE);
    scheduleSave();
  }

  onMounted(() => {
    conn = connect({
      onOpen: () => {
        connected.value = true;
        statusText.value = '监听中';
        conn.send({ cmd: 'service', service: 'apt' });
        pushRfSettings();
        conn.send({ cmd: 'aptListen', aptListen: true, freq: freqHz || 0 });
      },
      onClose: () => {
        connected.value = false;
        statusText.value = '已断开';
        apt.value = { ...apt.value, listening: false };
      },
      onStatus: (msg) => {
        if (msg.centerFreq) displayState.centerFreq = msg.centerFreq;
        if (msg.sampleRate) displayState.sampleRate = msg.sampleRate;
        if (msg.filterBW && msg.service === 'apt') {
          displayState.filterBW = msg.filterBW;
        }
        if (msg.gain != null) gain.value = msg.gain;
        if (msg.agc != null) agc.value = msg.agc;
      },
      onMessage: (msg) => {
        if (msg.type !== 'apt' || !msg.apt) return;
        apt.value = msg.apt;
        if (msg.apt.freqHz) {
          displayState.tuneFreq = msg.apt.freqHz;
        }
      },
      onSpectrum: (centerFreq, bins) => {
        displayState.centerFreq = centerFreq;
        displayState.sampleRate = APT_SAMPLE_RATE;
        display?.pushSpectrum(bins);
      },
      onAudio: (pcm) => waveform?.push(pcm),
    });
  });

  onUnmounted(stop);

  return {
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
  };
}
