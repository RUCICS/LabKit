import { computed, createApp, defineComponent, h, ref, watch } from 'vue';
import { RouterLink, RouterView } from 'vue-router';
import StatusBadge from './components/chrome/StatusBadge.vue';
import type { LeaderboardLabDetail } from './components/board/types';
import { readAPIError } from './lib/http';
import { getLabPhase, labPhaseLabel } from './lib/labs';
import { createAppPinia } from './stores';
import { router } from './router';
import './styles/main.css';

const App = defineComponent({
  name: 'LabKitApp',
  setup() {
    const lab = ref<LeaderboardLabDetail | null>(null);
    let requestSeq = 0;

    const isAuthRoute = computed(() => {
      const name = router.currentRoute.value.name;
      return name === 'auth-device' || name === 'auth-confirm';
    });

    const currentLabId = computed(() => {
      const value = router.currentRoute.value.params.labID;
      return typeof value === 'string' ? value : '';
    });

    const routeMeta = computed(() => {
      const route = router.currentRoute.value;
      if (currentLabId.value) {
        return currentLabId.value;
      }
      switch (route.name) {
        case 'admin-labs':
          return 'admin';
        case 'devices':
          return 'devices';
        case 'auth-device':
        case 'auth-confirm':
          return 'auth';
        default:
          return 'catalog';
      }
    });

    const showHistoryLink = computed(() => currentLabId.value !== '');
    const statusPhase = computed(() => (lab.value ? getLabPhase(lab.value.manifest?.schedule) : null));

    async function loadLabContext(labId: string) {
      const requestId = ++requestSeq;
      if (!labId) {
        lab.value = null;
        return;
      }
      try {
        const response = await fetch(`/api/labs/${encodeURIComponent(labId)}`);
        if (requestId !== requestSeq) {
          return;
        }
        if (!response.ok) {
          throw new Error(await readAPIError(response, 'Failed to load lab context'));
        }
        lab.value = (await response.json()) as LeaderboardLabDetail;
      } catch {
        if (requestId === requestSeq) {
          lab.value = null;
        }
      }
    }

    watch(
      () => currentLabId.value,
      (labId) => {
        void loadLabContext(labId);
      },
      { immediate: true }
    );

    return () =>
      h('div', { class: 'app-shell', 'data-testid': 'app-shell' }, [
        h('div', { class: 'app-shell__glow', 'aria-hidden': 'true' }),
        !isAuthRoute.value
          ? h('header', { class: 'app-shell__header' }, [
              h('div', { class: 'app-shell__brand-lockup' }, [
                h('span', { class: 'app-shell__brand-icon', 'aria-hidden': 'true' }, 'L'),
                h('div', { class: 'app-shell__brand-copy' }, [
                  h('span', { class: 'app-shell__brand' }, 'LabKit'),
                  h('span', { class: 'app-shell__brand-divider', 'aria-hidden': 'true' }, '/'),
                  h('span', { class: 'app-shell__brand-meta' }, routeMeta.value)
                ])
              ]),
              h('nav', { class: 'app-shell__nav', 'aria-label': 'Primary', 'data-testid': 'app-shell-nav' }, [
                h(RouterLink, { to: '/', class: 'app-shell__nav-link' }, { default: () => 'Labs' }),
                showHistoryLink.value
                  ? h(
                      RouterLink,
                      { to: `/labs/${currentLabId.value}/history`, class: 'app-shell__nav-link' },
                      { default: () => 'History' }
                    )
                  : null,
                h(RouterLink, { to: '/devices', class: 'app-shell__nav-link' }, { default: () => 'Devices' }),
                h(RouterLink, { to: '/admin', class: 'app-shell__nav-link' }, { default: () => 'Admin' }),
              ]),
              statusPhase.value
                ? h(StatusBadge, {
                    label: labPhaseLabel(statusPhase.value),
                    tone: statusPhase.value,
                    class: 'app-shell__status'
                  })
                : null
            ])
          : null,
        h('div', { class: ['app-shell__content', isAuthRoute.value ? 'app-shell__content--auth' : ''] }, [
          h(RouterView)
        ])
      ]);
  }
});

createApp(App).use(createAppPinia()).use(router).mount('#app');
