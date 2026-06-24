import { afterEach, describe, expect, it, vi } from 'vitest';
import { createApp, defineComponent, h, nextTick } from 'vue';
import { createMemoryHistory, createRouter } from 'vue-router';
import GradeView from './GradeView.vue';

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

async function mountGradeView(url = '/grade') {
  const el = document.createElement('div');
  document.body.appendChild(el);
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/grade', component: GradeView },
      { path: '/labs/:labID/grade', component: GradeView, props: (route) => ({ labId: String(route.params.labID) }) },
      { path: '/login', component: { render: () => null } }
    ]
  });
  const app = createApp(defineComponent({ render: () => h('div', [h(GradeView)]) }));
  app.use(router);
  await router.push(url);
  await router.isReady();
  app.mount(el);
  await flush();
  return {
    router,
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

describe('GradeView', () => {
  it('renders the total and breakdown for a published grade', async () => {
    const fetchMock = vi.fn(async () =>
      jsonResponse({
        lab_id: 'colab-2026-p2',
        student_id: '2026001',
        total: 86.5,
        track: 'throughput',
        ratio: 1.2,
        perf_score: 85,
        percentile: 0.91,
        board_score: 14,
        remark: '复核无误',
        updated_at: '2026-06-20T10:00:00Z'
      })
    );
    vi.stubGlobal('fetch', fetchMock);

    const view = await mountGradeView('/grade');

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/labs/colab-2026-p2/grade',
      expect.objectContaining({ credentials: 'include' })
    );
    expect(document.body.textContent).toContain('86.50');
    expect(document.body.textContent).toContain('throughput');
    expect(document.body.textContent).toContain('总评 = 0.85 × 性能分 + 0.15 × 打榜分');
    expect(document.body.textContent).toContain('复核无误');

    view.unmount();
  });

  it('uses the labID route param when present', async () => {
    const fetchMock = vi.fn(async () =>
      jsonResponse({ lab_id: 'sorting', student_id: '2026001', total: 90, updated_at: '2026-06-20T10:00:00Z' })
    );
    vi.stubGlobal('fetch', fetchMock);

    const view = await mountGradeView('/labs/sorting/grade');

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/labs/sorting/grade',
      expect.objectContaining({ credentials: 'include' })
    );

    view.unmount();
  });

  it('shows the not-published state on 404', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse({ error: { message: '成绩尚未发布' } }, 404)));

    const view = await mountGradeView('/grade');

    expect(document.body.textContent).toContain('成绩尚未发布');

    view.unmount();
  });

  it('shows a login prompt on 401', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse({ error: { message: 'unauthorized' } }, 401)));

    const view = await mountGradeView('/grade');

    expect(document.body.textContent).toContain('需要登录');
    expect(document.body.textContent).toContain('前往登录');

    view.unmount();
  });
});
