import { ref, reactive, onMounted, onUnmounted } from 'vue';
import { connect } from '@/utils/protocol.js';
import { Display } from '@/utils/display.js';
import { createWaveform } from '@/utils/waveform.js';

const LRPT_CENTER = 137_900_000;
const LRPT_SAMPLE_RATE = 2_048_000;
export const LRPT_DEFAULT_FILTER = 150_000;

export function useLRPTListen(freqHz) {
  const lrpt = ref({
    freqHz: freqHz || LRPT_CENTER,
    freq: '',
    strength: null,
    metric: '',
    locked: false,
    listening: false,
    elapsedSec: 0,
  });
  const statusText = ref('连接中…');
  const connected = ref(false);
  const gain = ref(20);
  const agc = ref(false);

  const displayState = reactive({
    centerFreq: LRPT_CENTER,
    tuneFreq: freqHz || LRPT_CENTER,
    sampleRate: LRPT_SAMPLE_RATE,
    filterBW: LRPT_DEFAULT_FILTER,
    specMin: -110,
    specMax: -20,
    zoom: 2,
  });

  let conn = null;
  let display = null;
  let waveform = null;

  function stop() {
    if (!conn) return;
    if (connected.value) {
      conn.send({ cmd: 'lrptListen', lrptListen: false });
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

  onMounted(() => {
    conn = connect({
      onOpen: () => {
        connected.value = true;
        statusText.value = '监听中';
        conn.send({ cmd: 'service', service: 'lrpt' });
        conn.send({ cmd: 'gain', gain: gain.value });
        conn.send({ cmd: 'agc', agc: agc.value });
        conn.send({ cmd: 'filter', filterBW: displayState.filterBW });
        conn.send({ cmd: 'lrptListen', lrptListen: true, freq: freqHz || LRPT_CENTER });
      },
      onClose: () => {
        connected.value = false;
        statusText.value = '已断开';
        lrpt.value = { ...lrpt.value, listening: false };
      },
      onMessage: (msg) => {
        if (msg.type !== 'lrpt' || !msg.lrpt) return;
        lrpt.value = msg.lrpt;
        if (msg.lrpt.freqHz) displayState.tuneFreq = msg.lrpt.freqHz;
      },
      onSpectrum: (centerFreq, bins) => {
        displayState.centerFreq = centerFreq;
        displayState.sampleRate = LRPT_SAMPLE_RATE;
        display?.pushSpectrum(bins);
      },
      onAudio: (pcm) => waveform?.push(pcm),
    });
  });

  onUnmounted(stop);

  return {
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
  };
}
