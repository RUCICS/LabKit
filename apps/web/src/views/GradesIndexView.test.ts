import { afterEach, describe, expect, it, vi } from 'vitest';
import { createApp, defineComponent, h, nextTick } from 'vue';
import { createMemoryHistory, createRouter } from 'vue-router';
import GradesIndexView from './GradesIndexView.vue';

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

async function mountIndex() {
  const el = document.createElement('div');
  document.body.appendChild(el);
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/grade', component: GradesIndexView },
      { path: '/labs/:labID/grade', component: { render: () => null } },
      { path: '/login', component: { render: () => null } }
    ]
  });
  const app = createApp(defineComponent({ render: () => h('div', [h(GradesIndexView)]) }));
  app.use(router);
  await router.push('/grade');
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

afterEach(() => {
  document.body.innerHTML = '';
  vi.restoreAllMocks();
});

describe('GradesIndexView', () => {
  it('lists every published grade across labs without assuming a default lab', async () => {
    const fetchMock = vi.fn(async () =>
      jsonResponse({
        grades: [
          { lab_id: 'colab-2026-p2', student_id: '2026001', total: '86.5', items: [{ label: '赛道', value: 'throughput' }], updated_at: '2026-06-20T10:00:00Z' },
          { lab_id: 'sorting', student_id: '2026001', total: '90', items: [], updated_at: '2026-06-19T10:00:00Z' }
        ]
      })
    );
    vi.stubGlobal('fetch', fetchMock);

    const view = await mountIndex();

    expect(fetchMock).toHaveBeenCalledWith('/api/grades', expect.objectContaining({ credentials: 'include' }));
    const labs = document.querySelectorAll('[data-testid="grades-index-lab"]');
    expect(labs.length).toBe(2);
    expect(document.body.textContent).toContain('colab-2026-p2');
    expect(document.body.textContent).toContain('sorting');
    expect(document.body.textContent).toContain('86.5');
    expect(document.body.textContent).toContain('throughput');

    view.unmount();
  });

  it('shows an empty state when the student has no published grades', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse({ grades: [] })));

    const view = await mountIndex();

    expect(document.querySelector('[data-testid="grades-empty"]')).not.toBeNull();

    view.unmount();
  });

  it('shows a login prompt on 401', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse({ error: { message: 'unauthorized' } }, 401)));

    const view = await mountIndex();

    expect(document.body.textContent).toContain('请先登录');

    view.unmount();
  });
});
