<script setup>
import { useRoute } from 'vue-router';
import IconMenu from './components/IconMenu.vue';

const route = useRoute();
</script>

<template>
  <div class="app-layout">
    <IconMenu />
    <div class="layout-content">
      <router-view v-slot="{ Component }">
        <Transition name="page-fade" mode="out-in">
          <div :key="route.path" class="page-shell">
            <component :is="Component" />
          </div>
        </Transition>
      </router-view>
    </div>
  </div>
</template>

<style scoped>
.app-layout {
  display: flex;
  width: 100%;
  height: 100%;
  overflow: hidden;
}

.layout-content {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  overflow: hidden;
  position: relative;
}

.page-shell {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  width: 100%;
  height: 100%;
}

.page-fade-enter-active,
.page-fade-leave-active {
  transition: opacity 0.12s ease;
}

.page-fade-enter-from,
.page-fade-leave-to {
  opacity: 0;
}
</style>
