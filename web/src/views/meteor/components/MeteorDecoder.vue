<script setup>
import MeteorScatter from './MeteorScatter.vue';

defineProps({
  connected: { type: Boolean, default: false },
  meteor: { type: Object, required: true },
  mainImage: { type: String, default: '' },
  selectedChannel: { type: Number, default: 0 },
});

defineEmits(['select-channel']);
</script>

<template>
  <footer class="decoder-grid">
    <div class="cell scatter-cell">
      <div class="cell-label">星座图</div>
      <MeteorScatter
        mini
        class="decoder-scatter"
        :points="meteor.constellation"
        :locked="meteor.locked"
      />
    </div>

    <div class="cell status-cell">
      <div class="cell-label">解码状态</div>
      <div class="status-lines">
        <div class="status-row">
          <span class="k">载波</span>
          <span class="v" :class="{ on: meteor.locked }">{{ meteor.locked ? '锁定' : '搜锁' }}</span>
        </div>
        <div class="status-row">
          <span class="k">帧同步</span>
          <span class="v" :class="{ on: meteor.synced }">{{ meteor.synced ? '已同步' : '—' }}</span>
        </div>
        <div v-if="meteor.metric" class="status-row">
          <span class="k">符号率</span>
          <span class="v dim">{{ meteor.metric }}</span>
        </div>
        <div v-if="meteor.lines" class="status-row">
          <span class="k">行数</span>
          <span class="v dim">{{ meteor.lines }}</span>
        </div>
      </div>
    </div>

    <div class="cell channels-cell">
      <div class="cell-label">通道</div>
      <div class="ch-grid">
        <button
          v-for="(ch, i) in meteor.channels"
          :key="ch.id"
          type="button"
          class="ch-tile"
          :class="{ on: selectedChannel === i, live: ch.active }"
          :title="`${ch.name} · ${ch.band}`"
          @click="$emit('select-channel', i)"
        >
          <img v-show="ch.image" :src="ch.image" alt="" class="ch-thumb">
          <span v-show="!ch.image" class="ch-placeholder">{{ ch.name.slice(0, 2) }}</span>
          <span class="ch-name">{{ ch.name }}</span>
          <span v-if="ch.lines" class="ch-lines">{{ ch.lines }}</span>
        </button>
      </div>
    </div>

    <div class="cell preview-cell">
      <div class="cell-label">合成预览</div>
      <div class="preview-wrap">
        <img v-show="mainImage" :src="mainImage || undefined" alt="">
        <div v-show="!mainImage" class="preview-hint">
          {{ !connected ? '连接中…' : meteor.locked ? '累积行…' : '等待信号' }}
        </div>
      </div>
    </div>
  </footer>
</template>

<style scoped>
.decoder-grid {
  flex-shrink: 0;
  display: grid;
  grid-template-columns: 1fr 1fr;
  grid-template-rows: 100px 100px;
  gap: 4px;
  padding: 6px;
  border-top: 1px solid #141414;
  background: #080808;
}

.cell {
  position: relative;
  border: 1px solid #1a1a1a;
  border-radius: 4px;
  background: #0a0a0a;
  min-width: 0;
  overflow: hidden;
}

.cell-label {
  position: absolute;
  left: 6px;
  top: 4px;
  font-size: 9px;
  font-weight: 600;
  color: #444;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  z-index: 1;
  pointer-events: none;
}

.scatter-cell {
  display: flex;
  align-items: center;
  justify-content: center;
  padding-top: 14px;
}

.decoder-scatter {
  width: 88px;
  height: 72px;
}

.status-cell {
  padding: 18px 10px 8px;
}

.status-lines {
  display: flex;
  flex-direction: column;
  gap: 4px;
  height: 100%;
  justify-content: center;
}

.status-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
  font-size: 11px;
}

.status-row .k { color: #555; }
.status-row .v { color: #666; font-family: "SF Mono", Menlo, monospace; }
.status-row .v.on { color: #8c8; }
.status-row .v.dim { color: #777; font-size: 10px; }

.channels-cell {
  padding: 16px 6px 4px;
}

.ch-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 3px;
  height: 100%;
}

.ch-tile {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 1px;
  padding: 2px;
  border: 1px solid #141414;
  border-radius: 3px;
  background: #080808;
  color: #666;
  font-size: 9px;
  cursor: pointer;
  min-height: 0;
  overflow: hidden;
}

.ch-tile.on {
  border-color: #3a6a8a;
  background: #0a1018;
  color: #9cf;
}

.ch-tile.live {
  border-color: #2a4a2a;
}

.ch-thumb {
  width: 100%;
  height: 28px;
  object-fit: cover;
  object-position: top;
  image-rendering: pixelated;
  border-radius: 2px;
}

.ch-placeholder {
  width: 100%;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #111;
  color: #444;
  border-radius: 2px;
  font-size: 10px;
}

.ch-name {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 100%;
}

.ch-lines {
  font-size: 8px;
  color: #5a8;
  font-family: "SF Mono", Menlo, monospace;
}

.preview-cell {
  padding: 16px 6px 4px;
  display: flex;
  flex-direction: column;
}

.preview-wrap {
  flex: 1;
  min-height: 0;
  position: relative;
  border-radius: 3px;
  overflow: hidden;
  background: #000;
}

.preview-wrap img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: top;
  image-rendering: pixelated;
}

.preview-hint {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 10px;
  color: #444;
  background: #000;
}
</style>
