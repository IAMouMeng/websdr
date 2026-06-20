<script setup>
import { ref, watch, onMounted, onUnmounted } from 'vue';

const props = defineProps({
  points: { type: Array, default: () => [] },
  locked: { type: Boolean, default: false },
  compact: { type: Boolean, default: false },
  mini: { type: Boolean, default: false },
});

const canvasRef = ref(null);

function draw() {
  const canvas = canvasRef.value;
  if (!canvas) return;
  const dpr = window.devicePixelRatio || 1;
  const w = canvas.clientWidth;
  const h = canvas.clientHeight;
  if (w < 10 || h < 10) return;
  canvas.width = w * dpr;
  canvas.height = h * dpr;
  const ctx = canvas.getContext('2d');
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);

  ctx.fillStyle = '#050505';
  ctx.fillRect(0, 0, w, h);

  const cx = w / 2;
  const cy = h / 2;
  const r = Math.min(w, h) * 0.44;

  if (!props.mini) {
    ctx.strokeStyle = '#1a1a1a';
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(cx - r, cy);
    ctx.lineTo(cx + r, cy);
    ctx.moveTo(cx, cy - r);
    ctx.lineTo(cx, cy + r);
    ctx.stroke();
  }

  const pts = props.points;
  if (!pts || pts.length < 4) {
    ctx.fillStyle = '#444';
    ctx.font = props.mini ? '8px system-ui' : '11px system-ui';
    ctx.textAlign = 'center';
    ctx.fillText(props.mini ? '等待锁定' : '等待 OQPSK 锁定…', cx, cy);
    return;
  }

  const dot = props.mini ? 1.8 : 2.2;
  const color = props.locked ? 'rgba(100,220,150,0.7)' : 'rgba(90,150,220,0.5)';
  ctx.fillStyle = color;
  for (let i = 0; i + 1 < pts.length; i += 2) {
    const x = cx + pts[i] * r;
    const y = cy - pts[i + 1] * r;
    ctx.fillRect(x - dot, y - dot, dot * 2, dot * 2);
  }

  ctx.fillStyle = '#555';
  if (!props.mini) {
    ctx.font = '10px system-ui';
    ctx.textAlign = 'left';
    ctx.fillText('I', cx + r + 6, cy + 4);
    ctx.textAlign = 'center';
    ctx.fillText('Q', cx, cy - r - 6);
  }
}

let ro;
onMounted(() => {
  draw();
  ro = new ResizeObserver(draw);
  if (canvasRef.value) ro.observe(canvasRef.value);
});
onUnmounted(() => ro?.disconnect());
watch(() => [props.points, props.locked], draw, { deep: true });
</script>

<template>
  <div class="scatter-wrap" :class="{ compact, mini }">
    <div v-if="!mini" class="scatter-title">OQPSK 星座</div>
    <canvas ref="canvasRef" class="scatter-canvas" />
  </div>
</template>

<style scoped>
.scatter-wrap {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  border: 1px solid #1a1a1a;
  border-radius: 6px;
  background: #080808;
  overflow: hidden;
}

.scatter-wrap.compact {
  flex: none;
  height: 100px;
}

.scatter-wrap.mini {
  flex: none;
  width: 64px;
  height: 56px;
  border-radius: 4px;
}

.scatter-wrap.mini .scatter-canvas {
  min-height: 0;
}

.scatter-title {
  flex-shrink: 0;
  padding: 4px 8px;
  font-size: 10px;
  color: #555;
  border-bottom: 1px solid #141414;
}

.scatter-canvas {
  flex: 1;
  width: 100%;
  min-height: 160px;
  display: block;
}

.compact .scatter-canvas {
  min-height: 72px;
}
</style>
