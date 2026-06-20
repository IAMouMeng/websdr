<script setup>
import { computed } from 'vue';
import { useProtocolScan } from '../composables/useProtocolScan.js';
import { TABLE_COLS } from '../constants.js';
import SignalDecodePanel from './SignalDecodePanel.vue';
import SignalPlayLink from './SignalPlayLink.vue';

const {
  listening,
  fullScanning,
  fullScanPhase,
  startedAt,
  scanBand,
  scanProgress,
  filters,
  filteredSignals,
  counts,
  totalCount,
  expanded,
  toggleExpand,
  typeLabels,
  groupOrder,
} = useProtocolScan();

const groups = computed(() =>
  groupOrder
    .filter((t) => filters.value[t])
    .map((type) => ({
      type,
      label: typeLabels[type],
      cols: TABLE_COLS[type],
      items: filteredSignals.value
        .filter((s) => s.type === type)
        .sort((a, b) => (a.freqHz || 0) - (b.freqHz || 0) || String(a.id).localeCompare(String(b.id))),
    }))
    .filter((g) => g.items.length > 0),
);

const startedTimeText = computed(() => {
  if (!startedAt.value) return '';
  return startedAt.value.toLocaleTimeString('zh-CN', { hour12: false });
});

const countSummary = computed(() =>
  groupOrder
    .filter((t) => counts.value[t] > 0)
    .map((t) => `${typeLabels[t].split(' ')[0]} ${counts.value[t]}`)
    .join(' · '),
);

function strengthClass(v) {
  if (v >= -60) return 'good';
  if (v >= -80) return 'mid';
  return 'weak';
}
</script>

<template>
  <div class="signal-list">
    <header class="signal-head">
      <div>
        <h2>附近无线信号</h2>
        <p class="signal-meta">
          <span v-if="fullScanning">
            全频扫描中 · {{ totalCount }} 条
            <template v-if="scanProgress"> · {{ Math.round(scanProgress.pct || 0) }}%</template>
            <template v-if="scanBand"> · {{ scanBand }}</template>
          </span>
          <span v-else-if="listening">监听中 · 实时 {{ totalCount }} 条<template v-if="scanBand"> · {{ scanBand }}</template></span>
          <template v-else-if="fullScanPhase === 'done'">
            <span>全频扫描完成 · 共 {{ totalCount }} 条信号</span>
          </template>
          <template v-else-if="startedAt">
            <span>已停止 · 开始于 {{ startedTimeText }}</span>
            <template v-if="countSummary">
              <span class="dot">·</span>
              <span>保留 {{ totalCount }} 条</span>
            </template>
          </template>
          <span v-else>尚未开始监听</span>
          <template v-if="listening && countSummary">
            <span class="dot">·</span>
            <span>{{ countSummary }}</span>
          </template>
        </p>
      </div>
    </header>

    <div v-if="!listening && !fullScanning && fullScanPhase !== 'done' && totalCount === 0" class="signal-empty">
      <p>点击左侧「一键扫全频」或「持续监听」开始探测</p>
      <p class="signal-empty-sub">全频扫描显示瀑布与已确认信号 · 持续监听在左侧列表实时更新</p>
    </div>

    <div v-else-if="(listening || fullScanning) && groups.length === 0" class="signal-empty">
      <p>监听中，等待信号…</p>
    </div>

    <div v-for="group in groups" :key="group.type" class="signal-group">
      <div class="signal-group-title">{{ group.label }} ({{ group.items.length }})</div>

      <div class="signal-table-wrap">
        <table class="signal-table">
          <thead>
            <tr>
              <th class="col-expand" />
              <th>标识</th>
              <th>频率</th>
              <th v-for="col in group.cols" :key="col.key">{{ col.label }}</th>
              <th>强度</th>
              <th class="col-play" />
            </tr>
          </thead>
          <tbody>
            <template v-for="item in group.items" :key="item.id">
              <tr class="signal-row" @click="toggleExpand(item.id)">
                <td class="col-expand">
                  <span class="expand-icon" :class="{ open: expanded.has(item.id) }">›</span>
                </td>
                <td class="label-cell">
                  {{ item.label }}
                  <span v-if="item.decoded" class="decoded-badge">已解调</span>
                </td>
                <td class="mono">{{ item.freq }}</td>
                <td v-for="col in group.cols" :key="col.key" class="mono">
                  <span v-if="['sec','rat','adv','proto','mode','fmt','df','label','mod','svc','dir','pass','sub'].includes(col.key)" class="tag">
                    {{ item.cols[col.key] }}
                  </span>
                  <template v-else>{{ item.cols[col.key] }}</template>
                </td>
                <td class="mono strength-cell" :class="strengthClass(item.strength)">
                  {{ item.strength }} dBm
                </td>
                <td class="col-play" @click.stop>
                  <SignalPlayLink :item="item" />
                </td>
              </tr>
              <tr v-if="expanded.has(item.id) && item.decoded && item.decode" class="decode-row">
                <td :colspan="5 + group.cols.length">
                  <SignalDecodePanel :item="item" />
                </td>
              </tr>
              <tr v-if="expanded.has(item.id)" class="detail-row">
                <td :colspan="5 + group.cols.length">
                  <div v-if="item.details?.length" class="detail-grid">
                    <div v-for="([k, v], idx) in item.details" :key="idx" class="detail-item">
                      <span class="detail-key">{{ k }}</span>
                      <span class="detail-val">{{ v }}</span>
                    </div>
                  </div>
                </td>
              </tr>
            </template>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<style scoped>
