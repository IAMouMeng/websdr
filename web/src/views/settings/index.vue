<script setup>
import { useReceiverControl } from '@/composables/useReceiverControl.js';

const { enabled, connected, statusText, busy, setEnabled } = useReceiverControl();

function onToggle() {
  setEnabled(!enabled.value);
}
</script>

<template>
  <div class="settings-page">
    <aside class="sidebar settings-sidebar">
      <div class="sidebar-head">
        <div class="sidebar-brand">
          <h1>设置</h1>
        </div>
        <div class="status" :class="{ ok: connected, err: !connected }">{{ statusText }}</div>
      </div>

      <div class="section">
        <div class="section-title">SDR 设备</div>
        <div class="setting-card">
          <div class="setting-row">
            <div class="setting-info">
              <div class="setting-label">启用 RTL-SDR</div>
              <div class="setting-desc">
                {{ enabled ? '设备正在采集射频数据' : '设备已停止，USB 已释放' }}
              </div>
            </div>
            <button
              type="button"
              class="toggle"
              :class="{ on: enabled }"
              :disabled="!connected || busy"
              :aria-pressed="enabled"
              aria-label="切换 SDR 设备"
              @click="onToggle"
            >
              <span class="toggle-thumb" />
            </button>
          </div>
        </div>
        <p class="setting-hint">
          关闭后 SDR 将停止工作并释放 USB 设备，其他程序可占用该 dongle。重新开启后恢复采集。
          无线电页面调谐频率低于 24 MHz 时会自动开启 Q 通道短波模式，≥24 MHz 自动切回 VHF/UHF。
        </p>
      </div>
    </aside>

    <main class="settings-main">
      <div class="settings-hero">
        <div class="hero-icon" :class="{ off: !enabled }">
          <svg viewBox="0 0 24 24" width="48" height="48" fill="none" stroke="currentColor" stroke-width="1.5">
            <rect x="3" y="5" width="18" height="14" rx="2" />
            <path d="M7 9h4M7 12h6M7 15h3" />
            <path v-if="!enabled" d="M4 4l16 16" stroke-width="2" />
          </svg>
        </div>
        <h2>{{ enabled ? 'SDR 运行中' : 'SDR 已关闭' }}</h2>
        <p v-if="enabled">无线电、协议扫描、ADS-B、AIS 等功能可正常使用。</p>
        <p v-else>请在左侧开启设备，或前往其他页面时也会保持此状态。</p>
      </div>
    </main>
  </div>
</template>

<style scoped>
.settings-page {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  width: 100%;
  height: 100%;
}

.settings-sidebar {
  width: 300px;
}

.setting-card {
  background: #111;
  border: 1px solid #1d1d1d;
  border-radius: 6px;
  padding: 12px;
}

.setting-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.setting-label {
  font-size: 13px;
  color: #ddd;
  margin-bottom: 4px;
}

.setting-desc {
  font-size: 11px;
  color: #666;
}

.setting-hint {
  margin-top: 10px;
  font-size: 11px;
  line-height: 1.5;
  color: #555;
}

.toggle {
  flex-shrink: 0;
  width: 44px;
  height: 26px;
  border: none;
  border-radius: 13px;
  background: #2a2a2a;
  padding: 2px;
  cursor: pointer;
  transition: background 0.2s;
}

.toggle.on {
  background: #3a6a3a;
}

.toggle:disabled {
  opacity: 0.4;
  cursor: default;
}

.toggle-thumb {
  display: block;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: #888;
  transition: transform 0.2s, background 0.2s;
}

.toggle.on .toggle-thumb {
  transform: translateX(18px);
  background: #cfc;
}

.settings-main {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #000;
  border-left: 1px solid #181818;
}

.settings-hero {
  text-align: center;
  max-width: 360px;
  padding: 24px;
}

.hero-icon {
  color: #6c6;
  margin-bottom: 16px;
}

.hero-icon.off {
  color: #666;
}

.settings-hero h2 {
  font-size: 18px;
  color: #eee;
  margin-bottom: 8px;
}

.settings-hero p {
  font-size: 13px;
  color: #666;
  line-height: 1.6;
}
</style>
