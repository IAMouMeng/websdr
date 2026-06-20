import { ref, reactive, onMounted, onUnmounted } from 'vue';
import { connect } from '@/utils/protocol.js';
import { Display } from '@/utils/display.js';
import { createWaveform } from '@/utils/waveform.js';

const DEFAULT_CENTER = 137_900_000;
const DEFAULT_SAMPLE_RATE = 2_048_000;
export const METEOR_DEFAULT_FILTER = 150_000;

export function useMeteorListen({ freqHz, norad, autoDoppler, onTrackTick, onIQ }) {
  const meteor = ref({
    freqHz: freqHz || DEFAULT_CENTER,
    nominalFreqHz: freqHz || DEFAULT_CENTER,
    centerFreqHz: DEFAULT_CENTER,
    sampleRateHz: DEFAULT_SAMPLE_RATE,
    dopplerHz: 0,
    manualOffsetHz: 0,
    freq: '',
    strength: null,
    metric: '',
    locked: false,
    synced: false,
    decoded: false,
    lines: 0,
    image: '',
    listening: false,
    elapsedSec: 0,
    elevation: 0,
    azimuth: 0,
    autoDoppler: false,
    norad: norad || 40069,
    satellite: '',
    constellation: [],
    channels: [],
  });
  const statusText = ref('连接中…');
  const connected = ref(false);
  const gain = ref(20);
  const agc = ref(false);

  const displayState = reactive({
    centerFreq: DEFAULT_CENTER,
    tuneFreq: freqHz || DEFAULT_CENTER,
    sampleRate: DEFAULT_SAMPLE_RATE,
    filterBW: METEOR_DEFAULT_FILTER,
    specMin: -110,
    specMax: -20,
    zoom: 2,
    dragging: false,
  });

  let conn = null;
  let display = null;
  let waveform = null;
  let trackTimer = null;
  let tuneTimer = null;
  let dragCanvas = null;
  let dragMode = 'tune';

  const imageHold = reactive({
    composite: '',
    channels: [],
    lines: 0,
    synced: false,
  });

  function clearImageHold() {
    imageHold.composite = '';
    imageHold.channels = [];
    imageHold.lines = 0;
    imageHold.synced = false;
    meteor.value = {
      ...meteor.value,
      image: '',
      lines: 0,
      synced: false,
      locked: false,
      decoded: false,
      constellation: [],
      channels: (meteor.value.channels || []).map((ch) => ({
        ...ch,
        image: '',
        lines: 0,
        active: false,
      })),
    };
  }

  let heldNorad = norad || 40069;

  function applyImageHold(raw) {
    if (raw.norad && raw.norad !== heldNorad) {
      heldNorad = raw.norad;
      imageHold.composite = '';
      imageHold.channels = [];
      imageHold.lines = 0;
      imageHold.synced = false;
    }
    if (raw.locked && raw.lines > imageHold.lines) {
      if (raw.image) imageHold.composite = raw.image;
      raw.channels?.forEach((ch, i) => {
        if (ch.image) imageHold.channels[i] = ch.image;
      });
      imageHold.lines = raw.lines;
    }
    if (raw.synced) imageHold.synced = true;

    const channels = (raw.channels || []).map((ch, i) => ({
      ...ch,
      image: imageHold.channels[i] || '',
      lines: imageHold.lines,
      active: imageHold.lines >= 2,
    }));

    return {
      ...raw,
      image: imageHold.composite,
      lines: imageHold.lines,
      synced: imageHold.synced,
      channels,
    };
  }

  function stop() {
    if (trackTimer) {
      clearInterval(trackTimer);
      trackTimer = null;
    }
    clearTimeout(tuneTimer);
    if (!conn) return;
    if (connected.value) {
      conn.send({ cmd: 'meteorListen', meteorListen: false });
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
  }

  function setAgc(v) {
    agc.value = v;
    conn?.send({ cmd: 'agc', agc: v });
  }

  function setFilterBW(hz) {
    displayState.filterBW = hz;
    conn?.send({ cmd: 'filter', filterBW: hz });
  }

  function clampTune(hz) {
    const center = displayState.centerFreq;
    const half = displayState.sampleRate / 2;
    const lo = center - half + 1000;
    const hi = center + half - 1000;
    return Math.max(lo, Math.min(hi, Math.round(hz)));
  }

  function setManualTune(hz, immediate = false) {
    const f = clampTune(hz);
    displayState.tuneFreq = f;
    clearTimeout(tuneTimer);
    const send = () => conn?.send({ cmd: 'meteorTune', freq: f });
    if (immediate) send();
    else tuneTimer = setTimeout(send, 80);
  }

  function canvasX(clientX, canvas) {
    const rect = canvas.getBoundingClientRect();
    const dpr = window.devicePixelRatio || 1;
    return (clientX - rect.left) * dpr;
  }

  function bwEdgeUnder(cx) {
    if (!display) return null;
    const dpr = window.devicePixelRatio || 1;
    const half = displayState.filterBW / 2;
    const center = display.freqToCanvasX(displayState.tuneFreq);
    const left = display.freqToCanvasX(displayState.tuneFreq - half);
    const right = display.freqToCanvasX(displayState.tuneFreq + half);
    if (Math.abs(right - center) < 14 * dpr) return null;
    const th = 7 * dpr;
    if (Math.abs(cx - left) <= th) return 'left';
    if (Math.abs(cx - right) <= th) return 'right';
    return null;
  }

  function onCanvasWheel(e) {
    e.preventDefault();
    const zoom = displayState.zoom > 1 ? displayState.zoom : 1;
    const span = displayState.sampleRate / zoom;
    let step = Math.max(100, Math.round(span / 100));
    if (e.shiftKey) step *= 10;
    const dir = e.deltaY < 0 ? 1 : -1;
    setManualTune(displayState.tuneFreq + dir * step);
  }

  function onCanvasDown(clientX, canvas) {
    if (!display) return;
    dragCanvas = canvas;
    displayState.dragging = true;
    const cx = canvasX(clientX, canvas);
    dragMode = bwEdgeUnder(cx) ? 'bw' : 'tune';
    if (dragMode === 'bw') {
      displayState.filterBW = Math.abs(display.canvasXToFreq(cx) - displayState.tuneFreq) * 2;
    } else {
      setManualTune(display.canvasXToFreq(cx), true);
    }
  }

  function onCanvasMove(clientX) {
    if (!displayState.dragging || !dragCanvas || !display) return;
    const cx = canvasX(clientX, dragCanvas);
    if (dragMode === 'bw') {
      displayState.filterBW = Math.abs(display.canvasXToFreq(cx) - displayState.tuneFreq) * 2;
    } else {
      setManualTune(display.canvasXToFreq(cx));
    }
  }

  function onCanvasUp() {
    if (dragMode === 'bw') conn?.send({ cmd: 'filter', filterBW: displayState.filterBW });
    displayState.dragging = false;
    dragCanvas = null;
  }

  function onCanvasHover(clientX, canvas) {
    if (displayState.dragging) return;
    canvas.style.cursor = bwEdgeUnder(canvasX(clientX, canvas)) ? 'ew-resize' : 'crosshair';
  }

  function startListen(opts = {}) {
    if (!conn || !connected.value) return;
    const f = opts.freqHz ?? freqHz ?? DEFAULT_CENTER;
    const n = opts.norad ?? norad ?? 40069;
    const ad = opts.autoDoppler ?? autoDoppler ?? false;
    conn.send({
      cmd: 'meteorListen',
      meteorListen: true,
      freq: f,
      meteorNorad: n,
      meteorAutoDoppler: ad,
    });
  }

  function sendTrack({ dopplerHz, elevation, azimuth }) {
    conn?.send({
      cmd: 'meteorTrack',
      meteorDoppler: dopplerHz,
      meteorElevation: elevation,
      meteorAzimuth: azimuth,
    });
  }

  function sendCmd(obj) {
    conn?.send(obj);
  }

  function setDeviceSampleRate(hz) {
    displayState.sampleRate = hz;
    conn?.send({ cmd: 'sampleRate', sampleRate: hz });
  }

  function resetDisplay() {
    display?.resetWaterfall();
    clearImageHold();
  }

  onMounted(() => {
    conn = connect({
      onOpen: () => {
        connected.value = true;
        statusText.value = '监听中';
        conn.send({ cmd: 'service', service: 'meteor' });
        conn.send({ cmd: 'gain', gain: gain.value });
        conn.send({ cmd: 'agc', agc: agc.value });
        conn.send({ cmd: 'filter', filterBW: displayState.filterBW });
        startListen({ freqHz, norad, autoDoppler });

        trackTimer = setInterval(() => {
          onTrackTick?.(sendTrack);
        }, 1000);
      },
      onClose: () => {
        connected.value = false;
        statusText.value = '已断开';
        meteor.value = { ...meteor.value, listening: false };
      },
      onMessage: (msg) => {
        if (msg.type !== 'meteor' || !msg.meteor) return;
        meteor.value = applyImageHold(msg.meteor);
        const m = meteor.value;
        if (!displayState.dragging) {
          if (m.freqHz) displayState.tuneFreq = m.freqHz;
        }
        if (m.centerFreqHz) displayState.centerFreq = m.centerFreqHz;
        if (m.sampleRateHz) displayState.sampleRate = m.sampleRateHz;
      },
      onSpectrum: (centerFreq, bins) => {
        displayState.centerFreq = centerFreq;
        display?.pushSpectrum(bins);
      },
      onAudio: (pcm) => waveform?.push(pcm),
      onIQ: (frame) => onIQ?.(frame),
    });
  });

  onUnmounted(stop);

  return {
    meteor,
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
    setManualTune,
    onCanvasDown,
    onCanvasMove,
    onCanvasUp,
    onCanvasWheel,
    onCanvasHover,
    startListen,
    sendTrack,
    resetDisplay,
    clearImageHold,
    sendCmd,
    setDeviceSampleRate,
  };
}
