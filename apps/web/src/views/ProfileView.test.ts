import { afterEach, describe, expect, it, vi } from 'vitest';
import { createApp, defineComponent, h, nextTick, ref } from 'vue';
import { createMemoryHistory, createRouter } from 'vue-router';
import AuthConfirmView from './AuthConfirmView.vue';
import AdminQueueView from './AdminQueueView.vue';
import ProfileView from './ProfileView.vue';

async function flush() {
  await Promise.resolve();
  await Promise.resolve();
  await nextTick();
}

async function mountView(component: any, url = '/') {
  const el = document.createElement('div');
  document.body.appendChild(el);
  window.history.pushState({}, '', url);
  const app = createApp(component);
  app.mount(el);
  await flush();
  return {
    unmount() {
      app.unmount();
      el.remove();
    }
  };
}

async function mountProfileView(url = '/profile') {
  const el = document.createElement('div');
  document.body.appendChild(el);
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/profile', component: ProfileView },
      { path: '/labs/:labID/board', component: { render: () => null } },
      { path: '/labs/:labID/history', component: { render: () => null } }
    ]
  });
  const app = createApp(defineComponent({ render: () => h('div', [h(ProfileView)]) }));
  app.use(router);
  await router.push(url);
  await router.isReady();
  app.mount(el);
  await flush();
  return {
    unmount() {
      app.unmount();
      el.remove();
    }
  };
}

function deferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
  });
  return { promise, resolve };
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

describe('AuthConfirmView', () => {
  it('renders the OAuth confirmation success state', async () => {
    vi.useFakeTimers();
    const close = vi.fn();
    Object.defineProperty(window, 'close', {
      value: close,
      configurable: true
    });

    const view = await mountView(
      AuthConfirmView,
      '/auth/confirm?student_id=2026001&user_key_id=11'
    );

    expect(document.body.textContent).toContain('设备已授权');
    expect(document.body.textContent).toContain('2026001');
    expect(document.body.textContent).toContain('你现在可以回到终端使用 LabKit');
    expect(document.body.textContent).toContain('页面将在 5 秒后关闭');
    expect(document.querySelector('svg')).not.toBeNull();

    vi.advanceTimersByTime(5000);
    await flush();

    expect(close).toHaveBeenCalledTimes(1);

    view.unmount();
    vi.useRealTimers();
  });

  it('renders the generic browser session recovery state without student details', async () => {
    vi.useFakeTimers();
    const close = vi.fn();
    Object.defineProperty(window, 'close', {
      value: close,
      configurable: true
    });

    const view = await mountView(AuthConfirmView, '/auth/confirm?mode=web-session');

    expect(document.body.textContent).toContain('Browser session restored');
    expect(document.body.textContent).toContain('页面将在 5 秒后关闭');
    expect(document.body.textContent).not.toContain('Student ID');
    expect(document.body.textContent).not.toContain('Key 11');

    vi.advanceTimersByTime(5000);
    await flush();

    expect(close).toHaveBeenCalledTimes(1);

    view.unmount();
    vi.useRealTimers();
  });
});

