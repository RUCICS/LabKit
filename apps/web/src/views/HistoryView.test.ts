import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp, nextTick } from 'vue';
import HistoryView from './HistoryView.vue';

async function flushPromises() {
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

async function mountHistory(labId = 'sorting') {
  const el = document.createElement('div');
  document.body.appendChild(el);
  const app = createApp(HistoryView, { labId });
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
  window.history.pushState({}, '', '/');
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('HistoryView', () => {
  it('renders submission history, quota summary, and expanded detail', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith('/api/labs/sorting/history')) {
        return jsonResponse({
          submissions: [
            {
              id: '11111111-1111-7111-8111-111111111111',
              status: 'done',
              verdict: 'scored',
              created_at: '2026-03-31T10:00:00Z',
              finished_at: '2026-03-31T10:02:00Z'
            },
            {
              id: '22222222-2222-7222-8222-222222222222',
              status: 'queued',
              created_at: '2026-03-31T11:00:00Z'
            }
          ],
          quota: {
            daily: 3,
            used: 1,
            left: 2,
            reset_hint: '00:00 Asia/Shanghai'
          }
        });
      }
      if (url.endsWith('/api/labs/sorting/submissions/11111111-1111-7111-8111-111111111111')) {
        return jsonResponse({
          id: '11111111-1111-7111-8111-111111111111',
          status: 'done',
          verdict: 'scored',
          created_at: '2026-03-31T10:00:00Z',
          finished_at: '2026-03-31T10:02:00Z',
          scores: [
            { metric_id: 'throughput', value: 1.82 },
            { metric_id: 'latency', value: 1.21 }
          ],
          detail: {
            format: 'markdown',
            content: '### Public Workloads\n\nsteady-state passed'
          },
          quota: {
            daily: 3,
            used: 1,
            left: 2,
            reset_hint: '00:00 Asia/Shanghai'
          }
        });
      }
      if (url.endsWith('/api/labs/sorting')) {
        return jsonResponse({
          id: 'sorting',
          name: 'CoLab 调度器竞赛',
          manifest: {
            board: {
              pick: true
            },
            metrics: [
              { id: 'throughput', name: 'Throughput', sort: 'desc', unit: 'x' },
              { id: 'latency', name: 'Latency', sort: 'desc', unit: 'x' }
            ],
            schedule: {
              open: '2026-03-01T00:00:00Z',
              close: '2026-06-01T00:00:00Z'
            }
          }
        });
      }
      throw new Error(`unexpected fetch ${url}`);
    });
    vi.stubGlobal('fetch', fetchMock);

    const view = await mountHistory();

    expect(document.body.textContent).toContain('My submissions');
    expect(document.body.textContent).toContain('2 left today');
    expect(document.body.textContent).toContain('scored');
    expect(document.body.textContent).toContain('queued');

    const button = Array.from(document.querySelectorAll('button')).find((item) =>
      item.textContent?.includes('Expand detail')
    ) as HTMLButtonElement | undefined;
    expect(button).toBeDefined();

    button?.click();
    await flushPromises();

    expect(document.body.textContent).toContain('steady-state passed');
    expect(document.body.textContent).toContain('1.82');
    expect(document.body.textContent).toContain('1.21');

    view.unmount();
  });
});
