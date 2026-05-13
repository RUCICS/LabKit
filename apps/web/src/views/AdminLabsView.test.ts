import { afterEach, describe, expect, it, vi } from 'vitest';
import { createApp, nextTick } from 'vue';
import { createMemoryHistory, createRouter } from 'vue-router';
import AdminLabsView from './AdminLabsView.vue';
import { adminTokenStorageKey } from '../lib/admin';

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

const labsPayload = {
  labs: [
    {
      id: 'sorting',
      name: 'Sorting Lab',
      manifest: {
        Metrics: [{ ID: 'runtime_ms', Name: 'Runtime', Sort: 'asc', Unit: 'ms' }],
        Schedule: { Open: '2026-03-10T00:00:00Z', Close: '2026-04-30T00:00:00Z', Visible: '2026-03-10T00:00:00Z' }
      }
    }
  ]
};

afterEach(() => {
  document.body.innerHTML = '';
  vi.restoreAllMocks();
  window.sessionStorage.clear();
  window.localStorage.clear();
});

function mountLabsView() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/admin/login', name: 'admin-login', component: { template: '<div>login</div>' } },
      { path: '/admin/labs', name: 'admin-labs', component: AdminLabsView },
      { path: '/admin/labs/:labID/queue', name: 'admin-queue', component: { template: '<div>queue</div>' } }
    ]
  });
  return router;
}

describe('AdminLabsView', () => {
  it('shows lab list and Edit / Queue buttons per lab', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(labsPayload)));
    window.sessionStorage.setItem(adminTokenStorageKey, 'secret');

    const router = mountLabsView();
    await router.push('/admin/labs');
    await router.isReady();

    const el = document.createElement('div');
    document.body.appendChild(el);
    const app = createApp(AdminLabsView);
    app.use(router);
    app.mount(el);
    await flush();

    expect(document.body.textContent).toContain('Sorting Lab');
    const buttons = Array.from(document.querySelectorAll('button'));
    expect(buttons.some((b) => b.textContent?.includes('Edit'))).toBe(true);
    expect(document.body.innerHTML).toContain('/admin/labs/sorting/queue');

    app.unmount();
    el.remove();
  });

  it('opens new-lab drawer when + New lab is clicked', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(labsPayload)));
    window.sessionStorage.setItem(adminTokenStorageKey, 'secret');

    const router = mountLabsView();
    await router.push('/admin/labs');
    await router.isReady();

    const el = document.createElement('div');
    document.body.appendChild(el);
    const app = createApp(AdminLabsView);
    app.use(router);
    app.mount(el);
    await flush();

    const newLabBtn = Array.from(document.querySelectorAll('button')).find((b) =>
      b.textContent?.includes('New lab')
    );
    expect(newLabBtn).toBeDefined();
    newLabBtn!.click();
    await flush();

    const drawer = document.querySelector('[data-testid="lab-edit-drawer"]');
    expect(drawer).not.toBeNull();
    // creation mode: shows "Create lab" save button, not "Save changes"
    expect(drawer!.textContent).toContain('New lab');
    const createBtn = Array.from(document.querySelectorAll('button')).find((b) =>
      b.textContent?.trim() === 'Create lab'
    );
    expect(createBtn).toBeDefined();

    app.unmount();
    el.remove();
  });

  it('opens the edit drawer when Edit is clicked', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (url: string) => {
        if (String(url) === '/api/labs') return jsonResponse(labsPayload);
        if (String(url).startsWith('/api/admin/labs/sorting')) {
          return jsonResponse({
            id: 'sorting',
            name: 'Sorting Lab',
            manifest: {
              Lab: { ID: 'sorting', Name: 'Sorting Lab', Tags: {} },
              Submit: { Files: ['main.c'], MaxSize: '1MB' },
              Eval: { Image: 'grader:v1', Timeout: 300 },
              Quota: { Daily: 10, Free: [] },
              Metrics: [{ ID: 'score', Name: 'Score', Sort: 'desc', Unit: 'pts' }],
              Board: { RankBy: 'score', Pick: false },
              Schedule: { Visible: '0001-01-01T00:00:00Z', Open: '2026-03-10T00:00:00Z', Close: '0001-01-01T00:00:00Z' }
            }
          });
        }
        throw new Error(`unexpected fetch ${String(url)}`);
      })
    );
    window.sessionStorage.setItem(adminTokenStorageKey, 'secret');

    const router = mountLabsView();
    await router.push('/admin/labs');
    await router.isReady();

    const el = document.createElement('div');
    document.body.appendChild(el);
    const app = createApp(AdminLabsView);
    app.use(router);
    app.mount(el);
    await flush();

    const editBtn = Array.from(document.querySelectorAll('button')).find((b) =>
      b.textContent?.trim() === 'Edit'
    );
    expect(editBtn).toBeDefined();
    editBtn!.click();
    await flush();
    await flush();

    expect(document.querySelector('[data-testid="lab-edit-drawer"]')).not.toBeNull();

    app.unmount();
    el.remove();
  });
});