describe('ProfileView', () => {
  it('loads /api/profile and renders identity, devices, and activity', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url === '/api/profile' && (!init || init.method === undefined || init.method === 'GET')) {
        return jsonResponse({
          user_id: 7,
          student_id: 's123',
          nickname: 'Aki',
          recent_activity: [
            {
              id: '11111111-1111-7111-8111-111111111111',
              lab_id: 'sorting',
              status: 'done',
              created_at: '2026-03-31T12:00:00Z'
            }
          ]
        });
      }
      if (url === '/api/keys') {
        return jsonResponse({
          keys: [
            {
              id: 11,
              public_key: 'ssh-ed25519 AAAA',
              device_name: 'Laptop',
              created_at: '2026-03-31T10:00:00Z'
            },
            {
              id: 12,
              public_key: 'ssh-ed25519 BBBB',
              device_name: 'Phone',
              created_at: '2026-03-31T11:00:00Z'
            }
          ]
        });
      }
      return jsonResponse({ error: { message: 'not found' } }, 404);
    });
    vi.stubGlobal('fetch', fetchMock);

    const view = await mountProfileView('/profile');

    const h1s = Array.from(document.querySelectorAll('h1')).map((node) => node.textContent?.trim());
    expect(h1s).toEqual(['Profile']);

    const h2s = Array.from(document.querySelectorAll('h2')).map((node) => node.textContent?.trim());
    expect(h2s).toEqual(expect.arrayContaining(['Identity', 'Devices', 'Activity']));

    expect(document.body.textContent).toContain('Laptop');
    expect(document.body.textContent).toContain('Phone');
    expect(document.body.textContent).toContain('sorting');
    expect(document.body.textContent).toContain('Copy key');
    expect(document.body.textContent).not.toContain('已绑定设备与密钥指纹。');
    expect(fetchMock).toHaveBeenCalledWith('/api/profile', expect.objectContaining({ credentials: 'include' }));
    expect(fetchMock).toHaveBeenCalledWith('/api/keys', expect.objectContaining({ credentials: 'include' }));

    view.unmount();
  });

  it('renders an empty state when no keys exist', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url === '/api/profile') {
        return jsonResponse({ user_id: 7, student_id: 's123', nickname: 'Aki', recent_activity: [] });
      }
      if (url === '/api/keys') {
        return jsonResponse({ keys: [] });
      }
      return jsonResponse({ error: { message: 'not found' } }, 404);
    });
    vi.stubGlobal('fetch', fetchMock);

    const view = await mountProfileView('/profile');

    expect(document.body.textContent).toContain('No keys are registered yet.');

    view.unmount();
  });
});

describe('AdminQueueView', () => {
  it('renders queue job statuses', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () =>
        jsonResponse({
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
              attempts: 0,
              available_at: '2026-03-31T10:05:00Z',
              worker_id: '',
              last_error: '',
              started_at: '',
              finished_at: '',
              created_at: '2026-03-31T10:05:00Z',
              updated_at: '2026-03-31T10:05:00Z'
            }
          ]
        })
      )
    );

    const view = await mountView(AdminQueueView, '/admin/labs/sorting/queue');

    expect(document.body.textContent).toContain('Queue status');
    expect(document.body.textContent).toContain('running');
    expect(document.body.textContent).toContain('queued');

    view.unmount();
  });

  it('reads the admin token from browser storage for queue fetches', async () => {
    window.sessionStorage.setItem('labkit_admin_token', 'secret');
    const fetchMock = vi.fn(async () =>
      jsonResponse({
        lab_id: 'sorting',
        jobs: []
      })
    );
    vi.stubGlobal('fetch', fetchMock);

    const view = await mountView(AdminQueueView, '/admin/labs/sorting/queue');

    const [, init] = fetchMock.mock.calls[0] ?? [];
    expect(fetchMock).toHaveBeenCalledWith('/api/admin/labs/sorting/queue', expect.any(Object));
    expect(new Headers((init as RequestInit | undefined)?.headers).get('Authorization')).toBe(
      'Bearer secret'
    );

    view.unmount();
  });

  it('keeps the newest queue result when the lab changes quickly', async () => {
    const first = deferred<Response>();
    const second = deferred<Response>();
    const fetchMock = vi.fn()
      .mockImplementationOnce(async () => first.promise)
      .mockImplementationOnce(async () => second.promise);
    vi.stubGlobal('fetch', fetchMock);

    const labId = ref('sorting');
    const Root = defineComponent({
      setup() {
        return () => h(AdminQueueView, { labId: labId.value });
      }
    });

    const el = document.createElement('div');
    document.body.appendChild(el);
    const app = createApp(Root);
    app.mount(el);
    await flush();

    labId.value = 'systems';
    await flush();

    second.resolve(
      jsonResponse({
        lab_id: 'systems',
        jobs: [{ id: 'latest-job', status: 'queued', updated_at: '2026-03-31T10:10:00Z' }]
      })
    );
    await flush();

    first.resolve(
      jsonResponse({
        lab_id: 'sorting',
        jobs: [{ id: 'stale-job', status: 'running', updated_at: '2026-03-31T10:09:00Z' }]
      })
    );
    await flush();

    expect(document.body.textContent).toContain('systems');
    expect(document.body.textContent).toContain('latest-job');
    expect(document.body.textContent).not.toContain('stale-job');

    app.unmount();
    el.remove();
  });
});
