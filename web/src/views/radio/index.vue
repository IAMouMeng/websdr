<script setup>
import { useRoute } from 'vue-router';
import Sidebar from './components/Sidebar.vue';
import TopBar from './components/TopBar.vue';
import DisplayArea from './components/DisplayArea.vue';
import { provideSdrApp } from './composables/useSdrApp.js';

const route = useRoute();
const tune = Number(route.query.tune);
const mode = typeof route.query.mode === 'string' ? route.query.mode : null;

provideSdrApp({
  pendingTune: Number.isFinite(tune) && tune > 0 ? tune : null,
  pendingMode: mode,
});
</script>

<template>
  <aside class="sidebar">
    <Sidebar />
  </aside>
  <main class="main">
    <TopBar />
    <DisplayArea />
  </main>
</template>
