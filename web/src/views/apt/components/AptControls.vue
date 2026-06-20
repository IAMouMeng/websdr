<script setup>
import { computed } from 'vue';
import {
  APT_FILTER_MIN,
  APT_FILTER_MAX,
} from '../composables/useAPTListen.js';

const props = defineProps({
  gain: { type: Number, required: true },
  agc: { type: Boolean, required: true },
  filterBW: { type: Number, required: true },
});

const emit = defineEmits(['update:gain', 'update:agc', 'update:filterBW']);

const filterRange = { min: APT_FILTER_MIN, max: APT_FILTER_MAX, step: 1000 };
const filterKHz = computed(() => (props.filterBW / 1000).toFixed(0));
</script>

<template>
  <div class="apt-rf">
    <label class="apt-rf-item">
      <span>增益</span>
      <input type="range" min="0" max="50" :value="gain" @input="emit('update:gain', Number($event.target.value))">
      <span class="apt-rf-v">{{ gain }} dB</span>
    </label>
    <label class="apt-rf-item apt-rf-check">
      <input type="checkbox" :checked="agc" @change="emit('update:agc', $event.target.checked)">
      <span>AGC</span>
    </label>
    <label class="apt-rf-item">
      <span>带宽</span>
      <input
        type="range"
        :min="filterRange.min"
        :max="filterRange.max"
        :step="filterRange.step"
        :value="filterBW"
        @input="emit('update:filterBW', Number($event.target.value))"
      >
      <span class="apt-rf-v">{{ filterKHz }} kHz</span>
    </label>
  </div>
</template>

<style scoped>
.apt-rf {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 4px 8px;
  border-bottom: 1px solid #141414;
  background: #050505;
  overflow-x: auto;
}

.apt-rf-item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: #666;
  white-space: nowrap;
  flex-shrink: 0;
}

.apt-rf-item input[type='range'] {
  width: 72px;
  accent-color: #6a9;
}

.apt-rf-v {
  font-family: "SF Mono", "Menlo", monospace;
  font-size: 10px;
  color: #777;
  min-width: 44px;
}

.apt-rf-check {
  gap: 4px;
  cursor: pointer;
}

.apt-rf-check input {
  accent-color: #6a9;
}
</style>
