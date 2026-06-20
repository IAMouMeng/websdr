<script setup>
import { computed } from 'vue';
import { useSdrApp } from '../composables/useSdrApp.js';
import { MODES, SAMPLE_RATES, fmtSampleRate, BW_MIN, BW_MAX } from '@/utils/constants.js';

const {
  state,
  mode,
  cwPitch,
  applyMode,
  applyBandwidth,
  onSampleRateChange,
  onCwPitchInput,
} = useSdrApp();

const showCwPitch = computed(() => mode.value === 'cw');

function onModeChange() {
  applyMode();
}

function onFilterBWChange(e) {
  applyBandwidth(parseFloat(e.target.value));
}

function onBwMinus() {
  applyBandwidth(state.filterBW - 100);
}

function onBwPlus() {
  applyBandwidth(state.filterBW + 100);
}
</script>

<template>
  <div class="section">
    <div class="section-title">解调</div>
    <div class="field">
      <label>协议</label>
      <div class="radio-group grid-2">
        <label v-for="m in MODES" :key="m.value">
          <input v-model="mode" type="radio" name="mode" :value="m.value" @change="onModeChange">
          {{ m.label }}
        </label>
      </div>
    </div>
    <div class="field">
      <label>采样率</label>
      <select v-model.number="state.sampleRate" @change="onSampleRateChange">
        <option v-for="sr in SAMPLE_RATES" :key="sr" :value="sr">
          {{ fmtSampleRate(sr) }}
        </option>
      </select>
    </div>
    <div class="field">
      <label>滤波带宽 (Hz)</label>
      <div class="num-input">
        <button title="减小" @click="onBwMinus">−</button>
        <input
          type="number"
          :min="BW_MIN"
          :max="BW_MAX"
          step="100"
          :value="state.filterBW"
          @change="onFilterBWChange"
        >
        <button title="增大" @click="onBwPlus">+</button>
      </div>
    </div>
    <div v-show="showCwPitch" class="field">
      <div class="field-row">
        <label>CW 拍频</label>
        <span class="val">{{ cwPitch }} Hz</span>
      </div>
      <input v-model.number="cwPitch" type="range" min="400" max="1200" step="10" @input="onCwPitchInput">
    </div>
  </div>
</template>
