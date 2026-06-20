// WebRTC audio fallback — browser plays RTP via <audio>, no Web Audio API.

function waitIceGathering(pc) {
  if (pc.iceGatheringState === 'complete') return Promise.resolve();
  return new Promise((resolve) => {
    const done = () => {
      if (pc.iceGatheringState === 'complete') {
        pc.removeEventListener('icegatheringstatechange', done);
        resolve();
      }
    };
    pc.addEventListener('icegatheringstatechange', done);
    setTimeout(resolve, 3000);
  });
}

export class WebRtcAudio {
  constructor() {
    this.pc = null;
    this.el = null;
    this._active = false;
  }

  get active() {
    return this._active;
  }

  async start() {
    await this.stop();

    const pc = new RTCPeerConnection({
      iceServers: [{ urls: 'stun:stun.l.google.com:19302' }],
    });
    const el = document.createElement('audio');
    el.autoplay = true;
    el.playsInline = true;
    el.style.display = 'none';
    document.body.appendChild(el);

    pc.ontrack = (ev) => {
      const stream = ev.streams[0] ?? new MediaStream([ev.track]);
      el.srcObject = stream;
      void el.play().catch(() => {});
    };

    pc.addTransceiver('audio', { direction: 'recvonly' });

    const offer = await pc.createOffer();
    await pc.setLocalDescription(offer);
    await waitIceGathering(pc);

    const resp = await fetch('/api/webrtc/offer', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ sdp: pc.localDescription.sdp }),
    });
    if (!resp.ok) {
      throw new Error(`WebRTC offer failed: ${resp.status}`);
    }
    const { sdp } = await resp.json();
    await pc.setRemoteDescription({ type: 'answer', sdp });

    this.pc = pc;
    this.el = el;
    this._active = true;
  }

  async stop() {
    this._active = false;
    if (this.pc) {
      try { this.pc.close(); } catch { /* ignore */ }
      this.pc = null;
    }
    if (this.el) {
      this.el.srcObject = null;
      this.el.remove();
      this.el = null;
    }
  }

  setVolume(v) {
    if (this.el) this.el.volume = Math.max(0, Math.min(1, v));
  }
}
