<script setup>
import { useSdrApp } from '../composables/useSdrApp.js';

const {
  gain,
  agc,
  nr,
  nrLevel,
  volume,
  onGainInput,
  onAgcChange,
  onNrChange,
  onNrLevelInput,
  onVolumeInput,
} = useSdrApp();
</script>

<template>
  <div class="section">
    <div class="section-title">射频</div>
    <div class="field">
      <div class="field-row">
        <label>增益</label>
        <span class="val">{{ gain }} dB</span>
      </div>
      <input v-model.number="gain" type="range" min="0" max="50" @input="onGainInput">
    </div>
    <div class="field">
      <label class="check-row">
        <input v-model="agc" type="checkbox" @change="onAgcChange">
        自动增益 (AGC)
      </label>
    </div>
    <div class="field">
      <label class="check-row">
        <input v-model="nr" type="checkbox" @change="onNrChange">
        降噪 (NR)
      </label>
    </div>
    <div v-show="nr" class="field">
      <div class="field-row">
        <label>降噪强度</label>
        <span class="val">{{ nrLevel }}</span>
      </div>
      <input v-model.number="nrLevel" type="range" min="0" max="100" step="5" @input="onNrLevelInput">
    </div>
    <div class="field">
      <div class="field-row">
        <label>音量</label>
        <span class="val">{{ volume }}</span>
      </div>
      <input v-model.number="volume" type="range" min="0" max="100" @input="onVolumeInput">
    </div>
  </div>
</template>
