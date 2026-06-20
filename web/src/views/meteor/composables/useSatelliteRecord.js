import { ref, computed, watch, onUnmounted } from 'vue';
import { buildIQWav, downloadBlob, fmtBytes } from '@/utils/iqWav.js';
import { currentPass, nextPass, fmtCountdown } from '@/utils/satellitePass.js';

const RATE_OPTIONS = [
  { value: 1_024_000, label: '1.024 Msps' },
  { value: 2_048_000, label: '2.048 Msps' },
  { value: 2_400_000, label: '2.400 Msps' },
];

function passFileLabel(pass) {
  const d = pass.aos;
  const p = (n) => String(n).padStart(2, '0');
  return `pass_${d.getUTCFullYear()}${p(d.getUTCMonth() + 1)}${p(d.getUTCDate())}-${p(d.getUTCHours())}${p(d.getUTCMinutes())}`;
}

export function useSatelliteRecord({ sendCmd, setSampleRate, getAutoPassCtx }) {
  const sampleRate = ref(2_048_000);
  const channels = ref(2);
  const state = ref('idle'); // idle | recording | paused
  const chunks = ref([]);
  const bytesTotal = ref(0);
  const startedAt = ref(0);
  const elapsedMs = ref(0);
  const autoPassRecord = ref(false);
  const preRollSec = ref(5);
  const postRollSec = ref(5);
  const autoPassStatus = ref('');

  let tickTimer = null;
  let autoPassTimer = null;
  let handledPassKey = null;
  let autoRecordingPassKey = null;
  let autoRecordingPass = null;

  function syncBackend() {
    const active = state.value === 'recording' || state.value === 'paused';
    sendCmd?.({
      cmd: 'meteorRecord',
      meteorRecord: active,
      meteorRecordPause: state.value === 'paused',
      meteorRecordChannels: channels.value,
    });
  }

  function startTick() {
    stopTick();
    tickTimer = setInterval(() => {
      if (state.value === 'recording' && startedAt.value) {
        elapsedMs.value = Date.now() - startedAt.value;
      }
    }, 200);
  }

  function stopTick() {
    if (tickTimer) {
      clearInterval(tickTimer);
      tickTimer = null;
    }
  }

  function pushIQ(frame) {
    if (state.value !== 'recording') return;
    const copy = new Int8Array(frame.data);
    chunks.value.push({
      centerHz: frame.centerHz,
      rate: frame.rate,
      channels: frame.channels,
      data: copy,
    });
    bytesTotal.value += copy.length;
  }

  function start(opts = {}) {
    if (state.value !== 'idle') return;
    setSampleRate?.(sampleRate.value);
    chunks.value = [];
    bytesTotal.value = 0;
    startedAt.value = Date.now();
    elapsedMs.value = 0;
    state.value = 'recording';
    if (opts.autoPassKey != null) {
      autoRecordingPassKey = opts.autoPassKey;
      autoRecordingPass = opts.pass || null;
    } else {
      autoRecordingPassKey = null;
      autoRecordingPass = null;
    }
    syncBackend();
    startTick();
  }

  function pause() {
    if (state.value !== 'recording') return;
    state.value = 'paused';
    syncBackend();
  }

  function resume() {
    if (state.value !== 'paused') return;
    state.value = 'recording';
    syncBackend();
  }

  function stop() {
    state.value = 'idle';
    autoRecordingPassKey = null;
    autoRecordingPass = null;
    syncBackend();
    stopTick();
  }

  function download(satName = 'sat', pass = null) {
    const blob = buildIQWav(chunks.value, channels.value);
    if (!blob) return false;
    const ts = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19);
    const ch = channels.value === 2 ? 'iq' : 'i';
    const passPart = pass ? `_${passFileLabel(pass)}` : '';
    downloadBlob(blob, `${satName}${passPart}_${ch}_${(sampleRate.value / 1e6).toFixed(3)}Msps_${ts}.wav`);
    return true;
  }

  function stopAndDownload(satName, pass) {
    const hadData = chunks.value.length > 0;
    const p = pass || autoRecordingPass;
    stop();
    if (hadData) download(satName, p);
  }

  function findAutoTarget(passes, nowMs) {
    const cur = currentPass(passes, nowMs);
    if (cur) return cur;
    return nextPass(passes, nowMs);
  }

  function tickAutoPass() {
    if (!autoPassRecord.value) {
      autoPassStatus.value = '';
      return;
    }
    const ctx = getAutoPassCtx?.();
    if (!ctx) return;
    const { passes, isGeo, nowMs, satName } = ctx;

    if (isGeo) {
      autoPassStatus.value = 'GEO 不适用';
      return;
    }
    if (!passes?.length) {
      autoPassStatus.value = '无过境数据';
      return;
    }

    let target = findAutoTarget(passes, nowMs);
    if (!target) {
      autoPassStatus.value = '无计划过境';
      return;
    }

    let passKey = target.aos.getTime();
    let recStart = passKey - preRollSec.value * 1000;
    let recEnd = target.los.getTime() + postRollSec.value * 1000;

    while (state.value === 'idle' && nowMs > recEnd && passKey !== handledPassKey) {
      handledPassKey = passKey;
      target = nextPass(passes, target.los.getTime());
      if (!target) {
        autoPassStatus.value = '等待下次过境';
        return;
      }
      passKey = target.aos.getTime();
      recStart = passKey - preRollSec.value * 1000;
      recEnd = target.los.getTime() + postRollSec.value * 1000;
    }

    if (state.value === 'idle' && passKey === handledPassKey) {
      const nxt = nextPass(passes, target.los.getTime());
      if (!nxt) {
        autoPassStatus.value = '等待下次过境';
        return;
      }
      const nextStart = nxt.aos.getTime() - preRollSec.value * 1000;
      const wait = nextStart - nowMs;
      autoPassStatus.value = wait > 0 ? `下次 ${fmtCountdown(wait)}` : '即将开始';
      return;
    }

    if (state.value === 'recording' && autoRecordingPassKey != null) {
      if (nowMs >= recEnd) {
        stopAndDownload(satName || 'sat', target);
        handledPassKey = passKey;
        autoPassStatus.value = '已自动保存';
        return;
      }
      autoPassStatus.value = '自动录制中';
      return;
    }

    if (state.value === 'idle' && nowMs >= recStart && nowMs < recEnd) {
      start({ autoPassKey: passKey, pass: target });
      autoPassStatus.value = '自动录制中';
      return;
    }

    if (state.value === 'idle' && nowMs < recStart) {
      autoPassStatus.value = `等待 ${fmtCountdown(recStart - nowMs)}`;
    }
  }

  function startAutoPassTimer() {
    stopAutoPassTimer();
    autoPassTimer = setInterval(tickAutoPass, 1000);
    tickAutoPass();
  }

  function stopAutoPassTimer() {
    if (autoPassTimer) {
      clearInterval(autoPassTimer);
      autoPassTimer = null;
    }
  }

  function resetAutoPassState(opts = {}) {
    const { downloadIfRecording = false, satName = 'sat' } = opts;
    if (state.value !== 'idle' && autoRecordingPassKey != null) {
      if (downloadIfRecording) stopAndDownload(satName, autoRecordingPass);
      else stop();
    }
    handledPassKey = null;
    autoRecordingPassKey = null;
    autoRecordingPass = null;
    autoPassStatus.value = '';
  }

  watch(autoPassRecord, (on) => {
    if (on) startAutoPassTimer();
    else {
      stopAutoPassTimer();
      autoPassStatus.value = '';
    }
  });

  onUnmounted(() => {
    stopTick();
    stopAutoPassTimer();
  });

  const elapsedStr = computed(() => {
    const ms = elapsedMs.value;
    const sec = Math.floor(ms / 1000);
    const m = Math.floor(sec / 60);
    const s = sec % 60;
    return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
  });

  const sizeStr = computed(() => fmtBytes(bytesTotal.value));

  const canDownload = computed(() => chunks.value.length > 0 && state.value !== 'recording');

  return {
    RATE_OPTIONS,
    sampleRate,
    channels,
    state,
    bytesTotal,
    elapsedStr,
    sizeStr,
    canDownload,
    autoPassRecord,
    preRollSec,
    postRollSec,
    autoPassStatus,
    pushIQ,
    start,
    pause,
    resume,
    stop,
    download,
    resetAutoPassState,
    tickAutoPass,
  };
}
