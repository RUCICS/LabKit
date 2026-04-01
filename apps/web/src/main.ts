import { computed, createApp, defineComponent, h } from 'vue';
import { RouterLink, RouterView } from 'vue-router';
import { createAppPinia } from './stores';
import { router } from './router';
import './styles/main.css';

const App = defineComponent({
  name: 'LabKitApp',
  setup() {
    const routeMeta = computed(() => {
      const route = router.currentRoute.value;
      const labId = typeof route.params.labID === 'string' ? route.params.labID : '';
      if (labId) {
        return labId;
      }
      switch (route.name) {
        case 'admin-labs':
          return 'admin';
        case 'profile':
          return 'profile';
        case 'auth-confirm':
          return 'auth';
        default:
          return 'catalog';
      }
    });

    return () =>
      h('div', { class: 'app-shell', 'data-testid': 'app-shell' }, [
        h('div', { class: 'app-shell__glow', 'aria-hidden': 'true' }),
        h('header', { class: 'app-shell__header' }, [
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
            h(RouterLink, { to: '/admin', class: 'app-shell__nav-link' }, { default: () => 'Admin' }),
            h(RouterLink, { to: '/profile', class: 'app-shell__nav-link' }, { default: () => 'History' })
          ]),
          h('div', { class: 'app-shell__status' }, [
            h('span', { class: 'app-shell__status-dot', 'aria-hidden': 'true' }),
            h('span', null, 'OPEN')
          ])
        ]),
        h('div', { class: 'app-shell__content' }, [h(RouterView)])
      ]);
  }
});

createApp(App).use(createAppPinia()).use(router).mount('#app');
