import { afterEach, describe, expect, it, vi } from 'vitest';
import { createApp, nextTick } from 'vue';
import GradesDrawer from './GradesDrawer.vue';

async function flush() {
  await Promise.resolve();
  await Promise.resolve();
  await nextTick();
}

// File reads resolve on a macrotask in jsdom; settle drains a few cycles so the
// FileReader callback, the awaiting handler, and the re-render all complete.
async function settle() {
  for (let i = 0; i < 3; i += 1) {
    await new Promise((resolve) => setTimeout(resolve, 0));
    await flush();
  }
}

function jsonResponse(payload: unknown, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => payload,
    text: async () => JSON.stringify(payload)
  } as Response;
}

function statusPayload(over: Partial<{ total: number; published: number; unpublished: number }> = {}) {
  return {
    lab_id: 'colab-2026-p2',
    total: over.total ?? 0,
    published: over.published ?? 0,
    unpublished: over.unpublished ?? 0,
    last_updated_at: '2026-06-20T10:00:00Z'
  };
}

async function mountDrawer() {
  const el = document.createElement('div');
  document.body.appendChild(el);
  const app = createApp(GradesDrawer, { labId: 'colab-2026-p2', labName: 'CoLab', open: true });
  app.mount(el);
  await flush();
  return {
    unmount() {
      app.unmount();
      el.remove();
    }
  };
}

function fileInput(): HTMLInputElement {
  return document.body.querySelector('[data-testid="grades-file"]') as HTMLInputElement;
}

function selectFile(content: string, name = 'grades.csv') {
  const input = fileInput();
  const file = new File([content], name, { type: 'text/csv' });
  Object.defineProperty(input, 'files', { value: [file], configurable: true });
  input.dispatchEvent(new Event('change'));
}

afterEach(() => {
  document.body.innerHTML = '';
  vi.restoreAllMocks();
});

describe('GradesDrawer', () => {
  it('loads and renders the current grade status', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse(statusPayload({ total: 30, published: 12, unpublished: 18 })))
    );

    const view = await mountDrawer();

    expect(document.querySelector('[data-testid="grades-total"]')?.textContent).toBe('30');
    expect(document.querySelector('[data-testid="grades-published"]')?.textContent).toBe('12');
    expect(document.querySelector('[data-testid="grades-pending"]')?.textContent).toBe('18');

    view.unmount();
  });

  it('previews a selected CSV and enables import for valid required columns', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(statusPayload())));
    const view = await mountDrawer();

    selectFile('student_id,total,track\n2026001,90,throughput\n2026002,80,latency\n');
    await settle();

    const preview = document.querySelector('[data-testid="grades-preview"]');
    expect(preview).not.toBeNull();
    expect(preview?.textContent).toContain('共 2 行待导入');
    const importBtn = document.querySelector('[data-testid="grades-import"]') as HTMLButtonElement;
    expect(importBtn.disabled).toBe(false);

    view.unmount();
  });

  it('blocks import when a required column is missing', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(statusPayload())));
    const view = await mountDrawer();

    selectFile('student_id,track\n2026001,throughput\n');
    await settle();

    expect(document.querySelector('[data-testid="grades-preview"]')?.textContent).toContain('缺少必需列');
    const importBtn = document.querySelector('[data-testid="grades-import"]') as HTMLButtonElement;
    expect(importBtn.disabled).toBe(true);

    view.unmount();
  });

  it('uploads the CSV as multipart on import and refreshes status', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith('/grades/import')) {
        // The CSV must be sent as multipart form data.
        expect(init?.method).toBe('POST');
        expect(init?.body).toBeInstanceOf(FormData);
        return jsonResponse({ lab_id: 'colab-2026-p2', imported: 2 });
      }
      // status calls (initial + post-import refresh)
      return jsonResponse(statusPayload({ total: 2, published: 0, unpublished: 2 }));
    });
    vi.stubGlobal('fetch', fetchMock);

    const view = await mountDrawer();
    selectFile('student_id,total\n2026001,90\n2026002,80\n');
    await settle();

    (document.querySelector('[data-testid="grades-import"]') as HTMLButtonElement).click();
    await settle();

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/admin/labs/colab-2026-p2/grades/import',
      expect.objectContaining({ method: 'POST' })
    );
    expect(document.querySelector('[data-testid="grades-import-result"]')?.textContent).toContain('已导入 2 条');

    view.unmount();
  });

  it('publishes pending grades after confirmation', async () => {
    let published = false;
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith('/grades/publish')) {
        published = true;
        return jsonResponse({ lab_id: 'colab-2026-p2', published: 18 });
      }
      return jsonResponse(
        published
          ? statusPayload({ total: 18, published: 18, unpublished: 0 })
          : statusPayload({ total: 18, published: 0, unpublished: 18 })
      );
    });
    vi.stubGlobal('fetch', fetchMock);

    const view = await mountDrawer();

    (document.querySelector('[data-testid="grades-publish"]') as HTMLButtonElement).click();
    await flush();
    (document.querySelector('[data-testid="grades-publish-confirm"]') as HTMLButtonElement).click();
    await flush();
    await flush();

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/admin/labs/colab-2026-p2/grades/publish',
      expect.objectContaining({ method: 'POST' })
    );
    expect(document.querySelector('[data-testid="grades-published"]')?.textContent).toBe('18');

    view.unmount();
  });
});
