// Continuous full-sweep waterfall: X = 24 MHz–1.7 GHz, rows scroll downward.
// Band hops append rows at the correct frequency columns without clearing.

const DB_MIN = -120;
const DB_MAX = 0;
const DB_SPAN = DB_MAX - DB_MIN;

const byteToDb = (b) => DB_MIN + (b / 255) * DB_SPAN;

const WF_LUT = (() => {
  const lut = new Uint8ClampedArray(256 * 3);
  for (let i = 0; i < 256; i++) {
    lut[i * 3] = 0;
    lut[i * 3 + 1] = Math.min(255, Math.floor(i * 0.4));
    lut[i * 3 + 2] = i;
  }
  return lut;
})();

const axisLayout = () => {
  const dpr = window.devicePixelRatio || 1;
  return {
    axisW: Math.round(48 * dpr),
    scaleH: Math.round(20 * dpr),
    font: Math.round(11 * dpr),
    tick: Math.round(5 * dpr),
    pad: Math.round(3 * dpr),
  };
};

export class FullSweepDisplay {
  constructor(canvas, getState) {
    this.wf = canvas;
    this.wfCtx = canvas.getContext('2d');
    this.getState = getState;
    this.wfPixels = null;
    this.latestCenter = 0;

    this.resize();
    const tick = () => {
      this._drawOverlay();
      requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
  }

  pushSpectrum(centerFreq, bins, sampleRate) {
    if (!bins?.length || !sampleRate) return;
    this.latestCenter = centerFreq;
    this._scrollRow(centerFreq, bins, sampleRate);
  }

  reset() {
    this.wfPixels = null;
    this.latestCenter = 0;
    if (this.wfCtx && this.wf) {
      this.wfCtx.fillStyle = '#000';
      this.wfCtx.fillRect(0, 0, this.wf.width, this.wf.height);
    }
  }

  resize() {
    const dpr = window.devicePixelRatio || 1;
    const box = this.wf.parentElement.getBoundingClientRect();
    this.wf.width = Math.max(1, Math.floor(box.width * dpr));
    this.wf.height = Math.max(1, Math.floor(box.height * dpr));
    this.wfPixels = null;
    this.wfCtx.fillStyle = '#000';
    this.wfCtx.fillRect(0, 0, this.wf.width, this.wf.height);
  }

  _metrics() {
    const w = this.wf.width;
    const h = this.wf.height;
    const ax = axisLayout();
    const plotX = ax.axisW;
    const plotW = w - ax.axisW;
    const plotH = h - ax.scaleH;
    return { w, h, ax, plotX, plotW, plotH };
  }

  _ensureBuffer(w, h) {
    if (!this.wfPixels || this.wfPixels.width !== w || this.wfPixels.height !== h) {
      this.wfPixels = this.wfCtx.createImageData(w, h);
    }
  }

  _scrollRow(centerFreq, bins, sampleRate) {
    const s = this.getState();
    const { w, h, ax, plotX, plotW, plotH } = this._metrics();
    this._ensureBuffer(w, h);
    const px = this.wfPixels.data;
    const rowBytes = w * 4;

    for (let y = plotH - 1; y > 0; y--) {
      const dst = y * rowBytes + plotX * 4;
      const src = (y - 1) * rowBytes + plotX * 4;
      px.copyWithin(dst, src, src + plotW * 4);
    }

    const sweepLo = s.sweepLoHz;
    const sweepHi = s.sweepHiHz;
    const sweepSpan = sweepHi - sweepLo;
    const range = s.specMax - s.specMin;
    const n = bins.length;
    const chunkLo = centerFreq - sampleRate / 2;

    // Clear new row slice to black first.
    for (let x = 0; x < plotW; x++) {
      const off = plotX * 4 + x * 4;
      px[off] = 0;
      px[off + 1] = 0;
      px[off + 2] = 0;
      px[off + 3] = 255;
    }

    for (let bi = 0; bi < n; bi++) {
      const freq = chunkLo + (bi / n) * sampleRate;
      if (freq < sweepLo || freq > sweepHi) continue;
      const col = ((freq - sweepLo) / sweepSpan) * plotW;
      const x0 = Math.floor(col);
      const x1 = Math.ceil(col);
      let v = (byteToDb(bins[bi]) - s.specMin) / range;
      if (v < 0) v = 0;
      else if (v > 1) v = 1;
      const li = ((v * 255) | 0) * 3;
      for (const x of [x0, x1]) {
        if (x < 0 || x >= plotW) continue;
        const off = plotX * 4 + x * 4;
        px[off] = WF_LUT[li];
        px[off + 1] = WF_LUT[li + 1];
        px[off + 2] = WF_LUT[li + 2];
      }
    }

    this.wfCtx.putImageData(this.wfPixels, 0, 0);
    this._drawOverlay(ax, plotX, plotW, plotH, w, h, sweepLo, sweepHi, sweepSpan);
  }

  _drawOverlay(ax, plotX, plotW, plotH, w, h, sweepLo, sweepHi, sweepSpan) {
    if (!this.wfPixels) return;
    const s = this.getState();
    this.wfCtx.putImageData(this.wfPixels, 0, 0);

    this.wfCtx.fillStyle = '#0d0d0d';
    this.wfCtx.fillRect(0, 0, plotX, plotH);

    if (this.latestCenter >= sweepLo && this.latestCenter <= sweepHi) {
      const tx = plotX + ((this.latestCenter - sweepLo) / sweepSpan) * plotW;
      this.wfCtx.strokeStyle = 'rgba(255, 80, 80, 0.45)';
      this.wfCtx.lineWidth = 1;
      this.wfCtx.beginPath();
      this.wfCtx.moveTo(tx, 0);
      this.wfCtx.lineTo(tx, plotH);
      this.wfCtx.stroke();
    }

    this.wfCtx.strokeStyle = 'rgba(255,255,255,0.15)';
    this.wfCtx.lineWidth = 1;
    this.wfCtx.beginPath();
    this.wfCtx.moveTo(plotX, 0);
    this.wfCtx.lineTo(plotX, plotH);
    this.wfCtx.lineTo(plotX + plotW, plotH);
    this.wfCtx.stroke();

    this.wfCtx.fillStyle = '#0d0d0d';
    this.wfCtx.fillRect(0, plotH, w, h - plotH);
    this.wfCtx.font = `${ax.font}px monospace`;
    this.wfCtx.textBaseline = 'top';
    this.wfCtx.fillStyle = '#777';
    this.wfCtx.textAlign = 'left';
    this.wfCtx.fillText('MHz', ax.pad, plotH + ax.pad);

    const tickMHz = 100_000_000;
    const first = Math.ceil(sweepLo / tickMHz) * tickMHz;
    for (let f = first; f <= sweepHi; f += tickMHz) {
      const x = plotX + ((f - sweepLo) / sweepSpan) * plotW;
      this.wfCtx.strokeStyle = 'rgba(255,255,255,0.18)';
      this.wfCtx.beginPath();
      this.wfCtx.moveTo(x, plotH);
      this.wfCtx.lineTo(x, plotH + ax.tick);
      this.wfCtx.stroke();
      this.wfCtx.textAlign = 'center';
      this.wfCtx.fillStyle = '#888';
      this.wfCtx.fillText((f / 1e6).toFixed(0), x, plotH + ax.tick + ax.pad);
    }

    const pct = s.scanPct ?? 0;
    this.wfCtx.textAlign = 'right';
    this.wfCtx.fillStyle = '#6a6';
    this.wfCtx.fillText(`${pct.toFixed(0)}%`, plotX + plotW - ax.pad, ax.pad);
  }
}
