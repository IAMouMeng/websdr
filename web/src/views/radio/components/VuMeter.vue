<script setup>
import { ref, onMounted, onUnmounted } from 'vue';
import { useSdrApp } from '../composables/useSdrApp.js';

const VU_CSS_W = 220;
const VU_CSS_H = 38;
const VU_FLOOR = -60;
const VU_CEIL = 0;

const { audio } = useSdrApp();
const canvasRef = ref(null);
let vuPeak = -100;
let rafId = null;

function vuLayout() {
  const dpr = window.devicePixelRatio || 1;
  return {
    dpr,
    axisH: Math.round(20 * dpr),
    font: Math.round(13 * dpr),
    tick: Math.round(6 * dpr),
    pad: Math.round(4 * dpr),
  };
}

function resizeVU() {
  const canvas = canvasRef.value;
  if (!canvas) return;
  const { dpr } = vuLayout();
  canvas.width = Math.round(VU_CSS_W * dpr);
  canvas.height = Math.round(VU_CSS_H * dpr);
}

function drawVU() {
  const canvas = canvasRef.value;
  if (!canvas) {
    rafId = requestAnimationFrame(drawVU);
    return;
  }
  const vuCtx = canvas.getContext('2d');
  const { axisH, font, tick, pad } = vuLayout();
  const db = audio.getLevelDb();
  const lvl = Number.isFinite(db) ? db : VU_FLOOR;
  let v = (lvl - VU_FLOOR) / (VU_CEIL - VU_FLOOR);
  v = v < 0 ? 0 : v > 1 ? 1 : v;

  if (lvl > vuPeak) vuPeak = lvl;
  else vuPeak -= 0.8;
  let pv = (vuPeak - VU_FLOOR) / (VU_CEIL - VU_FLOOR);
  pv = pv < 0 ? 0 : pv > 1 ? 1 : pv;

  const w = canvas.width, h = canvas.height;
  const barY = axisH;
  const barH = h - axisH;
  const xOfDb = (dbTick) => w - ((dbTick - VU_FLOOR) / (VU_CEIL - VU_FLOOR)) * w;

  vuCtx.fillStyle = '#111';
  vuCtx.fillRect(0, 0, w, h);

  vuCtx.font = `${font}px monospace`;
  vuCtx.textBaseline = 'top';
  vuCtx.textAlign = 'center';
  vuCtx.strokeStyle = 'rgba(255,255,255,0.35)';
  vuCtx.fillStyle = '#bbb';
  for (let dbTick = VU_FLOOR; dbTick <= VU_CEIL; dbTick += 20) {
    const x = xOfDb(dbTick);
    vuCtx.beginPath();
    vuCtx.moveTo(x, axisH - tick);
    vuCtx.lineTo(x, axisH);
    vuCtx.stroke();
    if (dbTick === VU_CEIL) vuCtx.textAlign = 'left';
    else if (dbTick === VU_FLOOR) vuCtx.textAlign = 'right';
    else vuCtx.textAlign = 'center';
    vuCtx.fillText(String(dbTick), dbTick === VU_CEIL ? 0 : dbTick === VU_FLOOR ? w : x, pad);
  }
  vuCtx.strokeStyle = 'rgba(255,255,255,0.25)';
  vuCtx.beginPath();
  vuCtx.moveTo(0, axisH);
  vuCtx.lineTo(w, axisH);
  vuCtx.stroke();

  const barW = v * w;
  vuCtx.fillStyle = '#fc0';
  vuCtx.fillRect(0, barY, barW, barH);
  vuCtx.fillStyle = '#ffe066';
  vuCtx.fillRect(Math.min(w - 2, pv * w), barY, 2, barH);

  rafId = requestAnimationFrame(drawVU);
}

onMounted(() => {
  resizeVU();
  window.addEventListener('resize', resizeVU);
  rafId = requestAnimationFrame(drawVU);
});

onUnmounted(() => {
  window.removeEventListener('resize', resizeVU);
  if (rafId) cancelAnimationFrame(rafId);
});
</script>

<template>
  <canvas ref="canvasRef" class="vumeter" width="220" height="38" />
</template>
