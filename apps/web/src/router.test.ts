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
});
