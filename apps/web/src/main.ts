import { computed, createApp, defineComponent, h, ref, watch } from 'vue';
import { RouterLink, RouterView } from 'vue-router';
import StatusBadge from './components/chrome/StatusBadge.vue';
import type { LeaderboardLabDetail } from './components/board/types';
import { readAPIError } from './lib/http';
import { getLabPhase, getLabSchedule, labPhaseLabel } from './lib/labs';
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

    const statusPhase = computed(() => (lab.value ? getLabPhase(getLabSchedule(lab.value.manifest)) : null));
    const showAdmin = computed(() => Boolean(sessionStorage.getItem('labkit_admin_token')));

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
                h(
                  RouterLink,
                  { to: '/', class: 'app-shell__brand-link' },
                  {
                    default: () => [
                      h('span', { class: 'app-shell__brand-icon', 'aria-hidden': 'true' }, 'L'),
                      h('span', { class: 'app-shell__brand' }, 'LabKit')
                    ]
                  }
                )
              ]),
              h('nav', { class: 'app-shell__utility', 'aria-label': 'Utility' }, [
                showAdmin.value ? h(RouterLink, { to: '/admin', class: 'app-shell__utility-link' }, { default: () => 'Admin' }) : null,
                h(RouterLink, { to: '/profile', class: 'app-shell__utility-link' }, { default: () => 'Profile' }),
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
