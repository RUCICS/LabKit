import { afterEach, describe, expect, it, vi } from 'vitest';
import { createApp, defineComponent, h, nextTick } from 'vue';
import { createMemoryHistory, createRouter } from 'vue-router';
import LoginView from './LoginView.vue';

async function flush() {
  await Promise.resolve();
  await Promise.resolve();
  await nextTick();
}

function jsonResponse(payload: unknown, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => payload,
    text: async () => JSON.stringify(payload)
  } as Response;
}

async function mountLoginView(url = '/login') {
  const el = document.createElement('div');
  document.body.appendChild(el);
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/login', component: LoginView }]
  });
  const app = createApp(defineComponent({ render: () => h('div', [h(LoginView)]) }));
  app.use(router);
  await router.push(url);
  await router.isReady();
  app.mount(el);
  await flush();
  return {
    unmount() {
      app.unmount();
      el.remove();
    }
  };
}

afterEach(() => {
  document.body.innerHTML = '';
  vi.restoreAllMocks();
});

describe('LoginView', () => {
  it('shows the 微人大 login button when not signed in', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse({ error: { message: 'unauthorized' } }, 401)));

    const view = await mountLoginView('/login');

    expect(document.querySelector('[data-testid="login-cas"]')).not.toBeNull();
    expect(document.body.textContent).toContain('微人大登录');

    view.unmount();
  });

  it('shows a continue action when already signed in', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse({ user_id: 7, student_id: '2026001', nickname: 'Aki' })));

    const view = await mountLoginView('/login');

    expect(document.body.textContent).toContain('已登录为 2026001');
    expect(document.body.textContent).toContain('继续查看成绩');

    view.unmount();
  });
});
