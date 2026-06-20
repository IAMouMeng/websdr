<script setup>
import { ref, onMounted, onUnmounted } from 'vue';
import { useSdrApp } from '../composables/useSdrApp.js';
import { SPEC_MAX_RANGE, SPEC_MIN_RANGE, ZOOM_RANGE, snapVal, fmtZoom } from '@/utils/constants.js';
import SqSlider from './SqSlider.vue';

const {
  state,
  initDisplay,
  onCanvasDown,
  onCanvasMove,
  onCanvasHover,
  onCanvasWheel,
  setSpecMax,
  setSpecMin,
  setZoom,
} = useSdrApp();

const waterfallRef = ref(null);
const spectrumRef = ref(null);
let cleanupDisplay = null;

onMounted(() => {
  cleanupDisplay = initDisplay(waterfallRef.value, spectrumRef.value);
});

onUnmounted(() => {
  cleanupDisplay?.();
});

function onDown(e, canvas) {
  onCanvasDown(e.clientX, canvas);
}

function onTouchStart(e, canvas) {
  e.preventDefault();
  onDown(e, canvas);
}

function onTouchMove(e) {
  if (state.dragging) {
    e.preventDefault();
    onCanvasMove(e.touches[0].clientX);
  }
}

function onWheel(e) {
  onCanvasWheel(e);
}

function onSpecMax(v) { setSpecMax(snapVal(v, SPEC_MAX_RANGE)); }
function onSpecMin(v) { setSpecMin(snapVal(v, SPEC_MIN_RANGE)); }
function onZoom(v) { setZoom(snapVal(v, ZOOM_RANGE)); }
</script>

<template>
  <div class="display-body">
    <div class="displays">
      <div class="display-spectrum">
        <canvas
          ref="spectrumRef"
          class="spectrum-canvas"
          @mousedown="(e) => onDown(e, e.currentTarget)"
          @touchstart="(e) => onTouchStart(e, e.currentTarget)"
          @mousemove="(e) => onCanvasHover(e.clientX, e.currentTarget)"
          @touchmove="onTouchMove"
          @wheel="onWheel"
        />
      </div>
      <div class="display-waterfall">
        <canvas
          ref="waterfallRef"
          class="waterfall-canvas"
          @mousedown="(e) => onDown(e, e.currentTarget)"
          @touchstart="(e) => onTouchStart(e, e.currentTarget)"
          @mousemove="(e) => onCanvasHover(e.clientX, e.currentTarget)"
          @touchmove="onTouchMove"
          @wheel="onWheel"
        />
      </div>
    </div>
    <div class="right-panel">
      <SqSlider
        label="Max"
        :model-value="state.specMax"
        :range="SPEC_MAX_RANGE"
        @update:model-value="onSpecMax"
      />
      <SqSlider
        label="Min"
        :model-value="state.specMin"
        :range="SPEC_MIN_RANGE"
        @update:model-value="onSpecMin"
      />
      <SqSlider
        label="Zoom"
        :model-value="state.zoom"
        :range="ZOOM_RANGE"
        :formatter="fmtZoom"
        @update:model-value="onZoom"
      />
    </div>
  </div>
</template>
