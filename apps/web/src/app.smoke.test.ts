import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { nextTick } from 'vue';
import { refreshSession } from './lib/session';

describe('app smoke', () => {
  beforeEach(() => {
    document.body.innerHTML = '<div id="app"></div>';
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('bootstraps the app shell and renders the page shell', async () => {
    // Signed out: probing the session returns 401.
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => ({ ok: false, status: 401, json: async () => ({}), text: async () => '' }) as Response)
    );

    await import('./main');
    const { router } = await import('./router');
    await router.isReady();
    await refreshSession();
    await nextTick();

    const appShell = document.querySelector('[data-testid="app-shell"]');
    const shell = document.querySelector('[data-testid="page-shell"]');

    expect(appShell).not.toBeNull();
    expect(shell).not.toBeNull();
    expect(appShell?.textContent).toContain('LabKit');
    // Signed out → the header offers sign-in.
    expect(appShell?.textContent).toContain('Sign in');
  });
});