.signal-list {
  flex: 1;
  min-width: 0;
  min-height: 0;
  overflow-y: auto;
  padding: 16px 20px 24px;
}

.signal-head {
  margin-bottom: 20px;
}

.signal-head h2 {
  font-size: 15px;
  font-weight: 600;
  color: #eee;
  margin-bottom: 4px;
}

.signal-meta {
  font-size: 12px;
  color: #666;
}

.signal-meta .dot {
  margin: 0 6px;
  color: #444;
}

.signal-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 200px;
  color: #555;
  font-size: 13px;
}

.signal-empty-sub {
  font-size: 11px;
  color: #444;
}

.signal-group {
  margin-bottom: 28px;
}

.signal-group-title {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: #555;
  margin-bottom: 8px;
}

.signal-table-wrap {
  overflow-x: auto;
}

.signal-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}

.signal-table th {
  text-align: left;
  padding: 8px 10px;
  color: #666;
  font-weight: 500;
  border-bottom: 1px solid #1a1a1a;
  white-space: nowrap;
}

.signal-table td {
  padding: 9px 10px;
  color: #bbb;
  border-bottom: 1px solid #111;
}

.col-expand {
  width: 24px;
  padding: 9px 4px !important;
}

.col-play {
  width: 36px;
  padding: 9px 6px !important;
  text-align: right;
}

.strength-cell {
  white-space: nowrap;
}

.signal-row {
  cursor: pointer;
}

.signal-row:hover {
  background: rgba(255, 255, 255, 0.03);
}

.label-cell {
  color: #ddd;
  font-weight: 500;
  white-space: nowrap;
}

.decoded-badge {
  margin-left: 6px;
  padding: 1px 5px;
  border-radius: 3px;
  background: rgba(74, 120, 74, 0.35);
  color: #8ecf8e;
  font-size: 10px;
  font-weight: 500;
}

.expand-icon {
  display: inline-block;
  color: #555;
  font-size: 14px;
  transition: transform 0.12s;
}

.expand-icon.open {
  transform: rotate(90deg);
  color: #9cf;
}

.detail-row td {
  padding: 0 !important;
  background: #080808;
  border-bottom: 1px solid #1a1a1a;
}

.decode-row td {
  padding: 0 !important;
  background: #060806;
  border-bottom: 1px solid #1a1a1a;
}

.detail-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 6px 20px;
  padding: 12px 14px 14px 34px;
}

.detail-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.detail-key {
  font-size: 10px;
  color: #555;
  letter-spacing: 0.03em;
}

.detail-val {
  font-family: "SF Mono", "Menlo", monospace;
  font-size: 11px;
  color: #999;
  word-break: break-all;
}

.mono {
  font-family: "SF Mono", "Menlo", monospace;
  font-size: 11px;
  color: #999;
}

.tag {
  display: inline-block;
  padding: 2px 6px;
  border-radius: 3px;
  background: #151515;
  color: #888;
  font-size: 10px;
  font-family: inherit;
}

.good { color: #6c6; }
.mid { color: #ca8; }
.weak { color: #a66; }
</style>
