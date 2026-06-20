<script setup>
defineProps({
  item: { type: Object, required: true },
});
</script>

<template>
  <div v-if="item.decode" class="decode-panel">
    <div class="decode-head">
      <span class="decode-title">解调数据</span>
      <span class="decode-service">{{ item.decode.service }}</span>
      <span v-if="item.decode.imageLines" class="decode-lines">{{ item.decode.imageLines }} 行</span>
    </div>

    <div v-if="item.image" class="apt-image-wrap">
      <div class="apt-image-label">
        <span>可见光</span>
        <span>红外</span>
      </div>
      <img :src="item.image" class="apt-image" alt="APT 云图">
    </div>

    <div class="decode-grid">
      <div v-if="item.decode.pi && item.decode.pi !== '—'" class="decode-cell">
        <span class="decode-key">PI</span>
        <span class="decode-val">{{ item.decode.pi }}</span>
      </div>
      <div v-if="item.decode.ps && item.decode.ps !== '—'" class="decode-cell decode-cell--highlight">
        <span class="decode-key">台名</span>
        <span class="decode-val">{{ item.decode.ps }}</span>
      </div>
      <div v-if="item.decode.pty && item.decode.pty !== '—'" class="decode-cell">
        <span class="decode-key">PTY</span>
        <span class="decode-val">{{ item.decode.pty }}</span>
      </div>
      <div v-if="item.decode.mod" class="decode-cell">
        <span class="decode-key">调制</span>
        <span class="decode-val">{{ item.decode.mod }}</span>
      </div>
      <div v-if="item.decode.direction" class="decode-cell">
        <span class="decode-key">方向</span>
        <span class="decode-val">{{ item.decode.direction }}</span>
      </div>
      <div v-if="item.decode.subcarrier" class="decode-cell">
        <span class="decode-key">副载波</span>
        <span class="decode-val">{{ item.decode.subcarrier }}</span>
      </div>
      <div v-if="item.decode.metric" class="decode-cell">
        <span class="decode-key">{{ item.decode.metricLabel || '指标' }}</span>
        <span class="decode-val">{{ item.decode.metric }}</span>
      </div>
      <div v-if="item.freq" class="decode-cell">
        <span class="decode-key">频率</span>
        <span class="decode-val">{{ item.freq }}</span>
      </div>
      <div v-if="item.strength != null" class="decode-cell">
        <span class="decode-key">强度</span>
        <span class="decode-val">{{ item.strength }} dBm</span>
      </div>
    </div>
    <p v-if="item.decode.note" class="decode-note">{{ item.decode.note }}</p>
  </div>
</template>

<style scoped>
.decode-panel {
  margin: 0 10px 10px 34px;
  padding: 10px 0 14px;
}

.decode-head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
  flex-wrap: wrap;
}

.decode-title {
  font-size: 10px;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: #6a9f6a;
}

.decode-service {
  font-size: 12px;
  font-weight: 600;
  color: #b8ddb8;
}

.decode-lines {
  font-size: 10px;
  color: #7a9a7a;
  font-family: "SF Mono", "Menlo", monospace;
}

.apt-image-wrap {
  margin-bottom: 10px;
}

.apt-image-label {
  display: flex;
  justify-content: space-around;
  font-size: 10px;
  color: #6a806a;
  margin-bottom: 4px;
  padding: 0 4px;
}

.apt-image {
  display: block;
  width: 100%;
  max-width: 720px;
  height: auto;
  border-radius: 4px;
  background: #0a0a0a;
}

.decode-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
  gap: 8px 16px;
}

.decode-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.decode-cell--highlight .decode-val {
  color: #9ecf9e;
  font-weight: 600;
}

.decode-key {
  font-size: 10px;
  color: #5a705a;
}

.decode-val {
  font-family: "SF Mono", "Menlo", monospace;
  font-size: 11px;
  color: #a8c4a8;
}

.decode-note {
  margin: 8px 0 0;
  font-size: 11px;
  color: #6a806a;
  line-height: 1.45;
}
</style>
