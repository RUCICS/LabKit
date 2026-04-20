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
      { id: 'throughput', name: 'Throughput', sort: 'desc' },
      { id: 'latency', name: 'Latency', sort: 'asc' },
      { id: 'fairness', name: 'Fairness', sort: 'desc' }
    ],
    selectedMetricId: 'throughput'
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
  it('uses per-metric visual tones for each tab', async () => {
    const view = await mountTabs();
    const dots = Array.from(document.querySelectorAll('.board-tabs__dot'));

    expect(dots[0]?.className).toContain('throughput');
    expect(dots[1]?.className).toContain('latency');
    expect(dots[2]?.className).toContain('fairness');

    view.unmount();
  });
});
