// Audio playback pipeline.
//
// Browsers require AudioContext.resume() inside the user-gesture call stack.
// unlockFromGesture() must run synchronously on click/touch/keydown — before
// any await — and must wire up a ScriptProcessor immediately so playback works
// on every hostname (localhost, LAN IP, HTTPS reverse proxy).

export class AudioPlayer {
  constructor(rate = 48000) {
    this.rate = rate;
    this.ctx = null;
    this.gain = null;
    this.analyser = null;
    this._levelBuf = null;

    this.node = null;
    this.sp = null;
    this.mode = null;

    this._ring = null;
    this._cap = 0;
    this._r = 0;
    this._w = 0;
    this._n = 0;
    this._primed = false;
    this._prebuffer = 0;

    this._upgradePromise = null;

    this._playing = false;
    this._volume = 0.8;
    this.onError = null;
  }

  get playing() {
    return this._playing;
  }

  // unlockFromGesture creates/resumes AudioContext and attaches ScriptProcessor
  // synchronously inside a click/touch/keydown handler.
  unlockFromGesture() {
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

    if (this.ctx.state === 'suspended') {
      void this.ctx.resume();
    }

    // Safari / strict autoplay: play a silent buffer in the gesture stack.
    try {
      const buf = this.ctx.createBuffer(1, 1, this.ctx.sampleRate);
      const ping = this.ctx.createBufferSource();
      ping.buffer = buf;
      ping.connect(this.ctx.destination);
      ping.start(0);
      ping.stop(this.ctx.currentTime + 0.001);
    } catch { /* optional */ }

    if (!this.sp && !this.node) {
      this._attachScriptProcessor(this.ctx);
      this.mode = 'script';
    }
  }

  async start() {
    try {
      this.unlockFromGesture();

      if (this.ctx.state === 'suspended') {
        await this.ctx.resume();
      }

      // Upgrade to AudioWorklet in the background (secure contexts only).
      // ScriptProcessor already plays; worklet reduces main-thread jitter.
      if (window.isSecureContext && this.ctx.audioWorklet && !this.node) {
        void this._upgradeToWorklet();
      }

      this.reset();
      this._playing = true;
    } catch (err) {
      console.error('audio start failed', err);
      this._playing = false;
      this.onError?.(String(err?.message ?? err));
      throw err;
    }
  }

  async _upgradeToWorklet() {
    if (this._upgradePromise) return this._upgradePromise;
    this._upgradePromise = this._doUpgradeToWorklet();
    try {
      await this._upgradePromise;
    } catch (err) {
      this._upgradePromise = null;
      throw err;
    }
  }

  async _doUpgradeToWorklet() {
    const ctx = this.ctx;
    if (!ctx?.audioWorklet || this.node) return;

    await ctx.audioWorklet.addModule(new URL('/worklet.js', location.origin).href);
    const node = new AudioWorkletNode(ctx, 'pcm-player', {
      numberOfInputs: 0,
      numberOfOutputs: 1,
      outputChannelCount: [1],
    });
    node.connect(this.gain);

    const sp = this.sp;
    if (sp) {
      try { sp.disconnect(); } catch { /* ignore */ }
      sp.onaudioprocess = null;
      this.sp = null;
    }

    this.node = node;
    this.mode = 'worklet';
  }

  _attachScriptProcessor(ctx) {
    if (this.sp) {
      try { this.sp.disconnect(); } catch { /* ignore */ }
      this.sp.onaudioprocess = null;
    }
    this._buildScriptProcessor(ctx).connect(this.gain);
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

  push(pcm) {
    if (!this._playing) return;
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

  setVolume(v) {
    this._volume = v;
    if (this.gain) this.gain.gain.value = v;
  }

  stop() {
    this._playing = false;
    this.reset();
  }
}
