<script setup>
import { useSdrApp } from '../composables/useSdrApp.js';

const { statusText, statusClass, playDisabled, stopDisabled, onPlay, onStop } = useSdrApp();
</script>

<template>
  <div class="sidebar-head">
    <div class="sidebar-brand">
      <h1>WebSDR</h1>
      <div class="transport-btns">
        <button class="icon-btn primary" title="播放" :disabled="playDisabled" @click="onPlay">▶</button>
        <button class="icon-btn" title="停止" :disabled="stopDisabled" @click="onStop">■</button>
      </div>
    </div>
    <div class="status" :class="statusClass">{{ statusText }}</div>
  </div>
</template>
