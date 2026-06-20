<script setup>
import { ref, onMounted, onUnmounted } from 'vue';

const clockUtc = ref('--:--:--');
const clockBj = ref('--:--:--');

const pad2 = (n) => String(n).padStart(2, '0');
const fmtClock = (d) => `${pad2(d.getUTCHours())}:${pad2(d.getUTCMinutes())}:${pad2(d.getUTCSeconds())}`;

let timer;
function tickClock() {
  const now = new Date();
  clockUtc.value = fmtClock(now);
  clockBj.value = fmtClock(new Date(now.getTime() + 8 * 3600 * 1000));
}

onMounted(() => {
  tickClock();
  timer = setInterval(tickClock, 1000);
});

onUnmounted(() => clearInterval(timer));
</script>

<template>
  <div class="clocks">
    <div>UTC <span class="mono">{{ clockUtc }}</span></div>
    <div>北京 <span class="mono">{{ clockBj }}</span></div>
  </div>
</template>
