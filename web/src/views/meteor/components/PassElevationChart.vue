<script setup>
import { ref, watch, onMounted, onUnmounted } from 'vue';

const props = defineProps({
  curve: { type: Array, default: () => [] },
  active: { type: Boolean, default: false },
});

const canvasRef = ref(null);

function draw() {
  const canvas = canvasRef.value;
  if (!canvas) return;
  const dpr = window.devicePixelRatio || 1;
  const w = canvas.clientWidth;
  const h = canvas.clientHeight;
  if (w < 10) return;
  canvas.width = w * dpr;
  canvas.height = h * dpr;
  const ctx = canvas.getContext('2d');
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);

  const pad = { l: 36, r: 8, t: 8, b: 22 };
  const pw = w - pad.l - pad.r;
  const ph = h - pad.t - pad.b;

  ctx.fillStyle = '#060606';
  ctx.fillRect(0, 0, w, h);

  ctx.strokeStyle = '#1a1a1a';
  ctx.lineWidth = 1;
  for (let el = 0; el <= 90; el += 30) {
    const y = pad.t + ph - (el / 90) * ph;
    ctx.beginPath();
    ctx.moveTo(pad.l, y);
    ctx.lineTo(pad.l + pw, y);
    ctx.stroke();
    ctx.fillStyle = '#444';
    ctx.font = '9px system-ui';
    ctx.textAlign = 'right';
    ctx.fillText(`${el}°`, pad.l - 4, y + 3);
  }

  const curve = props.curve;
  if (!curve.length) {
    ctx.fillStyle = '#444';
    ctx.textAlign = 'center';
    ctx.fillText(props.active ? '当前可见' : '选择过境查看仰角曲线', w / 2, h / 2);
    return;
  }

  const maxMin = curve[curve.length - 1]?.minFromStart || 1;
  ctx.strokeStyle = props.active ? '#6c9' : '#69a';
  ctx.lineWidth = 1.5;
  ctx.beginPath();
  curve.forEach((p, i) => {
    const x = pad.l + (p.minFromStart / maxMin) * pw;
    const y = pad.t + ph - (Math.max(0, p.el) / 90) * ph;
    if (i === 0) ctx.moveTo(x, y);
    else ctx.lineTo(x, y);
  });
  ctx.stroke();

  ctx.fillStyle = '#555';
  ctx.textAlign = 'center';
  ctx.fillText('分钟', pad.l + pw / 2, h - 4);
}

let ro;
onMounted(() => {
  draw();
  ro = new ResizeObserver(draw);
  if (canvasRef.value) ro.observe(canvasRef.value);
});
onUnmounted(() => ro?.disconnect());
watch(() => [props.curve, props.active], draw, { deep: true });
</script>

<template>
  <div class="elev-chart">
    <div class="elev-title">过境仰角</div>
    <canvas ref="canvasRef" class="elev-canvas" />
  </div>
</template>

<style scoped>
.elev-chart {
  flex-shrink: 0;
  height: 120px;
  border: 1px solid #1a1a1a;
  border-radius: 6px;
  background: #080808;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.elev-title {
  padding: 6px 10px;
  font-size: 11px;
  color: #666;
  border-bottom: 1px solid #141414;
}

.elev-canvas {
  flex: 1;
  width: 100%;
  display: block;
}
</style>
