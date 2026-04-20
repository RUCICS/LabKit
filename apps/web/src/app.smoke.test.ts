import { beforeEach, describe, expect, it } from 'vitest';

describe('app smoke', () => {
  beforeEach(() => {
    document.body.innerHTML = '<div id="app"></div>';
  });

  it('bootstraps the app shell and renders the page shell', async () => {
    await import('./main');
    const { router } = await import('./router');
    await router.isReady();

    const appShell = document.querySelector('[data-testid="app-shell"]');
    const shell = document.querySelector('[data-testid="page-shell"]');

    expect(appShell).not.toBeNull();
    expect(shell).not.toBeNull();
    expect(appShell?.textContent).toContain('LabKit');
    expect(appShell?.textContent).toContain('Profile');
  });
});
