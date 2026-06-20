import { createRouter, createWebHashHistory } from 'vue-router';

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    {
      path: '/',
      component: () => import('@/layouts/default/index.vue'),
      children: [
        {
          path: '',
          redirect: '/radio',
        },
        {
          path: 'radio',
          name: 'radio',
          component: () => import('@/views/radio/index.vue'),
        },
        {
          path: 'ads-b',
          name: 'ads-b',
          component: () => import('@/views/ads-b/index.vue'),
        },
        {
          path: 'ais',
          name: 'ais',
          component: () => import('@/views/ais/index.vue'),
        },
        {
          path: 'protocol',
          name: 'protocol',
          component: () => import('@/views/protocol/index.vue'),
        },
        {
          path: 'apt',
          name: 'apt',
          component: () => import('@/views/apt/index.vue'),
        },
        {
          path: 'lrpt',
          name: 'lrpt',
          component: () => import('@/views/lrpt/index.vue'),
        },
        {
          path: 'meteor',
          redirect: '/satellite',
        },
        {
          path: 'satellite',
          name: 'satellite',
          component: () => import('@/views/meteor/index.vue'),
        },
        {
          path: 'settings',
          name: 'settings',
          component: () => import('@/views/settings/index.vue'),
        },
      ],
    },
  ],
});

export default router;
