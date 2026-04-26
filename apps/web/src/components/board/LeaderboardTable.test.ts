import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp, nextTick } from 'vue';
import LeaderboardTable from './LeaderboardTable.vue';

async function flushPromises() {
  await Promise.resolve();
  await Promise.resolve();
  await nextTick();
}

async function mountTable(props: {
  rows: Array<{
    rank: number;
    nickname: string;
    track?: string;
    scores: Array<{ metric_id: string; value: number }>;
    updated_at: string;
  }>;
  metrics: Array<{ id: string; name: string; sort: 'asc' | 'desc' }>;
  selectedMetricId: string;
  apiHint?: string;
}) {
  const el = document.createElement('div');
  document.body.appendChild(el);
  const app = createApp(LeaderboardTable, props);
  app.mount(el);
  await flushPromises();
  return {
    el,
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

describe('LeaderboardTable', () => {
  it('renders invalid timestamps safely', async () => {
    const view = await mountTable({
      rows: [
        {
          rank: 1,
          nickname: 'Bob',
          scores: [
            { metric_id: 'runtime_ms', value: 88 },
            { metric_id: 'latency_ms', value: 35 }
          ],
          updated_at: 'not-a-timestamp'
        }
      ],
      metrics: [
        { id: 'runtime_ms', name: 'Runtime', sort: 'desc' },
        { id: 'latency_ms', name: 'Latency', sort: 'asc' }
      ],
      selectedMetricId: 'runtime_ms',
      apiHint: 'GET /api/labs/sorting/board'
    });

    expect(document.body.textContent).toContain('Bob');
    expect(document.body.textContent).toContain('—');

    view.unmount();
  });

  it('dims and un-ranks rows that did not pick the selected track', async () => {
    const view = await mountTable({
      rows: [
        {
          rank: 1,
          nickname: 'Alice',
          track: 'runtime_ms',
          scores: [
            { metric_id: 'runtime_ms', value: 92 },
            { metric_id: 'latency_ms', value: 18 }
          ],
          updated_at: '2026-03-31T09:00:00Z'
        },
        {
          rank: 2,
          nickname: 'Bob',
          track: 'latency_ms',
          scores: [
            { metric_id: 'runtime_ms', value: 89 },
            { metric_id: 'latency_ms', value: 15 }
          ],
          updated_at: '2026-03-31T10:00:00Z'
        },
        {
          rank: 3,
          nickname: 'NoTrack',
          scores: [
            { metric_id: 'runtime_ms', value: 70 },
            { metric_id: 'latency_ms', value: 20 }
          ],
          updated_at: '2026-03-31T11:00:00Z'
        }
      ],
      metrics: [
        { id: 'runtime_ms', name: 'Runtime', sort: 'desc' },
        { id: 'latency_ms', name: 'Latency', sort: 'asc' }
      ],
      selectedMetricId: 'runtime_ms',
      apiHint: 'GET /api/labs/sorting/board'
    });

    const rows = document.querySelectorAll('[data-testid="leaderboard-row"]');
    expect(rows).toHaveLength(3);

    expect(rows[0]?.textContent).toContain('1');
    expect(rows[0]?.classList.contains('board-table__row--unranked')).toBe(false);

    expect(rows[1]?.textContent).toContain('—');
    expect(rows[1]?.classList.contains('board-table__row--unranked')).toBe(true);

    expect(rows[2]?.textContent).toContain('—');
    expect(rows[2]?.classList.contains('board-table__row--unranked')).toBe(true);

    view.unmount();
  });

  it('omits the Track column when the board is not track-based', async () => {
    const view = await mountTable({
      rows: [
        {
          rank: 1,
          nickname: 'Ada',
          scores: [
            { metric_id: 'runtime_ms', value: 92 },
            { metric_id: 'latency_ms', value: 18 }
          ],
          updated_at: '2026-03-31T09:00:00Z'
        }
      ],
      metrics: [
        { id: 'runtime_ms', name: 'Runtime', sort: 'desc' },
        { id: 'latency_ms', name: 'Latency', sort: 'asc' }
      ],
      selectedMetricId: 'runtime_ms',
      apiHint: 'GET /api/labs/sorting/board'
    });

    expect(document.body.textContent).not.toContain('Track');
    expect(document.body.textContent).toContain('Ada');

    view.unmount();
  });
});
