<script setup>
defineProps({
  utc: { type: String, default: '—' },
  beijing: { type: String, default: '—' },
  passHighlight: { type: String, default: '' },
  passDetail: { type: String, default: '' },
  freq: { type: String, default: '—' },
  linkLabel: { type: String, default: '下行' },
});
</script>

<template>
  <header class="sat-head">
    <h2>卫星解码</h2>
    <div class="clocks">
      <span class="clock"><em>UTC</em> {{ utc }}</span>
      <span class="clock"><em>北京</em> {{ beijing }}</span>
    </div>
    <div class="head-right">
      <span v-if="passHighlight" class="pass-hl">{{ passHighlight }}</span>
      <span v-if="passDetail" class="pass-detail">{{ passDetail }}</span>
      <span class="head-freq">
        <span class="link-tag">{{ linkLabel }}</span>
        {{ freq }}
      </span>
    </div>
  </header>
</template>

<style scoped>
.sat-head {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 14px;
  flex-wrap: nowrap;
  padding: 8px 14px;
  border-bottom: 1px solid #141414;
  background: #060606;
  overflow: hidden;
  min-height: 40px;
}

.sat-head h2 {
  flex-shrink: 0;
  font-size: 15px;
  font-weight: 600;
  color: #eee;
  margin: 0;
}

.clocks {
  flex-shrink: 0;
  display: flex;
  gap: 12px;
}

.clock {
  font-family: "SF Mono", Menlo, monospace;
  font-size: 12px;
  color: #aaa;
  white-space: nowrap;
}

.clock em {
  font-style: normal;
  color: #555;
  margin-right: 6px;
  font-size: 10px;
}

.head-right {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 14px;
  flex-shrink: 0;
  white-space: nowrap;
  font-size: 12px;
}

.pass-hl {
  color: #e6c84a;
  font-weight: 600;
  font-family: "SF Mono", Menlo, monospace;
}

.pass-detail {
  color: #9ab;
  font-family: "SF Mono", Menlo, monospace;
}

.head-freq {
  color: #9cf;
  font-family: "SF Mono", Menlo, monospace;
  font-size: 12px;
}

.link-tag {
  display: inline-block;
  margin-right: 6px;
  padding: 1px 5px;
  border-radius: 3px;
  background: #1a2a1a;
  color: #9c9;
  font-size: 10px;
}
</style>
