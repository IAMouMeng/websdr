<script setup>
import { computed } from 'vue';
import { RouterLink } from 'vue-router';
import { signalPlayRoute } from '../utils/signalPlay.js';

const props = defineProps({
  item: { type: Object, required: true },
});

const to = computed(() => signalPlayRoute(props.item));
</script>

<template>
  <RouterLink
    v-if="to"
    :to="to"
    class="play-link"
    title="打开收听/解码"
    @click.stop
  >
    <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
      <path
        fill="currentColor"
        d="M8 5.14v13.72a1 1 0 0 0 1.5.86l11.04-6.86a1 1 0 0 0 0-1.72L9.5 4.28A1 1 0 0 0 8 5.14z"
      />
    </svg>
  </RouterLink>
</template>

<style scoped>
.play-link {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  color: #666;
  text-decoration: none;
  border-radius: 4px;
  transition: color 0.12s;
}

.play-link:hover {
  color: #9cf;
}
</style>
