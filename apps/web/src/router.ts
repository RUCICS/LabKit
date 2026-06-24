import { createRouter, createWebHistory } from 'vue-router';
import AdminLabsView from './views/AdminLabsView.vue';
import AdminLoginView from './views/AdminLoginView.vue';
import AdminQueueView from './views/AdminQueueView.vue';
import AuthConfirmView from './views/AuthConfirmView.vue';
import DeviceAuthView from './views/DeviceAuthView.vue';
import GradeView from './views/GradeView.vue';
import HistoryView from './views/HistoryView.vue';
import LabListView from './views/LabListView.vue';
import LoginView from './views/LoginView.vue';
import ProfileView from './views/ProfileView.vue';
import LeaderboardView from './views/LeaderboardView.vue';
import { readAdminToken, sessionToken } from './lib/admin';

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
        path: '/login',
        name: 'login',
        component: LoginView
      },
      {
        path: '/grade',
        name: 'grade',
        component: GradeView
      },
      {
        path: '/labs/:labID/grade',
        name: 'lab-grade',
        component: GradeView,
        props: (route) => ({
          labId: String(route.params.labID)
        })
      },
      {
        path: '/profile',
        name: 'profile',
        component: ProfileView
      },
      {
        path: '/devices',
        redirect: '/profile'
      },
      {
        path: '/admin',
        redirect: (to) => ({
          path: '/admin/labs',
          query: to.query
        })
      },
      {
        path: '/admin/login',
        name: 'admin-login',
        component: AdminLoginView
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

router.beforeEach((to) => {
  if (!to.path.startsWith('/admin')) return;
  if (to.name === 'admin-login') return;

  // ?token=xxx shortcut: store as session-only (don't persist URL-delivered tokens) and strip from URL
  if (typeof to.query.token === 'string' && to.query.token.trim() !== '') {
    sessionToken(to.query.token.trim());
    const { token: _removed, ...rest } = to.query;
    return { ...to, query: rest, replace: true };
  }

  if (!readAdminToken()) {
    return { name: 'admin-login' };
  }
});

export { createMemoryHistory } from 'vue-router';
