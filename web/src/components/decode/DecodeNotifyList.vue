<script setup>
defineProps({
  items: { type: Array, default: () => [] },
  itemKey: { type: String, required: true },
  selectedId: { type: [String, Number], default: null },
  statusText: { type: String, default: '' },
  connected: { type: Boolean, default: false },
  emptyText: { type: String, default: '等待信号…' },
  isDim: { type: Function, default: () => false },
});

defineEmits(['select']);
</script>

<template>
  <div class="notify-stack">
    <div class="notify-status" :class="{ ok: connected }">
      {{ statusText }}
    </div>
    <div class="notify-list">
      <div
        v-for="item in items"
        :key="String(item[itemKey])"
        class="notify-item"
        :class="{ active: String(selectedId) === String(item[itemKey]), dim: isDim(item) }"
        @click="$emit('select', item)"
      >
        <slot name="item" :item="item" />
      </div>
      <div v-if="!items.length" class="notify-item notify-item--hint">
        {{ emptyText }}
      </div>
    </div>
  </div>
</template>
