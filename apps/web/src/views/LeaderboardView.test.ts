import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp, nextTick } from 'vue';
import LeaderboardView from './LeaderboardView.vue';

type BoardPayload = {
  lab_id: string;
  selected_metric: string;
  metrics: Array<{ id: string; name: string; sort: 'asc' | 'desc'; selected?: boolean }>;
  rows: Array<{
    rank: number;
    nickname: string;
    track?: string;
    scores: Array<{ metric_id: string; value: number }>;
    updated_at: string;
  }>;
};

type LabPayload = {
  id: string;
  name: string;
  manifest?: {
    schedule?: {
      close?: string;
    };
  };
};

const labPayload: LabPayload = {
  id: 'sorting',
  name: 'CoLab 调度器竞赛',
  manifest: {
    schedule: {
      close: '2026-06-01T00:00:00Z'
    }
  }
};

const boardByMetric: Record<string, BoardPayload> = {
  runtime_ms: {
    lab_id: 'sorting',
    selected_metric: 'runtime_ms',
    metrics: [
      { id: 'runtime_ms', name: 'Runtime', sort: 'desc', selected: true },
      { id: 'latency_ms', name: 'Latency', sort: 'asc', selected: false }
    ],
    rows: [
      {
        rank: 1,
        nickname: 'Bob',
        scores: [
          { metric_id: 'runtime_ms', value: 88 },
          { metric_id: 'latency_ms', value: 35 }
        ],
        updated_at: '2026-03-31T10:00:00Z'
      },
      {
        rank: 2,
        nickname: 'Ada',
        scores: [
          { metric_id: 'runtime_ms', value: 92 },
          { metric_id: 'latency_ms', value: 50 }
        ],
        updated_at: '2026-03-31T11:00:00Z'
      }
    ]
  },
  latency_ms: {
    lab_id: 'sorting',
    selected_metric: 'latency_ms',
    metrics: [
      { id: 'runtime_ms', name: 'Runtime', sort: 'desc', selected: false },
      { id: 'latency_ms', name: 'Latency', sort: 'asc', selected: true }
    ],
    rows: [
      {
        rank: 1,
        nickname: 'Ada',
        scores: [
          { metric_id: 'runtime_ms', value: 92 },
          { metric_id: 'latency_ms', value: 18 }
        ],
        updated_at: '2026-03-31T09:00:00Z'
      },
      {
        rank: 2,
        nickname: 'Bob',
        scores: [
          { metric_id: 'runtime_ms', value: 88 },
          { metric_id: 'latency_ms', value: 35 }
        ],
        updated_at: '2026-03-31T10:00:00Z'
      }
    ]
  }
};

const overlappingBoards: Record<string, BoardPayload> = {
  runtime_ms: {
    lab_id: 'sorting',
    selected_metric: 'runtime_ms',
    metrics: [
      { id: 'runtime_ms', name: 'Runtime', sort: 'desc', selected: true },
      { id: 'latency_ms', name: 'Latency', sort: 'asc', selected: false },
      { id: 'accuracy_ms', name: 'Accuracy', sort: 'desc', selected: false }
    ],
    rows: [
      {
        rank: 1,
        nickname: 'Bob',
        scores: [
          { metric_id: 'runtime_ms', value: 88 },
          { metric_id: 'latency_ms', value: 35 },
          { metric_id: 'accuracy_ms', value: 99 }
        ],
        updated_at: '2026-03-31T10:00:00Z'
      },
      {
        rank: 2,
        nickname: 'Ada',
        scores: [
          { metric_id: 'runtime_ms', value: 92 },
          { metric_id: 'latency_ms', value: 50 },
          { metric_id: 'accuracy_ms', value: 96 }
        ],
        updated_at: '2026-03-31T11:00:00Z'
      }
    ]
  },
  latency_ms: {
    lab_id: 'sorting',
    selected_metric: 'latency_ms',
    metrics: [
      { id: 'runtime_ms', name: 'Runtime', sort: 'desc', selected: false },
      { id: 'latency_ms', name: 'Latency', sort: 'asc', selected: true },
      { id: 'accuracy_ms', name: 'Accuracy', sort: 'desc', selected: false }
    ],
    rows: [
      {
        rank: 1,
        nickname: 'Ada',
        scores: [
          { metric_id: 'runtime_ms', value: 92 },
          { metric_id: 'latency_ms', value: 18 },
          { metric_id: 'accuracy_ms', value: 96 }
        ],
        updated_at: '2026-03-31T09:00:00Z'
      }
    ]
  },
  accuracy_ms: {
    lab_id: 'sorting',
    selected_metric: 'accuracy_ms',
    metrics: [
      { id: 'runtime_ms', name: 'Runtime', sort: 'desc', selected: false },
      { id: 'latency_ms', name: 'Latency', sort: 'asc', selected: false },
      { id: 'accuracy_ms', name: 'Accuracy', sort: 'desc', selected: true }
    ],
    rows: [
      {
        rank: 1,
        nickname: 'Cara',
        scores: [
          { metric_id: 'runtime_ms', value: 94 },
          { metric_id: 'latency_ms', value: 22 },
          { metric_id: 'accuracy_ms', value: 100 }
        ],
        updated_at: '2026-03-31T08:00:00Z'
      }
    ]
  }
};

