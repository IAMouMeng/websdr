// AudioWorklet PCM player — runs on the audio render thread so waterfall/canvas
// work on the main thread cannot stall playback.

class PCMPlayer extends AudioWorkletProcessor {
  constructor() {
    super();
    this.cap = Math.floor(sampleRate * 4);
    this.buf = new Float32Array(this.cap);
    this.r = 0;
    this.w = 0;
    this.n = 0;
    this.prebufferNormal = Math.floor(sampleRate * 0.08);
    this.prebufferFast = Math.floor(sampleRate * 0.03);
    this.prebuffer = this.prebufferNormal;
    this.primed = false;
    this.underruns = 0;

    this.port.onmessage = (e) => {
      const d = e.data;
      if (d.cmd === 'reset') {
        this.r = this.w = this.n = 0;
        this.primed = false;
        this.underruns = 0;
        this.prebuffer = d.fast ? this.prebufferFast : this.prebufferNormal;
        return;
      }
      const pcm = d.pcm;
      if (!pcm) return;
      const cap = this.cap;
      for (let i = 0; i < pcm.length; i++) {
        if (this.n >= cap) {
          this.r = (this.r + 1) % cap;
          this.n--;
        }
        this.buf[this.w] = pcm[i];
        this.w = (this.w + 1) % cap;
        this.n++;
      }
    };
  }

  process(_inputs, outputs) {
    const out = outputs[0][0];
    if (!out) return true;

    if (!this.primed) {
      if (this.n >= this.prebuffer) this.primed = true;
      else { out.fill(0); return true; }
    }

    const cap = this.cap;
    let gap = false;
    for (let i = 0; i < out.length; i++) {
      if (this.n > 0) {
        out[i] = this.buf[this.r];
        this.r = (this.r + 1) % cap;
        this.n--;
      } else {
        out[i] = 0;
        gap = true;
      }
    }
    if (gap) this.underruns++;
    return true;
  }
}

registerProcessor('pcm-player', PCMPlayer);
