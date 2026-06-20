// Simple rolling audio waveform for monitoring (no playback).

export function createWaveform(canvas) {
  const ctx = canvas.getContext('2d');
  const cap = 4096;
  const buf = new Float32Array(cap);
  let write = 0;

  function push(pcm) {
    for (let i = 0; i < pcm.length; i++) {
      buf[write % cap] = pcm[i];
      write++;
    }
  }

  function resize() {
    const dpr = window.devicePixelRatio || 1;
    const box = canvas.parentElement.getBoundingClientRect();
    canvas.width = Math.max(1, Math.floor(box.width * dpr));
    canvas.height = Math.max(1, Math.floor(box.height * dpr));
  }

  function render() {
    const w = canvas.width;
    const h = canvas.height;
    const mid = h * 0.5;
    ctx.fillStyle = '#000';
    ctx.fillRect(0, 0, w, h);
    if (write === 0) {
      requestAnimationFrame(render);
      return;
    }
    const n = Math.min(cap, write);
    ctx.strokeStyle = '#3a6a3a';
    ctx.lineWidth = 1;
    ctx.beginPath();
    for (let i = 0; i < n; i++) {
      const v = buf[(write - n + i) % cap];
      const x = (i / (n - 1 || 1)) * w;
      const y = mid - v * mid * 0.9;
      if (i === 0) ctx.moveTo(x, y);
      else ctx.lineTo(x, y);
    }
    ctx.stroke();
    requestAnimationFrame(render);
  }

  resize();
  requestAnimationFrame(render);

  return { push, resize };
}
