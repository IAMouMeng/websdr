<script setup>
import ScanSidebar from './components/ScanSidebar.vue';
import ScanDisplay from './components/ScanDisplay.vue';
import SignalList from './components/SignalList.vue';
import { provideProtocolScan } from './composables/useProtocolScan.js';

const { showScanPanel } = provideProtocolScan();
</script>

<template>
  <ScanSidebar />
  <main class="main protocol-main">
    <div class="protocol-body">
      <div class="protocol-left" :class="{ 'full-width': !showScanPanel }">
        <SignalList />
      </div>
      <ScanDisplay />
    </div>
  </main>
</template>

<style scoped>
.protocol-main {
  display: flex;
  flex-direction: column;
  background: #000;
  min-height: 0;
  overflow: hidden;
}

.protocol-body {
  flex: 1;
  display: flex;
  min-height: 0;
  overflow: hidden;
}

.protocol-left {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border-right: 1px solid #141414;
}

.protocol-left.full-width {
  border-right: none;
}
</style>
