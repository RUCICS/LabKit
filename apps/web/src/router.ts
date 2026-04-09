import { createRouter, createWebHistory } from 'vue-router';
import AdminLabsView from './views/AdminLabsView.vue';
import AdminQueueView from './views/AdminQueueView.vue';
import AuthConfirmView from './views/AuthConfirmView.vue';
import DeviceAuthView from './views/DeviceAuthView.vue';
import HistoryView from './views/HistoryView.vue';
import LabListView from './views/LabListView.vue';
import ProfileView from './views/ProfileView.vue';
import LeaderboardView from './views/LeaderboardView.vue';

export function createAppRouter(history = createWebHistory()) {
  return createRouter({
    history,
    routes: [
      {
        path: '/',
        name: 'home',
        component: LabListView
      },
      {
        path: '/labs/:labID/board',
        name: 'leaderboard',
        component: LeaderboardView,
        props: (route) => ({
          labId: String(route.params.labID)
        })
      },
      {
        path: '/labs/:labID/history',
        name: 'history',
        component: HistoryView,
        props: (route) => ({
          labId: String(route.params.labID)
        })
      },
      {
        path: '/auth/device',
        name: 'auth-device',
        component: DeviceAuthView
      },
      {
        path: '/auth/confirm',
        name: 'auth-confirm',
        component: AuthConfirmView
      },
      {
        path: '/devices',
        name: 'devices',
        component: ProfileView
      },
      {
        path: '/profile',
        redirect: '/devices'
      },
      {
        path: '/admin',
        redirect: (to) => ({
          path: '/admin/labs',
          query: to.query
        })
      },
      {
        path: '/admin/labs',
        name: 'admin-labs',
        component: AdminLabsView
      },
      {
        path: '/admin/labs/:labID/queue',
        name: 'admin-queue',
        component: AdminQueueView,
        props: (route) => ({
          labId: String(route.params.labID)
        })
      }
    ]
  });
}

export const router = createAppRouter();

export { createMemoryHistory } from 'vue-router';