function makeResponse(payload: BoardPayload | { detail: string }, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => payload,
    text: async () => JSON.stringify(payload)
  } as Response;
}

function makeLabResponse(payload: LabPayload, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => payload,
    text: async () => JSON.stringify(payload)
  } as Response;
}

function makeBoardErrorResponse(
  payload: { error: { code: string; message: string; request_id: string } },
  status = 200
) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => payload,
    text: async () => JSON.stringify(payload)
  } as Response;
}

async function flushPromises() {
  await Promise.resolve();
  await Promise.resolve();
  await nextTick();
}

async function waitForBodyText(text: string) {
  for (let i = 0; i < 10; i += 1) {
    await flushPromises();
    if (document.body.textContent?.includes(text)) {
      return;
    }
  }
  throw new Error(`Expected body text to include ${text}`);
}

async function mountBoard(labId = 'sorting') {
  const el = document.createElement('div');
  document.body.appendChild(el);
  const app = createApp(LeaderboardView, { labId });
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

describe('LeaderboardView', () => {
  it('renders leaderboard rows', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.includes('/api/labs/sorting/board')) {
          return makeResponse(boardByMetric.runtime_ms);
        }
        expect(url).toContain('/api/labs/sorting');
        return makeLabResponse(labPayload);
      })
    );

    const view = await mountBoard();

    expect(document.body.textContent).toContain('CoLab 调度器竞赛');
    expect(document.body.textContent).toContain('Participants');
    expect(document.body.textContent).toContain('2');
    expect(document.body.textContent).toContain('Bob');
    expect(document.body.textContent).toContain('88');
    expect(document.body.textContent).toContain('35');
    expect(document.body.textContent).toContain('Runtime');
    expect(document.body.textContent).toContain('Latency');
    expect(document.body.textContent).toContain('Last update');
    expect(document.body.textContent).toContain('GET /api/labs/sorting/board');

    view.unmount();
  });

  it('switches metrics when a tab is selected', async () => {
    const fetchSpy = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.includes('by=latency_ms')) {
        return makeResponse(boardByMetric.latency_ms);
      }
      if (url.includes('/board')) {
        return makeResponse(boardByMetric.runtime_ms);
      }
      return makeLabResponse(labPayload);
    });
    vi.stubGlobal('fetch', fetchSpy);

    const view = await mountBoard();
    const latencyTab = Array.from(document.querySelectorAll('button')).find((button) =>
      button.textContent?.includes('Latency')
    ) as HTMLButtonElement | undefined;

    expect(latencyTab).toBeDefined();
    latencyTab?.click();
    await waitForBodyText('Ada');

    expect(fetchSpy).toHaveBeenCalledTimes(4);
    expect(document.body.textContent).toContain('Ada');
    expect(document.body.textContent).toContain('18');

    view.unmount();
  });

  it('ignores stale metric responses that resolve out of order', async () => {
    const pending = new Map<string, (value: Response) => void>();
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        return new Promise<Response>((resolve) => {
          if (url.includes('by=latency_ms')) {
            pending.set('latency_ms', resolve);
            return;
          }
          if (url.includes('by=accuracy_ms')) {
            pending.set('accuracy_ms', resolve);
            return;
          }
          if (url.endsWith('/api/labs/sorting')) {
            resolve(makeLabResponse(labPayload));
            return;
          }
          pending.set('runtime_ms', resolve);
        });
      })
    );

    const view = await mountBoard();
    pending.get('runtime_ms')?.(makeResponse(overlappingBoards.runtime_ms));
    await waitForBodyText('Accuracy');

    const latencyTab = Array.from(document.querySelectorAll('button')).find((button) =>
      button.textContent?.includes('Latency')
    ) as HTMLButtonElement | undefined;
    const accuracyTab = Array.from(document.querySelectorAll('button')).find((button) =>
      button.textContent?.includes('Accuracy')
    ) as HTMLButtonElement | undefined;

    expect(latencyTab).toBeDefined();
    expect(accuracyTab).toBeDefined();

    latencyTab?.click();
    accuracyTab?.click();
    await flushPromises();

    pending.get('accuracy_ms')?.(makeResponse(overlappingBoards.accuracy_ms));
    await waitForBodyText('Cara');
    expect(document.body.textContent).toContain('100');

    pending.get('latency_ms')?.(makeResponse(overlappingBoards.latency_ms));
    await waitForBodyText('Cara');
    expect(document.body.textContent).toContain('100');

    view.unmount();
  });

  it('shows an empty board state', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.includes('/board')) {
          return makeResponse({
            ...boardByMetric.runtime_ms,
            rows: []
          });
        }
        return makeLabResponse(labPayload);
      })
    );

    const view = await mountBoard();

    expect(document.body.textContent).toContain('No leaderboard entries yet');
    expect(document.querySelector('[role="tablist"]')).not.toBeNull();
    expect(document.body.textContent).toContain('Runtime');
    expect(document.body.textContent).toContain('Latency');

    view.unmount();
  });

  it('shows the hidden board state before visibility', async () => {
    const fetchSpy = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.includes('/board')) {
        return makeBoardErrorResponse({
          error: {
            code: 'lab_hidden',
            message: 'Leaderboard is hidden',
            request_id: 'request-hidden'
          }
        }, 404);
      }
      return makeLabResponse(labPayload);
    });
    vi.stubGlobal('fetch', fetchSpy);

    const view = await mountBoard('hidden');

    expect(document.body.textContent).toContain('Leaderboard is hidden for now');

    view.unmount();
  });

  it('does not misrepresent an unknown lab URL as hidden', async () => {
    const fetchSpy = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.includes('/board')) {
        return makeBoardErrorResponse({
          error: {
            code: 'lab_not_found',
            message: 'Lab not found',
            request_id: 'request-missing'
          }
        }, 404);
      }
      return makeLabResponse(labPayload);
    });
    vi.stubGlobal('fetch', fetchSpy);

    const view = await mountBoard('missing');

    expect(document.body.textContent).not.toContain('Leaderboard is hidden for now');
    expect(document.body.textContent).toContain('Lab not found');

    view.unmount();
  });

  it('does not render metric tabs for a single-metric lab', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.includes('/board')) {
          return makeResponse({
            lab_id: 'local-smoke',
            selected_metric: 'score',
            metrics: [{ id: 'score', name: 'Score', sort: 'desc', selected: true }],
            rows: [
              {
                rank: 1,
                nickname: '哈基米',
                scores: [{ metric_id: 'score', value: 95 }],
                updated_at: '2026-04-01T06:00:32Z'
              }
            ]
          });
        }
        return makeLabResponse({
          id: 'local-smoke',
          name: 'Local Smoke',
          manifest: {
            schedule: {
              close: '2027-01-01T00:00:00Z'
            }
          }
        });
      })
    );

    const view = await mountBoard('local-smoke');

    expect(document.querySelector('[role="tablist"]')).toBeNull();
    expect(document.body.textContent).toContain('Local Smoke');
    expect(document.body.textContent).toContain('Score');

    view.unmount();
  });
});
