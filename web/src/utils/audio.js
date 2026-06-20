// WebSocket PCM → AudioWorklet (audio thread). ScriptProcessor fallback only
// if the worklet fails to load.

import { WebRtcAudio } from './webrtc-audio.js';
import { wsHost } from './protocol.js';

const AUDIO_IMPL = 'worklet-ws-v3';
const SRC_RATE = 48000;

function workletUrl() {
  const base = import.meta.env?.BASE_URL ?? '/';
  return new URL(`${base}worklet.js`, location.origin).href;
}

function pcmPeak(pcm) {
  let peak = 0;
  for (let i = 0; i < pcm.length; i++) {
    const a = Math.abs(pcm[i]);
    if (a > peak) peak = a;
  }
  return peak;
}

function resample(pcm, fromRate, toRate) {
  if (fromRate === toRate || pcm.length === 0) return pcm;
  const outLen = Math.max(1, Math.round(pcm.length * toRate / fromRate));
  const out = new Float32Array(outLen);
  const ratio = fromRate / toRate;
  for (let i = 0; i < outLen; i++) {
    const pos = i * ratio;
    const idx = pos | 0;
    const frac = pos - idx;
    const a = pcm[idx] ?? 0;
    const b = pcm[Math.min(idx + 1, pcm.length - 1)];
    out[i] = a + (b - a) * frac;
  }
  return out;
}

export class AudioPlayer {
  constructor(rate = SRC_RATE) {
    this.rate = rate;
    this.transport = 'none';
    this.mode = null;

    this.ctx = null;
    this.gain = null;
    this.analyser = null;
    this._levelBuf = null;
    this.node = null;

    this._sp = null;
    this._keepalive = null;
    this._ring = null;
    this._cap = 0;
    this._r = 0;
    this._w = 0;
    this._n = 0;
    this._primed = false;
    this._prebuffer = 0;
    this._prebufferNormal = 0;
    this._prebufferFast = 0;

    this._pending = [];
    this._pendingSamples = 0;
    this._pendingMax = rate * 3;

    this._webrtc = new WebRtcAudio();
    this._outputReady = null;

    this._playing = false;
    this._volume = 0.8;
    this.onError = null;
    this.onTransportChange = null;

    this._pushCount = 0;
    this._dropCount = 0;
    this._pcmPeak = 0;
    this._pcmPeakMax = 0;
    this._extraStats = null;
  }

  get playing() {
    return this._playing;
  }

  unlockFromGesture() {
    if (this.transport === 'webrtc') return;
    if (!this.ctx) {
      try {
        this.ctx = new AudioContext({ sampleRate: this.rate });
      } catch {
        this.ctx = new AudioContext();
      }
      this.gain = this.ctx.createGain();
      this.gain.gain.value = this._volume;
      this.analyser = this.ctx.createAnalyser();
      this.analyser.fftSize = 1024;
      this.gain.connect(this.analyser).connect(this.ctx.destination);
      this._levelBuf = new Float32Array(this.analyser.fftSize);
    }
    if (this.ctx.state === 'suspended') void this.ctx.resume();
    try {
      const ping = this.ctx.createBuffer(1, 1, this.ctx.sampleRate);
      const src = this.ctx.createBufferSource();
      src.buffer = ping;
      src.connect(this.ctx.destination);
      src.start(0);
    } catch { /* unlock ping */ }
  }

  async start() {
    this.unlockFromGesture();
    this._playing = true;
    this._pcmPeakMax = 0;
    this.transport = 'ws';
    this.onTransportChange?.('ws');

    try {
      if (this.ctx.state === 'suspended') await this.ctx.resume();
      const queued = this._pending;
      const queuedSamples = this._pendingSamples;
      this._pending = [];
      this._pendingSamples = 0;
      await this._ensureOutput();
      this.reset();
      this._pending = queued;
      this._pendingSamples = queuedSamples;
      this._flushPending();
    } catch (err) {
      this._playing = false;
      this.onError?.(String(err?.message ?? err));
      throw err;
    }
  }

  async _ensureOutput() {
    if (this.node || this._sp) return;
    if (this._outputReady) {
      await this._outputReady;
      return;
    }
    this._outputReady = this._initOutput();
    await this._outputReady;
  }

  async _initOutput() {
    const ctx = this.ctx;
    if (!ctx) throw new Error('AudioContext not ready');

    if (ctx.audioWorklet) {
      try {
        await ctx.audioWorklet.addModule(workletUrl());
        const node = new AudioWorkletNode(ctx, 'pcm-player', {
          numberOfInputs: 0,
          numberOfOutputs: 1,
          outputChannelCount: [1],
        });
        node.connect(this.gain);
        this.node = node;
        this.mode = 'worklet';
        return;
      } catch (err) {
        console.warn('[WebSDR] AudioWorklet unavailable, using ScriptProcessor', err);
      }
    }

    this._attachScriptProcessor(ctx);
    this.mode = 'script';
  }

  _attachScriptProcessor(ctx) {
    const sr = ctx.sampleRate;
    this._cap = Math.floor(sr * 4);
    this._ring = new Float32Array(this._cap);
    this._r = this._w = this._n = 0;
    this._primed = false;
    this._prebufferNormal = Math.floor(sr * 0.08);
    this._prebufferFast = Math.floor(sr * 0.03);
    this._prebuffer = this._prebufferNormal;

    const sp = ctx.createScriptProcessor(2048, 1, 1);
    const silent = ctx.createGain();
    silent.gain.value = 0;
    const src = typeof ctx.createConstantSource === 'function'
      ? ctx.createConstantSource()
      : ctx.createOscillator();
    if (src.offset) src.offset.value = 0;
    src.connect(silent);
    src.start(0);
    silent.connect(sp);
    this._keepalive = src;

    sp.onaudioprocess = (ev) => {
      const out = ev.outputBuffer.getChannelData(0);
      if (!this._primed) {
        if (this._n >= this._prebuffer) this._primed = true;
        else { out.fill(0); return; }
      }
      const cap = this._cap;
      for (let i = 0; i < out.length; i++) {
        if (this._n > 0) {
          out[i] = this._ring[this._r];
          this._r = (this._r + 1) % cap;
          this._n--;
        } else {
          out[i] = 0;
        }
      }
    };
    sp.connect(this.gain);
    this._sp = sp;
  }

