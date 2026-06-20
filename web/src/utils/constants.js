export const LS_KEY = 'websdr-settings';

export const MODE_FILTER_HZ = {
  wfm: 150000,
  fm: 12500,
  am: 6000,
  usb: 2400,
  lsb: 2400,
  dsb: 6000,
  cw: 500,
  raw: 12500,
};

export const MAX_FREQ = 1800000000;
export const MIN_FREQ_NORMAL = 24_000_000;
export const HF_MIN_FREQ = 100_000;
export const HF_MAX_FREQ = 28_800_000;
export const BW_MIN = 100;
export const BW_MAX = 250000;
export const DIAL_DIGITS = 12;

export const MODES = [
  { value: 'wfm', label: 'WFM' },
  { value: 'fm', label: 'NFM' },
  { value: 'am', label: 'AM' },
  { value: 'usb', label: 'USB' },
  { value: 'lsb', label: 'LSB' },
  { value: 'dsb', label: 'DSB' },
  { value: 'cw', label: 'CW' },
  { value: 'raw', label: 'RAW' },
];

export const SAMPLE_RATES = [
  250000, 960000, 1024000, 1200000, 1536000, 1600000,
  1792000, 1920000, 2048000, 2160000, 2400000, 2560000,
  2880000, 3200000,
];

export const SPEC_MAX_RANGE = { min: -60, max: 10, step: 1 };
export const SPEC_MIN_RANGE = { min: -140, max: -30, step: 1 };
export const ZOOM_RANGE = { min: 1, max: 32, step: 0.5 };

export function fmtSampleRate(hz) {
  if (hz >= 1e6) return `${(hz / 1e6).toFixed(3)} MHz`;
  return `${(hz / 1e3).toFixed(0)} kHz`;
}

export function fmtZoom(z) {
  return (z % 1 === 0 ? String(z) : z.toFixed(1)) + '×';
}

export function clampFreq(f) {
  const hz = Math.round(f);
  if (hz < MIN_FREQ_NORMAL) {
    return Math.max(HF_MIN_FREQ, Math.min(HF_MAX_FREQ, hz));
  }
  return Math.max(MIN_FREQ_NORMAL, Math.min(MAX_FREQ, hz));
}

export function snapVal(v, range) {
  v = Math.max(range.min, Math.min(range.max, v));
  if (range.step >= 1) return Math.round(v);
  return Math.round(v / range.step) * range.step;
}
