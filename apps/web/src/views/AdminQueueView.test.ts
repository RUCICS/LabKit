import { afterEach, describe, expect, it, vi } from 'vitest';
import { createApp, nextTick } from 'vue';
import AdminQueueView from './AdminQueueView.vue';

async function flush() {
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

async function mountQueue(url = '/admin/labs/sorting/queue') {
  const el = document.createElement('div');
  document.body.appendChild(el);
  window.history.pushState({}, '', url);
  const app = createApp(AdminQueueView);
  app.mount(el);
  await flush();
  return {
    unmount() {
      app.unmount();
      el.remove();
    }
  };
}

afterEach(() => {
  document.body.innerHTML = '';
  window.sessionStorage.clear();
  vi.restoreAllMocks();
});

describe('AdminQueueView actions', () => {
  it('triggers reevaluation with the stored admin token', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, _init?: RequestInit) => {
      if (String(input) === '/api/admin/labs/sorting/queue') {
        return jsonResponse({
          lab_id: 'sorting',
          jobs: [
            {
              id: 'job-1',
              submission_id: 'sub-1',
              user_id: 7,
              status: 'running',
              attempts: 1,
              available_at: '2026-03-31T10:00:00Z',
              worker_id: 'worker-1',
              last_error: '',
              started_at: '2026-03-31T10:01:00Z',
              finished_at: '',
              created_at: '2026-03-31T10:00:00Z',
              updated_at: '2026-03-31T10:02:00Z'
            },
            {
              id: 'job-2',
              submission_id: 'sub-2',
              user_id: 8,
              status: 'queued',
              attempts: 2,
              available_at: '2026-03-31T10:05:00Z',
              worker_id: '',
              last_error: 'Timeout from worker',
              started_at: '',
              finished_at: '',
              created_at: '2026-03-31T10:05:00Z',
              updated_at: '2026-03-31T10:05:00Z'
            }
          ]
        });
      }
      if (String(input) === '/api/admin/labs/sorting/reeval') {
        return jsonResponse({ lab_id: 'sorting', jobs_created: 2 }, 202);
      }
      throw new Error(`unexpected fetch ${String(input)}`);
    });
    vi.stubGlobal('fetch', fetchMock);
    window.sessionStorage.setItem('labkit_admin_token', 'secret');

    const view = await mountQueue();
    expect(document.body.textContent).not.toContain('队列状态、成绩导出与重评操作。');
    expect(document.body.textContent).toContain('queued');
    expect(document.body.textContent).toContain('running');
    expect(document.body.textContent).toContain('Timeout from worker');
    expect(document.body.textContent).toContain('2 jobs');
    expect(document.body.textContent).toContain('1 running');
    const button = Array.from(document.querySelectorAll('button')).find((candidate) =>
      candidate.textContent?.includes('Reevaluate')
    ) as HTMLButtonElement | undefined;

    expect(button).toBeDefined();

    button!.click();
    await flush();

    const [, init] = fetchMock.mock.calls.find(([input]) => String(input) === '/api/admin/labs/sorting/reeval') ?? [];
    expect(fetchMock).toHaveBeenCalledWith('/api/admin/labs/sorting/reeval', expect.any(Object));
    expect((init as RequestInit | undefined)?.method).toBe('POST');
    expect(new Headers((init as RequestInit | undefined)?.headers).get('Authorization')).toBe(
      'Bearer secret'
    );
    expect(document.body.textContent).toContain('Queued 2 re-evaluations');

    view.unmount();
  });

  it('exports grades with the stored admin token', async () => {
    const createObjectURL = vi.fn(() => 'blob:grades');
    const revokeObjectURL = vi.fn();
    vi.stubGlobal(
      'URL',
      class URLStub extends URL {
        static createObjectURL = createObjectURL;
        static revokeObjectURL = revokeObjectURL;
      }
    );

    const clickSpy = vi.fn();
    const originalCreateElement = document.createElement.bind(document);
    vi.spyOn(document, 'createElement').mockImplementation(((tagName: string) => {
      const el = originalCreateElement(tagName) as HTMLElement;
      if (tagName === 'a') {
        Object.defineProperty(el, 'click', { value: clickSpy });
      }
      return el;
    }) as typeof document.createElement);

    const fetchMock = vi.fn(async (input: RequestInfo | URL, _init?: RequestInit) => {
      if (String(input) === '/api/admin/labs/sorting/queue') {
        return jsonResponse({ lab_id: 'sorting', jobs: [] });
      }
      if (String(input) === '/api/admin/labs/sorting/grades') {
        return {
          ok: true,
          status: 200,
          headers: new Headers({
            'Content-Disposition': 'attachment; filename="sorting-grades.csv"'
          }),
          blob: async () => new Blob(['id,rank\n2026001,1\n'], { type: 'text/csv' }),
          text: async () => 'id,rank\n2026001,1\n'
        } as Response;
      }
      throw new Error(`unexpected fetch ${String(input)}`);
    });
    vi.stubGlobal('fetch', fetchMock);
    window.sessionStorage.setItem('labkit_admin_token', 'secret');

    const view = await mountQueue();
    expect(document.body.textContent).not.toContain('队列状态、成绩导出与重评操作。');
    const button = Array.from(document.querySelectorAll('button')).find((candidate) =>
      candidate.textContent?.includes('Export grades')
    ) as HTMLButtonElement | undefined;

    expect(button).toBeDefined();

    button!.click();
    await flush();

    const [, init] = fetchMock.mock.calls.find(([input]) => String(input) === '/api/admin/labs/sorting/grades') ?? [];
    expect(fetchMock).toHaveBeenCalledWith('/api/admin/labs/sorting/grades', expect.any(Object));
    expect(new Headers((init as RequestInit | undefined)?.headers).get('Authorization')).toBe(
      'Bearer secret'
    );
    expect(createObjectURL).toHaveBeenCalledTimes(1);
    expect(clickSpy).toHaveBeenCalledTimes(1);
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:grades');

    view.unmount();
  });
});
