import { inject, provide, reactive, ref, onMounted, onUnmounted } from 'vue';
import { connect } from '@/utils/protocol.js';
import { AudioPlayer, installAudioDebug } from '@/utils/audio.js';
import { Display } from '@/utils/display.js';
import {
  LS_KEY,
  MODE_FILTER_HZ,
  BW_MIN,
  BW_MAX,
  clampFreq,
  fmtZoom,
} from '@/utils/constants.js';

const SDR_KEY = Symbol('sdr');

function loadSettings() {
  try {
    const raw = localStorage.getItem(LS_KEY);
    return raw ? JSON.parse(raw) : null;
  } catch {
    return null;
  }
}

export function provideSdrApp(options = {}) {
  const savedSettings = loadSettings();
  const pendingTune = options.pendingTune;
  const pendingMode = options.pendingMode;

  const state = reactive({
    centerFreq: 100e6,
    tuneFreq: 100e6,
    sampleRate: 2048000,
    filterBW: 150000,
    specMin: -90,
    specMax: -10,
    zoom: 1,
    dragging: false,
  });

  const statusText = ref('连接中...');
  const statusClass = ref('');
  const mode = ref(savedSettings?.mode ?? 'wfm');
  const gain = ref(savedSettings?.gain ?? 20);
  const agc = ref(savedSettings?.agc ?? false);
  const nr = ref(savedSettings?.nr ?? false);
  const nrLevel = ref(savedSettings?.nrLevel ?? 60);
  const volume = ref(savedSettings?.volume ?? 80);
  const cwPitch = ref(savedSettings?.cwPitch ?? 700);
  const playing = ref(false);
  const playDisabled = ref(false);
  const stopDisabled = ref(true);
  const receiverEnabled = ref(true);
  const initialized = ref(false);

  const audio = new AudioPlayer(48000);

  function audioDebugExtra() {
    return {
      volume: volume.value,
      tuneFreq: state.tuneFreq,
      mode: mode.value,
    };
  }
  let display = null;
  let conn = null;
  let tuneTimer = null;
  let filterTimer = null;
  let saveTimer = null;

  function saveSettings() {
    const data = {
      mode: mode.value,
      sampleRate: state.sampleRate,
      filterBW: state.filterBW,
      gain: gain.value,
      agc: agc.value,
      nr: nr.value,
      nrLevel: nrLevel.value,
      volume: volume.value,
      cwPitch: cwPitch.value,
      specMin: state.specMin,
      specMax: state.specMax,
      zoom: state.zoom,
      tuneFreq: state.tuneFreq,
    };
    try { localStorage.setItem(LS_KEY, JSON.stringify(data)); } catch { /* quota */ }
  }

  function scheduleSave() {
    clearTimeout(saveTimer);
    saveTimer = setTimeout(saveSettings, 150);
  }

  function applySettings(saved) {
    if (!saved) return;
    if (saved.mode) mode.value = saved.mode;
    if (saved.sampleRate) state.sampleRate = saved.sampleRate;
    if (saved.filterBW) state.filterBW = saved.filterBW;
    if (saved.gain != null) gain.value = saved.gain;
    if (saved.agc != null) agc.value = saved.agc;
    if (saved.nr != null) nr.value = saved.nr;
    if (saved.nrLevel != null) nrLevel.value = saved.nrLevel;
    if (saved.volume != null) {
      volume.value = saved.volume;
    }
    audio.setVolume(volume.value / 100);
    if (saved.cwPitch != null) cwPitch.value = saved.cwPitch;
    if (saved.specMin != null) state.specMin = saved.specMin;
    if (saved.specMax != null) state.specMax = saved.specMax;
    if (saved.zoom != null) state.zoom = saved.zoom;
    if (saved.tuneFreq != null) state.tuneFreq = clampFreq(saved.tuneFreq);
  }

  applySettings(savedSettings);

  if (pendingMode) {
    mode.value = pendingMode;
    state.filterBW = MODE_FILTER_HZ[pendingMode] ?? state.filterBW;
  }
  if (pendingTune) {
    const hz = clampFreq(pendingTune);
    state.tuneFreq = hz;
    state.centerFreq = hz;
  }

  function setCenterFreq(hz) {
    state.centerFreq = hz;
  }

  function pushSettingsToServer() {
    if (!conn) return;
    conn.send({ cmd: 'service', service: 'radio' });
    // Service switch may reopen the dongle; push the rest after it completes.
    setTimeout(() => {
      if (!conn) return;
      conn.send({ cmd: 'mode', mode: mode.value });
      conn.send({ cmd: 'sampleRate', sampleRate: state.sampleRate });
      conn.send({ cmd: 'filter', filterBW: state.filterBW });
      conn.send({ cmd: 'gain', gain: gain.value });
      conn.send({ cmd: 'agc', agc: agc.value });
      conn.send({ cmd: 'nr', nr: nr.value });
      conn.send({ cmd: 'nrlevel', nrLevel: nrLevel.value / 100 });
      conn.send({ cmd: 'cwpitch', cwPitch: cwPitch.value });
      conn.send({ cmd: 'center', freq: clampFreq(state.centerFreq) });
      conn.send({ cmd: 'tune', freq: clampFreq(state.tuneFreq) });
    }, 150);
  }

  function sendTune() {
    audio.resetForTune();
    conn?.send({ cmd: 'tune', freq: clampFreq(state.tuneFreq) });
  }

  function scheduleTune() {
    clearTimeout(tuneTimer);
    tuneTimer = setTimeout(sendTune, 50);
  }

  function setTuneFreq(hz, sendNow = true) {
    state.tuneFreq = hz;
    scheduleSave();
    if (sendNow) scheduleTune();
  }

  async function ensurePlaying() {
    audio.unlockFromGesture();
    audio.setVolume(volume.value / 100);
    if (audio.playing) return;
    playDisabled.value = true;
    stopDisabled.value = false;
    try {
      await audio.start();
      playing.value = true;
    } catch {
      playDisabled.value = false;
      stopDisabled.value = true;
      playing.value = false;
    }
  }

  audio.onError = (msg) => {
    statusText.value = '音频错误: ' + msg;
    statusClass.value = 'err';
    playDisabled.value = false;
    stopDisabled.value = true;
    playing.value = false;
  };

  audio.onTransportChange = (t) => {
    if (!playing.value && t === 'none') return;
    if (t === 'webrtc') statusText.value = '已连接 WebRTC';
    else if (t === 'ws') statusText.value = '已连接';
    else if (statusClass.value === 'ok') statusText.value = '已连接';
  };

  function applyStatus(msg) {
    if (msg.enabled !== undefined) {
      receiverEnabled.value = msg.enabled;
      if (!msg.enabled) {
        statusText.value = 'SDR 已关闭';
        statusClass.value = 'err';
        playDisabled.value = true;
      } else if (statusText.value === 'SDR 已关闭') {
        statusText.value = '已连接';
        statusClass.value = 'ok';
        playDisabled.value = false;
      }
    }
    if (!initialized.value) {
      state.centerFreq = msg.centerFreq;
      if (!savedSettings?.tuneFreq) {
        state.tuneFreq = msg.tuneFreq || msg.centerFreq;
      }
      if (msg.sampleRate) state.sampleRate = msg.sampleRate;
      initialized.value = true;
    } else {
      setCenterFreq(msg.centerFreq);
    }
    if (msg.sampleRate) state.sampleRate = msg.sampleRate;
  }

  function applyMode() {
    state.filterBW = MODE_FILTER_HZ[mode.value] ?? 10000;
    conn?.send({ cmd: 'mode', mode: mode.value });
    conn?.send({ cmd: 'filter', filterBW: state.filterBW });
    scheduleSave();
  }

  function setBandwidth(hz, sendNow = true) {
    const bw = Math.max(BW_MIN, Math.min(BW_MAX, Math.round(hz)));
    state.filterBW = bw;
    scheduleSave();
    if (sendNow) {
      clearTimeout(filterTimer);
      conn?.send({ cmd: 'filter', filterBW: bw });
    } else {
      clearTimeout(filterTimer);
      filterTimer = setTimeout(() => conn?.send({ cmd: 'filter', filterBW: state.filterBW }), 60);
    }
  }

  function applyBandwidth(hz) {
    setBandwidth(hz, true);
  }

  function onSampleRateChange() {
    conn?.send({ cmd: 'sampleRate', sampleRate: state.sampleRate });
    scheduleSave();
  }

  function onRecenter() {
    conn?.send({ cmd: 'center', freq: clampFreq(state.tuneFreq) });
  }

  function onCwPitchInput() {
    conn?.send({ cmd: 'cwpitch', cwPitch: cwPitch.value });
    scheduleSave();
  }

  function onGainInput() {
    conn?.send({ cmd: 'gain', gain: gain.value });
    scheduleSave();
  }

  function onAgcChange() {
    conn?.send({ cmd: 'agc', agc: agc.value });
    scheduleSave();
  }

  function onNrChange() {
    conn?.send({ cmd: 'nr', nr: nr.value });
    scheduleSave();
  }

  function onNrLevelInput() {
    conn?.send({ cmd: 'nrlevel', nrLevel: nrLevel.value / 100 });
    scheduleSave();
  }

  function onVolumeInput() {
    audio.setVolume(volume.value / 100);
    scheduleSave();
  }

  function onPlay() {
    audio.unlockFromGesture();
    audio.setVolume(volume.value / 100);
    void ensurePlaying();
    sendTune();
  }

  function onStop() {
    audio.stop();
    playDisabled.value = false;
    stopDisabled.value = true;
    playing.value = false;
  }

  function syncSpecRange() {
    if (state.specMin > state.specMax - 5) state.specMin = state.specMax - 5;
    scheduleSave();
  }

  function setSpecMax(v) {
    state.specMax = v;
    syncSpecRange();
  }

  function setSpecMin(v) {
    state.specMin = v;
    syncSpecRange();
  }

  function setZoom(v) {
    state.zoom = v;
    scheduleSave();
  }

  const zoomLabel = () => fmtZoom(state.zoom);

  // Canvas interaction state
  let dragCanvas = null;
  let dragMode = 'tune';

  function canvasX(clientX, canvas) {
    const rect = canvas.getBoundingClientRect();
    const dpr = window.devicePixelRatio || 1;
    return (clientX - rect.left) * dpr;
  }

  function bwEdgeUnder(cx) {
    if (!display) return null;
    const dpr = window.devicePixelRatio || 1;
    const half = state.filterBW / 2;
    const center = display.freqToCanvasX(state.tuneFreq);
    const left = display.freqToCanvasX(state.tuneFreq - half);
    const right = display.freqToCanvasX(state.tuneFreq + half);
    if (Math.abs(right - center) < 14 * dpr) return null;
    const th = 7 * dpr;
    if (Math.abs(cx - left) <= th) return 'left';
    if (Math.abs(cx - right) <= th) return 'right';
    return null;
  }

  function onCanvasWheel(e) {
    e.preventDefault();
    ensurePlaying();
    const zoom = state.zoom > 1 ? state.zoom : 1;
    const span = state.sampleRate / zoom;
    let step = Math.max(100, Math.round(span / 100));
    if (e.shiftKey) step *= 10;
    const dir = e.deltaY < 0 ? 1 : -1;
    setTuneFreq(clampFreq(state.tuneFreq + dir * step));
  }

  function onCanvasDown(clientX, canvas) {
    dragCanvas = canvas;
    state.dragging = true;
    audio.unlockFromGesture();
    void ensurePlaying();
    const cx = canvasX(clientX, canvas);
    dragMode = bwEdgeUnder(cx) ? 'bw' : 'tune';
    if (dragMode === 'bw') {
      setBandwidth(Math.abs(display.canvasXToFreq(cx) - state.tuneFreq) * 2, false);
    } else {
      setTuneFreq(clampFreq(display.canvasXToFreq(cx)));
    }
  }

  function onCanvasMove(clientX) {
    if (!state.dragging || !dragCanvas) return;
    const cx = canvasX(clientX, dragCanvas);
    if (dragMode === 'bw') {
      setBandwidth(Math.abs(display.canvasXToFreq(cx) - state.tuneFreq) * 2, false);
    } else {
      setTuneFreq(clampFreq(display.canvasXToFreq(cx)));
    }
  }

  function onCanvasUp() {
    if (dragMode === 'bw') conn?.send({ cmd: 'filter', filterBW: state.filterBW });
    else if (dragMode === 'tune') {
      clearTimeout(tuneTimer);
      sendTune();
    }
    state.dragging = false;
    dragCanvas = null;
  }

  function onCanvasHover(clientX, canvas) {
    if (state.dragging) return;
    canvas.style.cursor = bwEdgeUnder(canvasX(clientX, canvas)) ? 'ew-resize' : 'crosshair';
  }

  function initDisplay(waterfallEl, spectrumEl) {
    display = new Display(waterfallEl, spectrumEl, () => state);
    const onResize = () => display.resize();
    window.addEventListener('resize', onResize);
    return () => window.removeEventListener('resize', onResize);
  }

  function startConnection() {
    conn = connect({
      onOpen: () => {
        statusText.value = '已连接';
        statusClass.value = 'ok';
        pushSettingsToServer();
      },
      onClose: () => {
        statusText.value = '已断开，重连中...';
        statusClass.value = 'err';
      },
      onStatus: applyStatus,
      onSpectrum: (centerFreq, bins) => {
        setCenterFreq(centerFreq);
        display?.pushSpectrum(bins);
      },
      onAudio: (pcm) => audio.push(pcm),
    });
  }

  onMounted(() => {
    installAudioDebug(audio, audioDebugExtra);
    audio.setVolume(volume.value / 100);
    startConnection();
    window.addEventListener('mousemove', onGlobalMove);
    window.addEventListener('mouseup', onCanvasUp);
    window.addEventListener('touchend', onCanvasUp);
  });

  function onGlobalMove(e) {
    onCanvasMove(e.clientX);
  }

  onUnmounted(() => {
    window.removeEventListener('mousemove', onGlobalMove);
    window.removeEventListener('mouseup', onCanvasUp);
    window.removeEventListener('touchend', onCanvasUp);
    conn?.close();
  });

  const sdr = {
    state,
    statusText,
    statusClass,
    mode,
    gain,
    agc,
    nr,
    nrLevel,
    volume,
    cwPitch,
    playing,
    playDisabled,
    stopDisabled,
    receiverEnabled,
    audio,
    setTuneFreq,
    applyMode,
    applyBandwidth,
    setBandwidth,
    onSampleRateChange,
    onRecenter,
    onCwPitchInput,
    onGainInput,
    onAgcChange,
    onNrChange,
    onNrLevelInput,
    onVolumeInput,
    onPlay,
    onStop,
    setSpecMax,
    setSpecMin,
    setZoom,
    zoomLabel,
    initDisplay,
    onCanvasDown,
    onCanvasMove,
    onCanvasUp,
    onCanvasHover,
    onCanvasWheel,
  };

  provide(SDR_KEY, sdr);
  return sdr;
}

export function useSdrApp() {
  const sdr = inject(SDR_KEY);
  if (!sdr) throw new Error('useSdrApp() must be used within provideSdrApp()');
  return sdr;
}
