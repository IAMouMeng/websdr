<script setup>
import { computed } from 'vue';
import { useSdrApp } from '../composables/useSdrApp.js';
import { DIAL_DIGITS, clampFreq } from '@/utils/constants.js';
import ClockDisplay from './ClockDisplay.vue';
import VuMeter from './VuMeter.vue';

const { state, setTuneFreq, onRecenter } = useSdrApp();

const dialItems = computed(() => {
  const items = [];
  for (let i = 0; i < DIAL_DIGITS; i++) {
    const exp = DIAL_DIGITS - 1 - i;
    if (i > 0 && exp % 3 === 2) {
      items.push({ type: 'sep' });
    }
    items.push({ type: 'digit', place: 10 ** exp });
  }
  return items;
});

function digitChar(place) {
  return Math.floor(state.tuneFreq / place) % 10;
}

function isLead(place) {
  return place > state.tuneFreq && place > 1;
}

function adjustDigit(place, dir) {
  setTuneFreq(clampFreq(state.tuneFreq + dir * place));
}

function onWheel(e, place) {
  e.preventDefault();
  adjustDigit(place, e.deltaY < 0 ? 1 : -1);
}

function onClick(e, place) {
  const r = e.currentTarget.getBoundingClientRect();
  adjustDigit(place, e.clientY - r.top < r.height / 2 ? 1 : -1);
}
</script>

<template>
  <header class="topbar">
    <div class="topbar-left">
      <div class="freq-dial">
        <template v-for="(item, idx) in dialItems" :key="idx">
          <span v-if="item.type === 'sep'" class="fd-sep">,</span>
          <span
            v-else
            class="fd"
            :class="{ lead: isLead(item.place) }"
            @wheel="onWheel($event, item.place)"
            @click="onClick($event, item.place)"
          >{{ digitChar(item.place) }}</span>
        </template>
        <span class="fd-unit">Hz</span>
      </div>
      <button class="recenter-btn" title="将频谱中心对准当前频率" @click="onRecenter">⌖</button>
    </div>
    <div class="topbar-right">
      <ClockDisplay />
      <div class="vu">
        <VuMeter />
      </div>
    </div>
  </header>
</template>
