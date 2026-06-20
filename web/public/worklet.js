// AudioWorklet PCM player. Runs on the audio render thread so heavy canvas
// drawing on the main thread can never starve playback. Holds a ring buffer
// with a prebuffer guard; on underflow it re-primes instead of stuttering.

class PCMPlayer extends AudioWorkletProcessor {
  constructor() {
    super();
    this.cap = Math.floor(sampleRate * 3);
    this.buf = new Float32Array(this.cap);
    this.r = 0;
    this.w = 0;
    this.n = 0;
    this.prebuffer = Math.floor(sampleRate * 0.12); // ~120 ms
    this.primed = false;
    this.announced = false;

    this.port.onmessage = (e) => {
      const d = e.data;
      if (d.cmd === 'reset') {
        this.r = this.w = this.n = 0;
        this.primed = false;
        return;
      }
      const pcm = d.pcm;
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
    if (!this.announced) {
      this.announced = true;
      this.port.postMessage({ type: 'alive' }); // confirms the worklet runs
    }
    const out = outputs[0][0];
    if (!out) return true;

    if (!this.primed) {
      if (this.n >= this.prebuffer) this.primed = true;
      else {
        out.fill(0);
        return true;
      }
    }

    const cap = this.cap;
    for (let i = 0; i < out.length; i++) {
      if (this.n > 0) {
        out[i] = this.buf[this.r];
        this.r = (this.r + 1) % cap;
        this.n--;
      } else {
        out[i] = 0;
        this.primed = false; // rebuild the prebuffer before resuming
      }
    }
    return true;
  }
}

registerProcessor('pcm-player', PCMPlayer);
