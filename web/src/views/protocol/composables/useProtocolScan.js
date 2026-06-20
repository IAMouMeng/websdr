import { inject, provide, ref, computed, reactive, onUnmounted } from 'vue';
import { connect } from '@/utils/protocol.js';
import { Display } from '@/utils/display.js';
import { createWaveform } from '@/utils/waveform.js';
import { TYPE_LABELS, GROUP_ORDER } from '../constants.js';

const TYPE_RANK = Object.fromEntries(GROUP_ORDER.map((t, i) => [t, i]));

function sortSignals(list) {
  return [...list].sort((a, b) => {
    const tr = (TYPE_RANK[a.type] ?? 99) - (TYPE_RANK[b.type] ?? 99);
    if (tr !== 0) return tr;
    const fr = (a.freqHz || 0) - (b.freqHz || 0);
    if (fr !== 0) return fr;
    return String(a.id).localeCompare(String(b.id));
  });
}

const PROTOCOL_KEY = Symbol('protocol');

export function provideProtocolScan() {
  const listening = ref(false);
  const fullScanning = ref(false);
  const fullScanPhase = ref('idle'); // idle | scanning | analyzing | done
  const startedAt = ref(null);
  const signals = ref([]);
  const bandSummaries = ref([]);
  const expanded = ref(new Set());
  const connected = ref(false);
  const receiverEnabled = ref(true);
  const statusText = ref('未连接');
  const scanBand = ref('');
  const scanProgress = ref(null);

  const displayState = reactive({
    centerFreq: 98_000_000,
    tuneFreq: 98_000_000,
    sampleRate: 2_048_000,
    filterBW: 200_000,
    specMin: -110,
    specMax: -20,
    zoom: 1,
    hideDbAxis: true,
  });

  let conn = null;
  let display = null;
  let waveform = null;

  const filters = ref(
    Object.fromEntries(GROUP_ORDER.map((k) => [k, true])),
  );

  const filteredSignals = computed(() =>
    signals.value.filter((s) => filters.value[s.type]),
  );

  const counts = computed(() =>
    Object.fromEntries(
      GROUP_ORDER.map((t) => [t, signals.value.filter((s) => s.type === t).length]),
    ),
  );

  const totalCount = computed(() => signals.value.length);

  const showScanPanel = computed(
    () => fullScanning.value || fullScanPhase.value === 'analyzing',
  );

  function toggleExpand(id) {
    const next = new Set(expanded.value);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    expanded.value = next;
  }

  function handleProtocolMessage(msg) {
    signals.value = sortSignals(msg.signals || []);
    if (msg.scanBand) scanBand.value = msg.scanBand;
    if (msg.scanProgress) {
      scanProgress.value = msg.scanProgress;
      fullScanPhase.value = msg.scanProgress.phase || 'scanning';
      if (msg.scanProgress.bandName) scanBand.value = msg.scanProgress.bandName;
      if (msg.scanProgress.centerHz) {
        displayState.centerFreq = msg.scanProgress.centerHz;
        displayState.tuneFreq = msg.scanProgress.centerHz;
      }
      if (msg.scanProgress.rateHz) {
        displayState.sampleRate = msg.scanProgress.rateHz;
      }
    }
    if (msg.fullScanComplete) {
      fullScanning.value = false;
      fullScanPhase.value = 'done';
      listening.value = false;
      bandSummaries.value = [];
      scanProgress.value = msg.scanProgress || { pct: 100, phase: 'done' };
      if (conn && connected.value) {
        conn.send({ cmd: 'service', service: 'radio' });
      }
    }
  }

  function openConnection() {
    if (conn) return;
    conn = connect({
      onOpen: () => {
        connected.value = true;
        statusText.value = '已连接';
      },
      onClose: () => {
        connected.value = false;
        if (listening.value || fullScanning.value) {
          statusText.value = '已断开，重连中…';
        } else {
          statusText.value = '未连接';
        }
      },
      onStatus: (msg) => {
        if (msg.enabled !== undefined) {
          receiverEnabled.value = msg.enabled;
          if (!msg.enabled) statusText.value = 'SDR 已关闭';
          else if (connected.value) statusText.value = '已连接';
        }
      },
      onMessage: (msg) => {
        if (msg.type !== 'protocol') return;
        handleProtocolMessage(msg);
      },
      onSpectrum: (centerFreq, bins) => {
        displayState.centerFreq = centerFreq;
        displayState.tuneFreq = centerFreq;
        const rate = scanProgress.value?.rateHz || displayState.sampleRate;
        displayState.sampleRate = rate;
        if (fullScanning.value || fullScanPhase.value === 'analyzing') {
          display?.pushSpectrum(bins);
        }
      },
      onAudio: (pcm) => waveform?.push(pcm),
    });
  }

  function closeConnection() {
    if (!conn) return;
    if (connected.value) {
      if (fullScanning.value) {
        conn.send({ cmd: 'protocolFullScan', protocolFullScan: false });
      }
      if (listening.value) {
        conn.send({ cmd: 'protocolListen', protocolListen: false });
      }
      conn.send({ cmd: 'service', service: 'radio' });
    }
    conn.close();
    conn = null;
    connected.value = false;
    statusText.value = '未连接';
    scanBand.value = '';
    scanProgress.value = null;
  }

  function afterConnectStart(cmd) {
    openConnection();
    const send = () => {
      if (!conn || !connected.value) {
        setTimeout(send, 100);
        return;
      }
      conn.send(cmd);
    };
    send();
  }

  function startListen() {
    if (listening.value || fullScanning.value) return;
    listening.value = true;
    fullScanPhase.value = 'idle';
    startedAt.value = new Date();
    signals.value = [];
    bandSummaries.value = [];
    expanded.value = new Set();
    afterConnectStart({ cmd: 'protocolListen', protocolListen: true });
  }

  function stopListen() {
    if (!listening.value) return;
    listening.value = false;
    closeConnection();
  }

  function startFullScan() {
    if (listening.value || fullScanning.value) return;
    fullScanning.value = true;
    fullScanPhase.value = 'scanning';
    listening.value = false;
    startedAt.value = new Date();
    signals.value = [];
    bandSummaries.value = [];
    expanded.value = new Set();
    scanProgress.value = { bandIdx: 0, bandTotal: 0, bandName: '', pct: 0, phase: 'scanning' };
    display?.resetWaterfall();
    afterConnectStart({ cmd: 'protocolFullScan', protocolFullScan: true });
  }

  function stopFullScan() {
    if (!fullScanning.value) return;
    fullScanning.value = false;
    fullScanPhase.value = bandSummaries.value.length ? 'done' : 'idle';
    if (connected.value && conn) {
      conn.send({ cmd: 'protocolFullScan', protocolFullScan: false });
      conn.send({ cmd: 'service', service: 'radio' });
    }
    conn?.close();
    conn = null;
    connected.value = false;
    statusText.value = '未连接';
  }

  function toggleListen() {
    if (listening.value) stopListen();
    else startListen();
  }

  function bindFullScanDisplay(waterfallEl, spectrumEl, waveformEl) {
    if (!waterfallEl || !spectrumEl || !waveformEl) return () => {};
    display = new Display(waterfallEl, spectrumEl, () => displayState);
    waveform = createWaveform(waveformEl);
    const onResize = () => {
      display?.resize();
      waveform?.resize();
    };
    window.addEventListener('resize', onResize);
    requestAnimationFrame(onResize);
    return () => {
      window.removeEventListener('resize', onResize);
      display = null;
      waveform = null;
    };
  }

  onUnmounted(() => {
    stopListen();
    stopFullScan();
  });

  const state = {
    listening,
    fullScanning,
    fullScanPhase,
    startedAt,
    signals,
    bandSummaries,
    filters,
    filteredSignals,
    counts,
    totalCount,
    expanded,
    connected,
    receiverEnabled,
    statusText,
    scanBand,
    scanProgress,
    showScanPanel,
    toggleExpand,
    startListen,
    stopListen,
    toggleListen,
    startFullScan,
    stopFullScan,
    bindFullScanDisplay,
    typeLabels: TYPE_LABELS,
    groupOrder: GROUP_ORDER,
  };

  provide(PROTOCOL_KEY, state);
  return state;
}

export function useProtocolScan() {
  const state = inject(PROTOCOL_KEY);
  if (!state) throw new Error('useProtocolScan() must be used within provideProtocolScan()');
  return state;
}
