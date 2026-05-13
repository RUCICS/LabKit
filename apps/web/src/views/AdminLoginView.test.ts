import { afterEach, describe, expect, it, vi } from 'vitest';
import { createApp, nextTick } from 'vue';
import { createMemoryHistory, createRouter } from 'vue-router';
import AdminLoginView from './AdminLoginView.vue';
import { adminTokenStorageKey } from '../lib/admin';

async function flush() {
  await Promise.resolve();
  await Promise.resolve();
  await nextTick();
}

afterEach(() => {
  document.body.innerHTML = '';
  vi.restoreAllMocks();
  window.sessionStorage.clear();
  window.localStorage.clear();
});

describe('AdminLoginView', () => {
  it('stores token in sessionStorage when remember is off and redirects to /admin/labs', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/admin/login', name: 'admin-login', component: AdminLoginView },
        { path: '/admin/labs', name: 'admin-labs', component: { template: '<div>labs</div>' } }
      ]
    });
    await router.push('/admin/login');
    await router.isReady();

    const el = document.createElement('div');
    document.body.appendChild(el);
    const app = createApp(AdminLoginView);
    app.use(router);
    app.mount(el);
    await flush();

    const input = document.querySelector('input[type="password"]') as HTMLInputElement;
    const submit = document.querySelector('button[type="submit"]') as HTMLButtonElement;
    expect(input).not.toBeNull();
    expect(submit).not.toBeNull();

    input.value = 'my-secret-token';
    input.dispatchEvent(new Event('input', { bubbles: true }));

    const navigationDone = new Promise<void>((resolve) => {
      const off = router.afterEach(() => { off(); resolve(); });
    });
    submit.click();
    await navigationDone;
    await flush();

    expect(window.sessionStorage.getItem(adminTokenStorageKey)).toBe('my-secret-token');
    expect(window.localStorage.getItem(adminTokenStorageKey)).toBeNull();
    expect(router.currentRoute.value.name).toBe('admin-labs');

    app.unmount();
    el.remove();
  });

  it('stores token in localStorage when remember is on', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/admin/login', name: 'admin-login', component: AdminLoginView },
        { path: '/admin/labs', name: 'admin-labs', component: { template: '<div>labs</div>' } }
      ]
    });
    await router.push('/admin/login');
    await router.isReady();

    const el = document.createElement('div');
    document.body.appendChild(el);
    const app = createApp(AdminLoginView);
    app.use(router);
    app.mount(el);
    await flush();

    const input = document.querySelector('input[type="password"]') as HTMLInputElement;
    const toggle = document.querySelector('input[type="checkbox"]') as HTMLInputElement;
    const submit = document.querySelector('button[type="submit"]') as HTMLButtonElement;

    toggle.checked = true;
    toggle.dispatchEvent(new Event('change', { bubbles: true }));
    input.value = 'persistent-token';
    input.dispatchEvent(new Event('input', { bubbles: true }));
    submit.click();
    await flush();

    expect(window.localStorage.getItem(adminTokenStorageKey)).toBe('persistent-token');
    expect(window.sessionStorage.getItem(adminTokenStorageKey)).toBeNull();

    app.unmount();
    el.remove();
  });
});
