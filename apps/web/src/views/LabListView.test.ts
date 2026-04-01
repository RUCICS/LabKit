import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp, nextTick } from 'vue';
import { createAppRouter, createMemoryHistory } from '../router';
import LabListView from './LabListView.vue';

async function flushPromises() {
  await Promise.resolve();
  await Promise.resolve();
  await nextTick();
}

async function mountLabList() {
  const el = document.createElement('div');
  document.body.appendChild(el);
  const router = createAppRouter(createMemoryHistory());
  await router.push('/');
  await router.isReady();
  const app = createApp(LabListView).use(router);
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

describe('LabListView', () => {
  it('renders public labs from the wrapped API payload', async () => {
    const fetchSpy = vi.fn(async (input: RequestInfo | URL) => {
      expect(String(input)).toBe('/api/labs');
      return {
        ok: true,
        status: 200,
        json: async () => ({
          labs: [
            {
              id: 'sorting',
              name: 'Sorting Lab',
              manifest: {
                submit: { files: ['main.c'] },
                metrics: [
                  { id: 'runtime_ms', name: 'Runtime', sort: 'desc' },
                  { id: 'latency_ms', name: 'Latency', sort: 'asc' }
                ],
                schedule: {
                  open: '2026-03-01T00:00:00Z',
                  close: '2026-06-01T00:00:00Z'
                }
              }
            },
            { id: 'graph', name: 'Graph Lab' }
          ]
        }),
        text: async () => JSON.stringify({
          labs: [
            {
              id: 'sorting',
              name: 'Sorting Lab',
              manifest: {
                submit: { files: ['main.c'] },
                metrics: [
                  { id: 'runtime_ms', name: 'Runtime', sort: 'desc' },
                  { id: 'latency_ms', name: 'Latency', sort: 'asc' }
                ],
                schedule: {
                  open: '2026-03-01T00:00:00Z',
                  close: '2026-06-01T00:00:00Z'
                }
              }
            },
            { id: 'graph', name: 'Graph Lab' }
          ]
        })
      } as Response;
    });
    vi.stubGlobal('fetch', fetchSpy);

    const view = await mountLabList();

    expect(fetchSpy).toHaveBeenCalledTimes(1);
    expect(document.body.textContent).toContain('Sorting Lab');
    expect(document.body.textContent).toContain('Graph Lab');
    expect(document.body.textContent).toContain('OPEN');
    expect(document.body.textContent).toContain('Runtime');
    expect(document.body.textContent).toContain('Latency');
    expect(document.body.textContent).toContain('closes');
    expect(document.body.textContent).not.toContain('Open a board, track standings, and jump back in quickly.');
    const links = Array.from(document.querySelectorAll('a'));
    expect(links.map((link) => link.getAttribute('href'))).toContain('/labs/sorting/board');
    expect(links.map((link) => link.getAttribute('href'))).toContain('/labs/graph/board');

    view.unmount();
  });
});
