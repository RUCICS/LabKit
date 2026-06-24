import { describe, expect, it } from 'vitest';
import { createAppRouter, createMemoryHistory } from './router';

describe('router', () => {
  it('redirects /admin to the admin labs screen', async () => {
    const router = createAppRouter(createMemoryHistory());

    await router.push('/admin?token=secret');
    await router.isReady();

    expect(router.currentRoute.value.name).toBe('admin-labs');
    expect(router.currentRoute.value.path).toBe('/admin/labs');
    expect(router.currentRoute.value.query.token).toBe('secret');
  });

  it('routes /auth/device to the device auth screen', async () => {
    const router = createAppRouter(createMemoryHistory());

    await router.push('/auth/device?user_code=ABCD-EFGH');
    await router.isReady();

    expect(router.currentRoute.value.name).toBe('auth-device');
    expect(router.currentRoute.value.path).toBe('/auth/device');
    expect(router.currentRoute.value.query.user_code).toBe('ABCD-EFGH');
  });

  it('routes lab history URLs to the history screen', async () => {
    const router = createAppRouter(createMemoryHistory());

    await router.push('/labs/sorting/history');
    await router.isReady();

    expect(router.currentRoute.value.name).toBe('history');
    expect(router.currentRoute.value.params.labID).toBe('sorting');
  });

  it('redirects the legacy /devices path to /profile', async () => {
    const router = createAppRouter(createMemoryHistory());

    await router.push('/devices');
    await router.isReady();

    expect(router.currentRoute.value.name).toBe('profile');
    expect(router.currentRoute.value.path).toBe('/profile');
  });

  it('routes /profile to the profile screen', async () => {
    const router = createAppRouter(createMemoryHistory());

    await router.push('/profile');
    await router.isReady();

    expect(router.currentRoute.value.name).toBe('profile');
    expect(router.currentRoute.value.path).toBe('/profile');
  });

  it('routes /login to the login screen', async () => {
    const router = createAppRouter(createMemoryHistory());

    await router.push('/login?next=/grade');
    await router.isReady();

    expect(router.currentRoute.value.name).toBe('login');
    expect(router.currentRoute.value.query.next).toBe('/grade');
  });

  it('routes /grade to the grade screen', async () => {
    const router = createAppRouter(createMemoryHistory());

    await router.push('/grade');
    await router.isReady();

    expect(router.currentRoute.value.name).toBe('grade');
    expect(router.currentRoute.value.path).toBe('/grade');
  });

  it('routes lab-scoped grade URLs to the grade screen', async () => {
    const router = createAppRouter(createMemoryHistory());

    await router.push('/labs/colab-2026-p2/grade');
    await router.isReady();

    expect(router.currentRoute.value.name).toBe('lab-grade');
    expect(router.currentRoute.value.params.labID).toBe('colab-2026-p2');
  });
});
