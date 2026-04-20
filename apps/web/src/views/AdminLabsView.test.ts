import { afterEach, describe, expect, it, vi } from 'vitest';
import { createApp, nextTick } from 'vue';
import { createMemoryHistory, createRouter } from 'vue-router';
import AdminLabsView from './AdminLabsView.vue';

const validManifest = `
[lab]
id = "sorting"
name = "Sorting Lab"

[submit]
files = ["main.py"]
max_size = "1MB"

[eval]
image = "ghcr.io/labkit/sorting:1"
timeout = 60

[quota]
daily = 3
free = ["build_failed"]

[[metric]]
id = "runtime_ms"
name = "Runtime"
sort = "asc"
unit = "ms"

[board]
rank_by = "runtime_ms"
pick = false

[schedule]
visible = 2026-03-01T00:00:00Z
open = 2026-03-10T00:00:00Z
close = 2026-04-30T00:00:00Z
`;

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

afterEach(() => {
  document.body.innerHTML = '';
  vi.restoreAllMocks();
});

describe('AdminLabsView', () => {
  it('moves the admin token into browser storage and keeps it out of the queue link', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () =>
        jsonResponse({
          labs: [{
            id: 'sorting',
            name: 'Sorting Lab',
            manifest: {
              metrics: [
                { id: 'runtime_ms', name: 'Runtime', sort: 'asc' },
                { id: 'latency_ms', name: 'Latency', sort: 'asc' }
              ],
              schedule: {
                open: '2026-03-10T00:00:00Z',
                close: '2026-04-30T00:00:00Z'
              }
            }
          }]
        })
      )
    );
    window.sessionStorage.clear();

    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/admin/labs', name: 'admin-labs', component: AdminLabsView },
        { path: '/admin/labs/:labID/queue', name: 'admin-queue', component: AdminLabsView }
      ]
    });
    await router.push('/admin/labs?token=secret');
    await router.isReady();

    const el = document.createElement('div');
    document.body.appendChild(el);
    const app = createApp(AdminLabsView);
    app.use(router);
    app.mount(el);
    await flush();

    expect(document.body.textContent).not.toContain('注册、更新并维护当前实验。');
    const link = document.querySelector('.admin-labs-view__row-actions a');
    expect(link?.getAttribute('href')).toBe('/admin/labs/sorting/queue');
    expect(window.location.search).toBe('');
    expect(window.sessionStorage.getItem('labkit_admin_token')).toBe('secret');
    expect(document.body.textContent).toContain('OPEN');
    expect(document.body.textContent).toContain('2 metrics');

    app.unmount();
    el.remove();
  });

  it('registers a lab from the manifest editor using the stored admin token', async () => {
    const fetchMock = vi
      .fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        if (String(input) === '/api/labs') {
          return jsonResponse({ labs: [] });
        }
        if (String(input) === '/api/admin/labs') {
          return jsonResponse({ id: 'sorting', name: 'Sorting Lab' }, 201);
        }
        throw new Error(`unexpected fetch ${String(input)}`);
      });
    vi.stubGlobal('fetch', fetchMock);
    window.sessionStorage.setItem('labkit_admin_token', 'secret');

    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/admin/labs', name: 'admin-labs', component: AdminLabsView },
        { path: '/admin/labs/:labID/queue', name: 'admin-queue', component: AdminLabsView }
      ]
    });
    await router.push('/admin/labs');
    await router.isReady();

    const el = document.createElement('div');
    document.body.appendChild(el);
    const app = createApp(AdminLabsView);
    app.use(router);
    app.mount(el);
    await flush();

    expect(document.body.textContent).not.toContain('注册、更新并维护当前实验。');
    const textarea = document.querySelector('textarea') as HTMLTextAreaElement | null;
    const submit = Array.from(document.querySelectorAll('button')).find((button) =>
      button.textContent?.includes('Register lab')
    ) as HTMLButtonElement | undefined;

    expect(textarea).not.toBeNull();
    expect(submit).toBeDefined();

    textarea!.value = validManifest;
    textarea!.dispatchEvent(new Event('input', { bubbles: true }));
    submit!.click();
    await flush();

    const [, init] = fetchMock.mock.calls.find(([input]) => String(input) === '/api/admin/labs') ?? [];
    expect(fetchMock).toHaveBeenCalledWith('/api/admin/labs', expect.any(Object));
    expect((init as RequestInit | undefined)?.method).toBe('POST');
    expect(new Headers((init as RequestInit | undefined)?.headers).get('Authorization')).toBe(
      'Bearer secret'
    );
    expect((init as RequestInit | undefined)?.body).toBe(validManifest);
    expect(document.body.textContent).toContain('Lab registered');

    app.unmount();
    el.remove();
  });

  it('updates an existing lab manifest from the admin editor', async () => {
    const fetchMock = vi
      .fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        if (String(input) === '/api/labs') {
          return jsonResponse({ labs: [{ id: 'sorting', name: 'Sorting Lab' }] });
        }
        if (String(input) === '/api/admin/labs/sorting') {
          return jsonResponse({ id: 'sorting', name: 'Sorting Lab' });
        }
        throw new Error(`unexpected fetch ${String(input)}`);
      });
    vi.stubGlobal('fetch', fetchMock);
    window.sessionStorage.setItem('labkit_admin_token', 'secret');

    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/admin/labs', name: 'admin-labs', component: AdminLabsView },
        { path: '/admin/labs/:labID/queue', name: 'admin-queue', component: AdminLabsView }
      ]
    });
    await router.push('/admin/labs');
    await router.isReady();

    const el = document.createElement('div');
    document.body.appendChild(el);
    const app = createApp(AdminLabsView);
    app.use(router);
    app.mount(el);
    await flush();

    expect(document.body.textContent).not.toContain('注册、更新并维护当前实验。');
    const action = document.querySelector('select') as HTMLSelectElement | null;
    const targetLab = document.querySelector('input[name="lab-id"]') as HTMLInputElement | null;
    const textarea = document.querySelector('textarea') as HTMLTextAreaElement | null;

    expect(action).not.toBeNull();
    expect(targetLab).not.toBeNull();
    expect(textarea).not.toBeNull();

    action!.value = 'update';
    action!.dispatchEvent(new Event('change', { bubbles: true }));
    await flush();

    const submit = Array.from(document.querySelectorAll('button')).find((button) =>
      button.textContent?.includes('Update lab')
    ) as HTMLButtonElement | undefined;
    expect(submit).toBeDefined();

    targetLab!.value = 'sorting';
    targetLab!.dispatchEvent(new Event('input', { bubbles: true }));
    textarea!.value = validManifest;
    textarea!.dispatchEvent(new Event('input', { bubbles: true }));
    submit!.click();
    await flush();

    const [, init] = fetchMock.mock.calls.find(([input]) => String(input) === '/api/admin/labs/sorting') ?? [];
    expect(fetchMock).toHaveBeenCalledWith('/api/admin/labs/sorting', expect.any(Object));
    expect((init as RequestInit | undefined)?.method).toBe('PUT');
    expect(new Headers((init as RequestInit | undefined)?.headers).get('Authorization')).toBe(
      'Bearer secret'
    );
    expect((init as RequestInit | undefined)?.body).toBe(validManifest);
    expect(document.body.textContent).toContain('Lab updated');

    app.unmount();
    el.remove();
  });
});
