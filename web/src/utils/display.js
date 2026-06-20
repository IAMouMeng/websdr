// Spectrum + waterfall rendering. Network receipt is decoupled from drawing:
// frames are queued and flushed on a requestAnimationFrame loop, and the
// spectrum trace is smoothed per-bin so motion stays fluid between updates.
//
// The backend sends each bin as a byte quantizing real dB over [DB_MIN, DB_MAX]
// (kept in sync with internal/receiver specDBMin/specDBMax). The vertical range
// is framed by user min/max (dB zoom); the horizontal range is framed by a zoom
// window centered on the tuned frequency.

const DB_MIN = -120;
const DB_MAX = 0;
const DB_SPAN = DB_MAX - DB_MIN;
const X_TICK_HZ = 200000; // 0.2 MHz frequency ticks

const axisLayout = () => {
  const dpr = window.devicePixelRatio || 1;
  return {
    axisW: Math.round(64 * dpr),
    scaleH: Math.round(24 * dpr),
    font: Math.round(11 * dpr),
    tick: Math.round(6 * dpr),
    pad: Math.round(4 * dpr),
  };
};

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

export class Display {
  // getState() must return
  // { centerFreq, tuneFreq, sampleRate, filterBW, specMin, specMax, zoom, hideDbAxis? }.
  constructor(waterfall, spectrum, getState) {
    this.wf = waterfall || null;
    this.sp = spectrum;
    this.wfCtx = this.wf ? this.wf.getContext('2d') : null;
    this.spCtx = spectrum.getContext('2d');
    this.getState = getState;

    this.wfPixels = null;
    this.rows = [];
    this.latest = null;
    this.smoothDb = null;

    this.resize();
    const tick = () => {
      this._render();
      requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
  }

  pushSpectrum(data) {
    this.latest = data;
    if (!this.wf) return;
    this.rows.push(data);
    if (this.rows.length > 8) this.rows.splice(0, this.rows.length - 8);
  }

  resetWaterfall() {
    this.rows = [];
    this.latest = null;
    this.smoothDb = null;
    this.wfPixels = null;
    if (this.wfCtx && this.wf) {
      this.wfCtx.fillStyle = '#000';
      this.wfCtx.fillRect(0, 0, this.wf.width, this.wf.height);
    }
  }

  resize() {
    const dpr = window.devicePixelRatio || 1;
    for (const c of (this.wf ? [this.wf, this.sp] : [this.sp])) {
      const box = c.parentElement.getBoundingClientRect();
      c.width = Math.max(1, Math.floor(box.width * dpr));
      c.height = Math.max(1, Math.floor(box.height * dpr));
    }
    if (!this.wfCtx) return;
    this.wfPixels = null;
    this.wfCtx.fillStyle = '#000';
    this.wfCtx.fillRect(0, 0, this.wf.width, this.wf.height);
    const s = this.getState();
    const win = this._window(s);
    const w = this.wf.width, h = this.wf.height;
    const { plotX, plotW, plotH, ax } = this._plotMetrics(w, h, false);
    this._drawWaterfallOverlay(s, win, w, h, plotX, plotW, plotH, ax);
  }

  // _window returns the visible horizontal frequency window given the zoom,
  // centered on the tuned frequency and clamped to the captured band, so the
  // tuned frequency stays at the middle of the view when zoomed in.
  _window(s) {
    const full = s.sampleRate;
    const fullLo = s.centerFreq - full / 2;
    const zoom = s.zoom > 1 ? s.zoom : 1;
    const span = full / zoom;
    let lo = s.tuneFreq - span / 2;
    if (lo < fullLo) lo = fullLo;
    if (lo + span > fullLo + full) lo = fullLo + full - span;
    return { fullLo, full, lo, span };
  }

  // xFracToFreq maps a 0..1 horizontal fraction (clamped) to a frequency.
  xFracToFreq(frac) {
    const f = frac < 0 ? 0 : frac > 1 ? 1 : frac;
    const { lo, span } = this._window(this.getState());
    return lo + f * span;
  }

  // canvasXToFreq maps a canvas pixel x (CSS coords scaled to backing store) to frequency.
  canvasXToFreq(canvasX) {
    const w = this.sp.width;
    const { plotX, plotW } = this._plotMetrics(w, this.sp.height, true);
    const frac = (canvasX - plotX) / plotW;
    return this.xFracToFreq(frac);
  }

  // freqToCanvasX is the inverse of canvasXToFreq: frequency to a backing-store
  // pixel x. Used to locate the filter-band edges for edge dragging.
  freqToCanvasX(freq) {
    const w = this.sp.width;
    const { plotX, plotW } = this._plotMetrics(w, this.sp.height, true);
    return this._freqToX(freq, plotW, this._window(this.getState()), plotX);
  }

  _freqToX(freq, plotW, win, plotX = 0) {
    return plotX + ((freq - win.lo) / win.span) * plotW;
  }

  _plotMetrics(w, h, withBottomScale = true) {
    const ax = axisLayout();
    const bottom = withBottomScale ? ax.scaleH : 0;
    const hideY = !!this.getState().hideDbAxis;
    const plotX = hideY ? 0 : ax.axisW;
    return { plotX, plotW: w - plotX, plotH: h - bottom, ax };
  }

  _norm(db, s) {
    const v = (db - s.specMin) / (s.specMax - s.specMin);
    return v < 0 ? 0 : v > 1 ? 1 : v;
  }

  _render() {
    const s = this.getState();
    const win = this._window(s);
    while (this.rows.length) this._scrollRow(this.rows.shift(), s, win);
    if (this.latest) this._drawSpectrum(this.latest, s, win);
    if (this.wfPixels) {
      const w = this.wf.width, h = this.wf.height;
      const { plotX, plotW, plotH, ax } = this._plotMetrics(w, h, false);
      this.wfCtx.putImageData(this.wfPixels, 0, 0);
      this._drawWaterfallOverlay(s, win, w, h, plotX, plotW, plotH, ax);
    }
  }

  _ensureBuffer() {
    const w = this.wf.width, h = this.wf.height;
    if (!this.wfPixels || this.wfPixels.width !== w || this.wfPixels.height !== h) {
      this.wfPixels = this.wfCtx.createImageData(w, h);
    }
  }

  _scrollRow(data, s, win) {
    this._ensureBuffer();
    const w = this.wf.width, h = this.wf.height;
    const { plotX, plotW, plotH, ax } = this._plotMetrics(w, h, false);
    const px = this.wfPixels.data;
    const rowBytes = w * 4;

    // Scroll only the plot region; axis gutters stay fixed.
    for (let y = plotH - 1; y > 0; y--) {
      const dst = y * rowBytes + plotX * 4;
      const src = (y - 1) * rowBytes + plotX * 4;
      px.copyWithin(dst, src, src + plotW * 4);
    }

    const range = s.specMax - s.specMin;
    const n = data.length;
    for (let x = 0; x < plotW; x++) {
      const freq = win.lo + (x / plotW) * win.span;
      let bin = (((freq - win.fullLo) / win.full) * n) | 0;
      if (bin < 0) bin = 0; else if (bin >= n) bin = n - 1;
      let v = (byteToDb(data[bin]) - s.specMin) / range;
      if (v < 0) v = 0; else if (v > 1) v = 1;
      const li = ((v * 255) | 0) * 3;
      const off = plotX * 4 + x * 4;
      px[off] = WF_LUT[li];
      px[off + 1] = WF_LUT[li + 1];
      px[off + 2] = WF_LUT[li + 2];
      px[off + 3] = 255;
    }
    this.wfCtx.putImageData(this.wfPixels, 0, 0);

    this._drawWaterfallOverlay(s, win, w, h, plotX, plotW, plotH, ax);
  }

  _drawWaterfallOverlay(s, win, w, h, plotX, plotW, plotH, ax) {
    const ctx = this.wfCtx;
    if (!s.hideDbAxis) {
      ctx.fillStyle = '#0d0d0d';
      ctx.fillRect(0, 0, plotX, plotH);
    }

    const tx = this._freqToX(s.tuneFreq, plotW, win, plotX);
    ctx.strokeStyle = 'rgba(255, 80, 80, 0.55)';
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(tx, 0);
    ctx.lineTo(tx, plotH);
    ctx.stroke();

    ctx.stroke();

    this._drawAxisFrame(ctx, plotX, plotW, plotH, h, false);
  }

  _binAtFreq(freq, win, n) {
    let bin = (((freq - win.fullLo) / win.full) * n) | 0;
    if (bin < 0) bin = 0;
    else if (bin >= n) bin = n - 1;
    return bin;
  }

  _drawSpectrum(data, s, win) {
    const ctx = this.spCtx;
    const w = this.sp.width, h = this.sp.height;
    const { plotX, plotW, plotH, ax } = this._plotMetrics(w, h, true);
    const n = data.length;

    if (!this.smoothDb || this.smoothDb.length !== n) {
      this.smoothDb = new Float32Array(n);
      for (let i = 0; i < n; i++) this.smoothDb[i] = byteToDb(data[i]);
    } else {
      const a = 0.35;
      for (let i = 0; i < n; i++) {
        this.smoothDb[i] += (byteToDb(data[i]) - this.smoothDb[i]) * a;
      }
    }
    const sm = this.smoothDb;

    ctx.fillStyle = '#000';
    ctx.fillRect(0, 0, w, h);

    const yAtBin = (bin) => plotH - this._norm(sm[bin], s) * plotH;

    // Sample one point per screen column so panning never draws spurious diagonals.
    ctx.beginPath();
    ctx.moveTo(plotX, plotH);
    for (let x = 0; x <= plotW; x++) {
      const freq = win.lo + (x / plotW) * win.span;
      ctx.lineTo(plotX + x, yAtBin(this._binAtFreq(freq, win, n)));
    }
    ctx.lineTo(plotX + plotW, plotH);
    ctx.closePath();
    const grad = ctx.createLinearGradient(0, 0, 0, plotH);
    grad.addColorStop(0, 'rgba(180, 180, 180, 0.22)');
    grad.addColorStop(1, 'rgba(60, 60, 60, 0.02)');
    ctx.fillStyle = grad;
    ctx.fill();

    this._drawPlotGrid(ctx, plotX, plotW, plotH, s, win);

    ctx.strokeStyle = 'rgba(170, 180, 190, 0.75)';
    ctx.lineWidth = 1.5;
    ctx.beginPath();
    for (let x = 0; x <= plotW; x++) {
      const freq = win.lo + (x / plotW) * win.span;
      const px = plotX + x;
      const py = yAtBin(this._binAtFreq(freq, win, n));
      if (x === 0) ctx.moveTo(px, py);
      else ctx.lineTo(px, py);
    }
    ctx.stroke();

    if (!s.hideDbAxis) this._drawDbAxis(ctx, w, plotH, s, ax);
    this._drawFreqScale(ctx, w, plotH, h, win, plotX, plotW, ax);
    this._drawAxisFrame(ctx, plotX, plotW, plotH, h, true);
    this._drawTuneMarker(ctx, plotW, plotH, s, win, plotX);
  }

  _drawPlotGrid(ctx, plotX, plotW, plotH, s, win) {
    const range = s.specMax - s.specMin;
    const raw = range / 5;
    const nice = [5, 10, 20, 25, 50, 100];
    let dbStep = nice[nice.length - 1];
    for (const v of nice) if (v >= raw) { dbStep = v; break; }

    ctx.strokeStyle = 'rgba(255,255,255,0.12)';
    ctx.lineWidth = 1;
    const startDb = Math.ceil(s.specMin / dbStep) * dbStep;
    for (let db = startDb; db <= s.specMax; db += dbStep) {
      const y = plotH - this._norm(db, s) * plotH;
      ctx.beginPath();
      ctx.moveTo(plotX, y);
      ctx.lineTo(plotX + plotW, y);
      ctx.stroke();
    }

    const hi = win.lo + win.span;
    const first = Math.ceil(win.lo / X_TICK_HZ) * X_TICK_HZ;
    for (let f = first; f <= hi; f += X_TICK_HZ) {
      const x = this._freqToX(f, plotW, win, plotX);
      ctx.beginPath();
      ctx.moveTo(x, 0);
      ctx.lineTo(x, plotH);
      ctx.stroke();
    }
  }

  _drawAxisFrame(ctx, plotX, plotW, plotH, h, withBottomScale = true) {
    ctx.strokeStyle = 'rgba(255,255,255,0.18)';
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(plotX, 0);
    ctx.lineTo(plotX, plotH);
    ctx.lineTo(plotX + plotW, plotH);
    ctx.stroke();
    if (withBottomScale) {
      ctx.fillStyle = '#0d0d0d';
      ctx.fillRect(0, plotH, plotX, h - plotH);
    }
  }

  _drawDbAxis(ctx, w, plotH, s, ax) {
    const range = s.specMax - s.specMin;
    const raw = range / 5;
    const nice = [5, 10, 20, 25, 50, 100];
    let dbStep = nice[nice.length - 1];
    for (const v of nice) if (v >= raw) { dbStep = v; break; }

    ctx.fillStyle = '#0d0d0d';
    ctx.fillRect(0, 0, ax.axisW, plotH);

    const unitBand = ax.font + ax.pad * 2;
    ctx.font = `${ax.font}px monospace`;
    ctx.textAlign = 'left';
    ctx.textBaseline = 'top';
    ctx.fillStyle = '#999';
    ctx.fillText('dB', ax.pad, ax.pad);

    const labelX = ax.axisW - ax.tick - ax.pad;
    ctx.textAlign = 'right';
    ctx.textBaseline = 'middle';
    const minLabelY = unitBand + ax.font * 0.45;
    const maxLabelY = plotH - ax.font * 0.45;
    const start = Math.ceil(s.specMin / dbStep) * dbStep;
    for (let db = start; db <= s.specMax; db += dbStep) {
      const y = plotH - this._norm(db, s) * plotH;
      ctx.strokeStyle = 'rgba(255,255,255,0.22)';
      ctx.lineWidth = 1;
      ctx.beginPath();
      ctx.moveTo(ax.axisW - ax.tick, y);
      ctx.lineTo(ax.axisW, y);
      ctx.stroke();
      if (y >= minLabelY && y <= maxLabelY) {
        ctx.fillStyle = '#bbb';
        ctx.fillText(String(db), labelX, y);
      }
    }
  }

  _drawFreqScale(ctx, w, plotH, h, win, plotX, plotW, ax) {
    ctx.fillStyle = '#0d0d0d';
    ctx.fillRect(0, plotH, w, h - plotH);
    ctx.font = `${ax.font}px monospace`;
    ctx.textBaseline = 'top';

    const y = plotH + ax.pad;
    const leftBound = plotX + ax.pad;
    const rightBound = w - ax.pad;

    // Unit label sits in the left gutter so it never collides with tick numbers.
    ctx.textAlign = 'left';
    ctx.fillStyle = '#999';
    ctx.fillText('MHz', ax.pad, y);

    const hi = win.lo + win.span;
    const first = Math.ceil(win.lo / X_TICK_HZ) * X_TICK_HZ;
    let lastRight = ax.pad + ctx.measureText('MHz').width;

    for (let f = first; f <= hi; f += X_TICK_HZ) {
      const x = this._freqToX(f, plotW, win, plotX);
      ctx.strokeStyle = 'rgba(255,255,255,0.22)';
      ctx.lineWidth = 1;
      ctx.beginPath();
      ctx.moveTo(x, plotH);
      ctx.lineTo(x, plotH + ax.tick);
      ctx.stroke();

      const label = (f / 1e6).toFixed(2);
      const labelW = ctx.measureText(label).width;
      let drawX = x;
      let align = 'center';

      if (x + labelW / 2 > rightBound) {
        align = 'right';
        drawX = rightBound;
      } else if (x - labelW / 2 < leftBound) {
        align = 'left';
        drawX = leftBound;
      }

      const labelLeft = align === 'left' ? drawX : align === 'right' ? drawX - labelW : drawX - labelW / 2;
      if (labelLeft < lastRight + ax.pad) continue;

      ctx.textAlign = align;
      ctx.fillStyle = '#bbb';
      ctx.fillText(label, drawX, y);
      lastRight = labelLeft + labelW;
    }
  }

  _drawTuneMarker(ctx, plotW, plotH, s, win, plotX) {
    const tx = this._freqToX(s.tuneFreq, plotW, win, plotX);
    const halfW = (s.filterBW / win.span) * plotW / 2;
    ctx.fillStyle = 'rgba(255, 80, 80, 0.08)';
    ctx.fillRect(tx - halfW, 0, halfW * 2, plotH);

    ctx.strokeStyle = '#e44';
    ctx.lineWidth = 1.5;
    ctx.beginPath();
    ctx.moveTo(tx, 0);
    ctx.lineTo(tx, plotH);
    ctx.stroke();
  }
}

export function fmtFreq(hz) {
  if (hz >= 1e6) return (hz / 1e6).toFixed(3) + ' MHz';
  if (hz >= 1e3) return (hz / 1e3).toFixed(1) + ' kHz';
  return hz.toFixed(0) + ' Hz';
}
