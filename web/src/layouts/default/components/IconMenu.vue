<script setup>
import { useRoute } from 'vue-router';

const route = useRoute();

const menus = [
  { path: '/radio', title: '无线电接收', icon: 'radio' },
  { path: '/protocol', title: '协议分析', icon: 'protocol' },
  { path: '/satellite', title: '卫星解码', icon: 'satellite' },
  { path: '/ads-b', title: 'ADS-B', icon: 'ads-b' },
  { path: '/ais', title: 'AIS', icon: 'ais' },
];

const bottomMenus = [
  { path: '/settings', title: '设置', icon: 'settings' },
];
</script>

<template>
  <nav class="icon-menu" aria-label="主导航">
    <div class="icon-menu-brand" title="WebSDR">
      <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="1.8">
        <circle cx="12" cy="12" r="3" />
        <path d="M12 2v2M12 20v2M2 12h2M20 12h2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" />
      </svg>
    </div>
    <ul class="icon-menu-list">
      <li v-for="item in menus" :key="item.path">
        <router-link
          :to="item.path"
          class="icon-menu-item"
          :class="{ active: route.path === item.path || (item.path === '/satellite' && route.path === '/meteor') }"
          :title="item.title"
        >
          <!-- Radio -->
          <svg v-if="item.icon === 'radio'" viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.8">
            <path d="M4 18V6l8-3 8 3v12l-8 3-8-3z" />
            <path d="M8 10v4M12 9v6M16 10v4" />
          </svg>
          <!-- Protocol -->
          <svg v-else-if="item.icon === 'protocol'" viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.8">
            <path d="M4 7h16M4 12h10M4 17h6" />
            <circle cx="18" cy="12" r="2" />
            <path d="M20 7v10" />
          </svg>
          <!-- Satellite decode -->
          <svg v-else-if="item.icon === 'satellite' || item.icon === 'meteor'" viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="14" r="5.5" />
            <path d="M12 3v2.5M12 18.5V21" />
            <ellipse cx="12" cy="14" rx="9" ry="3" transform="rotate(-25 12 14)" />
            <path d="M4 8l2 1.2M20 8l-2 1.2" />
          </svg>
          <!-- ADS-B -->
          <svg v-else-if="item.icon === 'ads-b'" viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.8">
            <path d="M21 16v-2l-8-5V3.5a1.5 1.5 0 0 0-3 0V9l-8 5v2l8-2.5V19l-2 1.5V22l3.5-1 3.5 1v-1.5L13 19v-5.5l8 2.5z" />
          </svg>
          <!-- AIS -->
          <svg v-else-if="item.icon === 'ais'" viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.8">
            <path d="M3 18h18" />
            <path d="M5 18V9l4-2v11" />
            <path d="M9 18V7l4-2v13" />
            <path d="M13 18V9l4-2v11" />
            <path d="M17 18V11l2-1v8" />
            <path d="M7 9l2-3h6l2 3" />
          </svg>
        </router-link>
      </li>
    </ul>
    <ul class="icon-menu-list icon-menu-bottom">
      <li v-for="item in bottomMenus" :key="item.path">
        <router-link
          :to="item.path"
          class="icon-menu-item"
          :class="{ active: route.path === item.path }"
          :title="item.title"
        >
          <!-- Settings: filled gear, readable at 20px -->
          <svg v-if="item.icon === 'settings'" viewBox="0 0 24 24" width="20" height="20" fill="currentColor" aria-hidden="true">
            <path d="M19.14 12.94c.04-.31.06-.63.06-.94 0-.31-.02-.63-.06-.94l2.03-1.58a.49.49 0 0 0 .12-.61l-1.92-3.32a.488.488 0 0 0-.59-.22l-2.39.96c-.5-.38-1.03-.7-1.62-.94l-.36-2.54a.484.484 0 0 0-.48-.41h-3.84c-.24 0-.43.17-.47.41l-.36 2.54c-.59.24-1.13.57-1.62.94l-2.39-.96c-.22-.08-.47 0-.59.22L2.74 8.87c-.12.21-.08.47.12.61l2.03 1.58c-.04.31-.06.63-.06.94s.02.63.06.94l-2.03 1.58a.49.49 0 0 0-.12.61l1.92 3.32c.12.22.37.29.59.22l2.39-.96c.5.38 1.03.7 1.62.94l.36 2.54c.05.24.24.41.48.41h3.84c.24 0 .44-.17.47-.41l.36-2.54c.59-.24 1.13-.56 1.62-.94l2.39.96c.22.08.47 0 .59-.22l1.92-3.32c.12-.22.07-.47-.12-.61l-2.01-1.58zM12 15.6c-1.98 0-3.6-1.62-3.6-3.6s1.62-3.6 3.6-3.6 3.6 1.62 3.6 3.6-1.62 3.6-3.6 3.6z" />
          </svg>
        </router-link>
      </li>
    </ul>
  </nav>
</template>

<style scoped>
.icon-menu {
  width: 52px;
  flex-shrink: 0;
  background: #060606;
  border-right: 1px solid #181818;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 12px 0;
  gap: 8px;
  user-select: none;
}

.icon-menu-brand {
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #888;
  margin-bottom: 8px;
}

.icon-menu-list {
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 4px;
  width: 100%;
  padding: 0 8px;
}

.icon-menu-bottom {
  margin-top: auto;
}

.icon-menu-item {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  margin: 0 auto;
  border-radius: 8px;
  color: #666;
  text-decoration: none;
  transition: background 0.15s, color 0.15s;
}

.icon-menu-item:hover {
  background: rgba(255, 255, 255, 0.06);
  color: #ccc;
}

.icon-menu-item.active {
  background: rgba(102, 204, 255, 0.12);
  color: #9cf;
}
</style>
