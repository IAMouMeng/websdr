<script setup>
defineProps({
  sampleRate: { type: Number, required: true },
  channels: { type: Number, required: true },
  state: { type: String, default: 'idle' },
  elapsedStr: { type: String, default: '00:00' },
  sizeStr: { type: String, default: '0 B' },
  canDownload: { type: Boolean, default: false },
  rateOptions: { type: Array, default: () => [] },
  autoPass: { type: Boolean, default: false },
  preRollSec: { type: Number, default: 5 },
  postRollSec: { type: Number, default: 5 },
  autoPassStatus: { type: String, default: '' },
  isGeo: { type: Boolean, default: false },
});

defineEmits([
  'update:sampleRate',
  'update:channels',
  'update:autoPass',
  'update:preRollSec',
  'update:postRollSec',
  'start',
  'pause',
  'resume',
  'stop',
  'download',
]);
</script>

<template>
  <aside class="record-panel">
    <div class="record-title">Record</div>

    <label class="rec-row">
      <span>采样率</span>
      <select
        :value="sampleRate"
        :disabled="state !== 'idle'"
        @change="$emit('update:sampleRate', Number($event.target.value))"
      >
        <option v-for="o in rateOptions" :key="o.value" :value="o.value">{{ o.label }}</option>
      </select>
    </label>

    <label class="rec-row">
      <span>通道</span>
      <select
        :value="channels"
        :disabled="state !== 'idle'"
        @change="$emit('update:channels', Number($event.target.value))"
      >
        <option :value="1">单通道 (I)</option>
        <option :value="2">双通道 (IQ)</option>
      </select>
    </label>

    <div class="rec-auto">
      <label class="rec-check" :class="{ disabled: isGeo }">
        <input
          type="checkbox"
          :checked="autoPass"
          :disabled="isGeo"
          @change="$emit('update:autoPass', $event.target.checked)"
        >
        <span>过境自动录制</span>
      </label>
      <p v-if="isGeo" class="rec-hint">GEO 卫星无过境窗口</p>
      <template v-else-if="autoPass">
        <div class="rec-buffer">
          <label class="buf-row">
            <span>提前</span>
            <input
              type="number"
              min="0"
              max="120"
              step="1"
              :value="preRollSec"
              :disabled="state !== 'idle'"
              @input="$emit('update:preRollSec', Number($event.target.value))"
            >
            <span>秒</span>
          </label>
          <label class="buf-row">
            <span>延后</span>
            <input
              type="number"
              min="0"
              max="120"
              step="1"
              :value="postRollSec"
              :disabled="state !== 'idle'"
              @input="$emit('update:postRollSec', Number($event.target.value))"
            >
            <span>秒</span>
          </label>
        </div>
        <p v-if="autoPassStatus" class="rec-auto-status">{{ autoPassStatus }}</p>
      </template>
    </div>

    <div class="rec-status">
      <span v-if="state === 'recording'" class="rec-live">● 录制中</span>
      <span v-else-if="state === 'paused'" class="rec-pause">⏸ 已暂停</span>
      <span v-else class="rec-idle">就绪</span>
      <span class="rec-meta">{{ elapsedStr }} · {{ sizeStr }}</span>
    </div>

    <div class="rec-actions">
      <button
        type="button"
        class="rec-btn primary"
        :disabled="state !== 'idle' || autoPass"
        @click="$emit('start')"
      >
        开始
      </button>
      <button
        type="button"
        class="rec-btn"
        :disabled="state !== 'recording'"
        @click="$emit('pause')"
      >
        暂停
      </button>
      <button
        type="button"
        class="rec-btn"
        :disabled="state !== 'paused'"
        @click="$emit('resume')"
      >
        继续
      </button>
      <button
        type="button"
        class="rec-btn warn"
        :disabled="state === 'idle'"
        @click="$emit('stop')"
      >
        结束
      </button>
      <button
        type="button"
        class="rec-btn full"
        :disabled="!canDownload"
        @click="$emit('download')"
      >
        下载 WAV
      </button>
    </div>
  </aside>
</template>

<style scoped>
.record-panel {
  width: 176px;
  flex-shrink: 0;
  align-self: stretch;
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px;
  border: 1px solid #1a1a1a;
  border-radius: 6px;
  background: #0a0a0a;
  min-height: 0;
  overflow-y: auto;
}

.record-title {
  font-size: 11px;
  font-weight: 600;
  color: #666;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.rec-row {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 10px;
  color: #666;
}

.rec-row select {
  background: #111;
  border: 1px solid #222;
  color: #ccc;
  border-radius: 4px;
  padding: 4px 6px;
  font-size: 11px;
}

.rec-row select:disabled {
  opacity: 0.5;
}

.rec-auto {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 6px 0;
  border-top: 1px solid #141414;
  border-bottom: 1px solid #141414;
}

.rec-check {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: #aaa;
  cursor: pointer;
}

.rec-check.disabled {
  opacity: 0.45;
  cursor: default;
}

.rec-hint,
.rec-auto-status {
  font-size: 10px;
  color: #666;
  margin: 0;
  line-height: 1.4;
}

.rec-auto-status {
  color: #9ab;
  font-family: "SF Mono", Menlo, monospace;
}

.rec-buffer {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.buf-row {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 10px;
  color: #666;
}

.buf-row input {
  width: 44px;
  background: #111;
  border: 1px solid #222;
  color: #ccc;
  border-radius: 4px;
  padding: 2px 4px;
  font-size: 11px;
}

.buf-row input:disabled {
  opacity: 0.5;
}

.rec-status {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 4px 0;
  font-size: 11px;
}

.rec-live { color: #c66; }
.rec-pause { color: #ca6; }
.rec-idle { color: #555; }

.rec-meta {
  font-family: "SF Mono", Menlo, monospace;
  font-size: 10px;
  color: #555;
}

.rec-actions {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 6px;
}

.rec-btn {
  padding: 7px 0;
  border: 1px solid #2a2a2a;
  border-radius: 4px;
  background: #111;
  color: #aaa;
  font-size: 11px;
  cursor: pointer;
}

.rec-btn.full {
  grid-column: 1 / -1;
}

.rec-btn:hover:not(:disabled) {
  background: #1a1a1a;
  color: #ddd;
}

.rec-btn:disabled {
  opacity: 0.35;
  cursor: default;
}

.rec-btn.primary:not(:disabled) {
  border-color: #3a5a6a;
  background: #0e1820;
  color: #9cf;
}

.rec-btn.warn:not(:disabled) {
  border-color: #5a3a2a;
  background: #1a100e;
  color: #c96;
}
</style>
