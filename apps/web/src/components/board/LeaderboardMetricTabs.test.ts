import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp, nextTick } from 'vue';
import LeaderboardMetricTabs from './LeaderboardMetricTabs.vue';

async function flushPromises() {
  await Promise.resolve();
  await Promise.resolve();
  await nextTick();
}

async function mountTabs() {
  const el = document.createElement('div');
  document.body.appendChild(el);
  const app = createApp(LeaderboardMetricTabs, {
    metrics: [
      { id: 'speed', name: 'Speed', sort: 'desc' },
      { id: 'memory', name: 'Memory', sort: 'asc' },
      { id: 'quality', name: 'Quality', sort: 'desc' }
    ],
    selectedMetricId: 'speed'
  });
  app.mount(el);
  await flushPromises();
  return {
    unmount() {
      app.unmount();
      el.remove();
    }
  };
}

beforeEach(() => {
  document.body.innerHTML = '';
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('LeaderboardMetricTabs', () => {
  it('assigns positional tone colors to each tab dot', async () => {
    const view = await mountTabs();
    const dots = Array.from(document.querySelectorAll('.board-tabs__dot'));

    expect(dots[0]?.className).toContain('amber');
    expect(dots[1]?.className).toContain('cyan');
    expect(dots[2]?.className).toContain('purple');

    view.unmount();
  });
});
