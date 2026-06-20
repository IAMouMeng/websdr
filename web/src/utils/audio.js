// Audio playback pipeline.
//
// Primary path is an AudioWorklet (off the main thread, so heavy canvas
// drawing can't starve playback). The worklet reports "alive" on its first
// render callback; if that confirmation doesn't arrive (worklet failed to
// load, was blocked, or silently isn't running) we transparently switch to a
// ScriptProcessorNode running the same ring-buffer logic. Either way audio
// plays, and any hard failure is surfaced via onError.

// loopbackHost reports whether the page is served from a local loopback name.
function loopbackHost() {
  const h = location.hostname;
  return h === 'localhost' || h === '127.0.0.1' || h === '[::1]';
}

// workletSupported is false on LAN/http origins where AudioWorklet is blocked.
function workletSupported(ctx) {
  return !!(window.isSecureContext && loopbackHost() && ctx.audioWorklet);
}

export class AudioPlayer {
  constructor(rate = 48000) {
    this.rate = rate;
    this.ctx = null;
    this.gain = null;
    this.analyser = null;
    this._levelBuf = null;

    this.node = null; // AudioWorkletNode when the worklet path is active
    this.sp = null;   // ScriptProcessorNode for the fallback path
    this.mode = null; // 'worklet' | 'script'

    // Main-thread ring buffer (fallback path only).
    this._ring = null;
    this._cap = 0;
    this._r = 0;
    this._w = 0;
    this._n = 0;
    this._primed = false;
    this._prebuffer = 0;

    this._aliveSeen = false;
    this._aliveCb = null;
    this._verified = false;

    this._playing = false;
    this._volume = 0.8;
    this._starting = null;
    this.onError = null; // optional callback(message)
  }

  get playing() {
    return this._playing;
  }

  // start is idempotent and safe to call from any user gesture.
  async start() {
    try {
      if (!this.ctx) {
        if (!this._starting) this._starting = this._build();
        await this._starting;
      }
      if (this.ctx.state === 'suspended') await this.ctx.resume();

      // Once the context is running, confirm the worklet actually renders.
      if (this.mode === 'worklet' && !this._verified) {
        this._verified = true;
        const alive = await this._waitAlive(1200);
        if (!alive) {
          console.warn('AudioWorklet not rendering, switching to ScriptProcessor');
          this._switchToFallback();
        }
      }

      this.reset();
      this._playing = true;
    } catch (err) {
      console.error('audio start failed', err);
      this.onError?.(String(err && err.message ? err.message : err));
      throw err;
    }
  }

  async _build() {
    const ctx = new AudioContext({ sampleRate: this.rate });
    const gain = ctx.createGain();
    gain.gain.value = this._volume;
    const analyser = ctx.createAnalyser();
    analyser.fftSize = 1024;
    // gain -> analyser -> destination; the source (worklet or SP) feeds gain.
    gain.connect(analyser).connect(ctx.destination);

    this.ctx = ctx;
    this.gain = gain;
    this.analyser = analyser;
    this._levelBuf = new Float32Array(analyser.fftSize);

    if (workletSupported(ctx)) {
      try {
        await ctx.audioWorklet.addModule(new URL('/worklet.js', location.origin).href);
        const node = new AudioWorkletNode(ctx, 'pcm-player', {
          numberOfInputs: 0,
          numberOfOutputs: 1,
          outputChannelCount: [1],
        });
        node.port.onmessage = (e) => {
          if (e.data && e.data.type === 'alive') {
            this._aliveSeen = true;
            this._aliveCb?.();
          }
        };
        node.connect(gain);
        this.node = node;
        this.mode = 'worklet';
        return;
      } catch (err) {
        console.warn('AudioWorklet unavailable, using ScriptProcessor', err);
      }
    } else if (!loopbackHost()) {
      console.info('Non-loopback HTTP origin: using ScriptProcessor for LAN access');
    }

    this._buildScriptProcessor(ctx).connect(gain);
    this.mode = 'script';
  }

  _waitAlive(timeoutMs) {
    if (this._aliveSeen) return Promise.resolve(true);
    return new Promise((resolve) => {
      let done = false;
      const finish = (v) => {
        if (done) return;
        done = true;
        this._aliveCb = null;
        resolve(v);
      };
      this._aliveCb = () => finish(true);
      setTimeout(() => finish(false), timeoutMs);
    });
  }

  _switchToFallback() {
    if (this.node) {
      try { this.node.disconnect(); } catch { /* ignore */ }
      this.node = null;
    }
    this._buildScriptProcessor(this.ctx).connect(this.gain);
    this.mode = 'script';
  }

  _buildScriptProcessor(ctx) {
    const sr = ctx.sampleRate;
    this._cap = Math.floor(sr * 3);
    this._ring = new Float32Array(this._cap);
    this._r = this._w = this._n = 0;
    this._primed = false;
    this._prebuffer = Math.floor(sr * 0.12);

    const sp = ctx.createScriptProcessor(2048, 0, 1);
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
          this._primed = false;
        }
      }
    };
    this.sp = sp;
    return sp;
  }

  // getLevelDb returns the current output RMS level in dBFS, or -Infinity when
  // silent / not yet started.
  getLevelDb() {
    if (!this.analyser || !this._playing) return -Infinity;
    this.analyser.getFloatTimeDomainData(this._levelBuf);
    let sum = 0;
    for (let i = 0; i < this._levelBuf.length; i++) {
      const v = this._levelBuf[i];
      sum += v * v;
    }
    const rms = Math.sqrt(sum / this._levelBuf.length);
    return rms > 1e-7 ? 20 * Math.log10(rms) : -Infinity;
  }

  reset() {
    if (this.node) {
      this.node.port.postMessage({ cmd: 'reset' });
    } else {
      this._r = this._w = this._n = 0;
      this._primed = false;
    }
  }

  // push hands a Float32Array of PCM to the active backend.
  push(pcm) {
    if (!this._playing) return;
    if (this.node) {
      this.node.port.postMessage({ pcm }, [pcm.buffer]);
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

  setVolume(v) {
    this._volume = v;
    if (this.gain) this.gain.gain.value = v;
  }

  stop() {
    this._playing = false;
    this.reset();
  }
}