  _scaledPcm(pcm) {
    const sr = this.ctx?.sampleRate ?? this.rate;
    return resample(pcm, this.rate, sr);
  }

  _enqueuePending(pcm) {
    const scaled = this._scaledPcm(pcm);
    const n = scaled.length;
    if (this._pendingSamples + n > this._pendingMax) {
      while (this._pending.length && this._pendingSamples + n > this._pendingMax) {
        this._pendingSamples -= this._pending.shift().length;
      }
    }
    this._pending.push(scaled);
    this._pendingSamples += n;
  }

  _flushPending() {
    if (!this._pending.length) return;
    const chunks = this._pending;
    this._pending = [];
    this._pendingSamples = 0;
    for (const pcm of chunks) this._writePcm(pcm);
  }

  _trackPeak(pcm) {
    const peak = pcmPeak(pcm);
    if (peak > this._pcmPeak) this._pcmPeak = peak;
    if (peak > this._pcmPeakMax) this._pcmPeakMax = peak;
  }

  _writePcm(pcm) {
    this._trackPeak(pcm);
    if (this.node) {
      this.node.port.postMessage({ pcm: new Float32Array(pcm) });
      return;
    }
    if (!this._ring) return;
    const cap = this._cap;
    for (let i = 0; i < pcm.length; i++) {
      if (this._n >= cap) {
        this._r = (this._r + 1) % cap;
        this._n--;
      }
      this._ring[this._w] = pcm[i];
      this._w = (this._w + 1) % cap;
      this._n++;
    }
  }

  getLevelDb() {
    if (!this._playing || !this.analyser) return -Infinity;
    this.analyser.getFloatTimeDomainData(this._levelBuf);
    let sum = 0;
    for (let i = 0; i < this._levelBuf.length; i++) {
      const v = this._levelBuf[i];
      sum += v * v;
    }
    const rms = Math.sqrt(sum / this._levelBuf.length);
    return rms > 1e-7 ? 20 * Math.log10(rms) : -Infinity;
  }

  reset(opts = {}) {
    const fast = opts.fast === true;
    this._pending = [];
    this._pendingSamples = 0;
    if (this.node) {
      this.node.port.postMessage({ cmd: 'reset', fast });
    } else {
      this._r = this._w = this._n = 0;
      this._primed = false;
      if (this._prebufferNormal) {
        this._prebuffer = fast ? this._prebufferFast : this._prebufferNormal;
      }
    }
  }

  resetForTune() {
    if (!this._playing) return;
    this.reset({ fast: true });
  }

  push(pcm) {
    if (!pcm?.length) return;
    if (this.transport === 'webrtc') {
      this._trackPeak(pcm);
      return;
    }
    if (!this._playing) {
      this._dropCount++;
      this._enqueuePending(pcm);
      return;
    }
    this._pushCount++;
    this._writePcm(this._scaledPcm(pcm));
  }

  setVolume(v) {
    this._volume = Math.max(0, Math.min(1, v));
    if (this.gain) this.gain.gain.value = this._volume;
    this._webrtc.setVolume(this._volume);
  }

  async stop() {
    this._playing = false;
    this._pending = [];
    this._pendingSamples = 0;
    this._pcmPeak = 0;
    this._outputReady = null;
    await this._webrtc.stop();
    this.transport = 'none';
    this.mode = null;
    this.onTransportChange?.('none');
  }

  async forceWebRTC() {
    if (!this._playing) this._playing = true;
    if (this.node) {
      try { this.node.disconnect(); } catch { /* ignore */ }
      this.node = null;
    }
    if (this._sp) {
      try { this._sp.disconnect(); } catch { /* ignore */ }
      this._sp = null;
    }
    await this._webrtc.start();
    this._webrtc.setVolume(this._volume);
    this.transport = 'webrtc';
    this.mode = 'webrtc';
    this.onTransportChange?.('webrtc');
  }

  setExtraStats(fn) {
    this._extraStats = fn;
  }

  debugStats() {
    const base = {
      impl: AUDIO_IMPL,
      mode: this.mode,
      transport: this.transport,
      wsHost: wsHost(),
      host: location.hostname,
      playing: this._playing,
      ctx: this.ctx?.state ?? 'n/a',
      ctxRate: this.ctx?.sampleRate,
      gain: this.gain?.gain.value,
      pushCount: this._pushCount,
      dropCount: this._dropCount,
      ringFill: this._n,
      pcmPeak: this._pcmPeak,
      pcmPeakMax: this._pcmPeakMax,
      pending: this._pendingSamples,
      webrtc: this._webrtc.active,
      db: this.getLevelDb(),
    };
    if (this._extraStats) Object.assign(base, this._extraStats());
    return base;
  }
}

export function installAudioDebug(audio, extraStats) {
  if (extraStats) audio.setExtraStats(extraStats);
  window.__sdrAudioRef = audio;
  window.__sdrAudioStats = () => audio.debugStats();
  window.__sdrForceWebRTC = () => audio.forceWebRTC();
}
