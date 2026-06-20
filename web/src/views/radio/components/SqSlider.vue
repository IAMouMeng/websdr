<script setup>
import { ref, computed, watch, onMounted, onUnmounted } from 'vue';

const props = defineProps({
  label: { type: String, required: true },
  modelValue: { type: Number, required: true },
  range: { type: Object, required: true },
  formatter: { type: Function, default: (v) => String(v) },
});

const emit = defineEmits(['update:modelValue']);

const trackRef = ref(null);
const dragging = ref(false);

const frac = computed(() => (props.modelValue - props.range.min) / (props.range.max - props.range.min));
const fillStyle = computed(() => ({ height: `${frac.value * 100}%` }));
const thumbStyle = computed(() => ({ top: `${(1 - frac.value) * 100}%` }));
const displayVal = computed(() => props.formatter(props.modelValue));

function apply(v) {
  emit('update:modelValue', v);
}

function setFromY(clientY) {
  const track = trackRef.value;
  if (!track) return;
  const rect = track.getBoundingClientRect();
  let f = 1 - (clientY - rect.top) / rect.height;
  f = f < 0 ? 0 : f > 1 ? 1 : f;
  apply(props.range.min + f * (props.range.max - props.range.min));
}

function onMouseDown(e) {
  dragging.value = true;
  setFromY(e.clientY);
}

function onTouchStart(e) {
  dragging.value = true;
  setFromY(e.touches[0].clientY);
}

function onGlobalMove(e) {
  if (dragging.value) setFromY(e.clientY);
}

function onGlobalUp() {
  dragging.value = false;
}

function onWheel(e) {
  e.preventDefault();
  apply(props.modelValue + (e.deltaY < 0 ? 1 : -1) * props.range.step);
}

onMounted(() => {
  window.addEventListener('mousemove', onGlobalMove);
  window.addEventListener('mouseup', onGlobalUp);
  window.addEventListener('touchend', onGlobalUp);
});

onUnmounted(() => {
  window.removeEventListener('mousemove', onGlobalMove);
  window.removeEventListener('mouseup', onGlobalUp);
  window.removeEventListener('touchend', onGlobalUp);
});
</script>

<template>
  <div class="sq-ctl">
    <div class="sq-label">{{ label }}</div>
    <div
      ref="trackRef"
      class="sq-track"
      @mousedown="onMouseDown"
      @touchstart.passive="onTouchStart"
      @touchmove.passive="onGlobalMove"
      @wheel.prevent="onWheel"
    >
      <div class="sq-fill" :style="fillStyle" />
      <div class="sq-thumb" :style="thumbStyle" />
    </div>
    <div class="sq-val">{{ displayVal }}</div>
  </div>
</template>
