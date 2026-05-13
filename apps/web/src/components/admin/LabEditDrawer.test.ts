import { afterEach, describe, expect, it, vi } from 'vitest';
import { createApp, nextTick } from 'vue';
import { createMemoryHistory, createRouter } from 'vue-router';
import LabEditDrawer from './LabEditDrawer.vue';
import { adminTokenStorageKey } from '../../lib/admin';

async function flush() {
  await Promise.resolve();
  await Promise.resolve();
  await nextTick();
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

const labDetail = {
  id: 'sorting',
  name: 'Sorting Lab',
  manifest: {
    Lab: { ID: 'sorting', Name: 'Sorting Lab', Tags: {} },
    Submit: { Files: ['main.c'], MaxSize: '2MB' },
    Eval: { Image: 'grader:v1', Timeout: 120 },
    Quota: { Daily: 5, Free: [] },
    Metrics: [
      { ID: 'score', Name: 'Score', Sort: 'desc', Unit: 'pts' },
      { ID: 'time_ms', Name: 'Time', Sort: 'asc', Unit: 'ms' }
    ],
    Board: { RankBy: 'score', Pick: false },
    Schedule: {
      Visible: '0001-01-01T00:00:00Z',
      Open: '2026-03-10T00:00:00Z',
      Close: '0001-01-01T00:00:00Z'
    }
  }
};

afterEach(() => {
  document.body.innerHTML = '';
  vi.restoreAllMocks();
  window.sessionStorage.clear();
  window.localStorage.clear();
});

function mountDrawer(props: { labId: string | null; open: boolean }) {
  window.sessionStorage.setItem(adminTokenStorageKey, 'test-token');
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/', component: { template: '<div/>' } }]
  });
  const el = document.createElement('div');
  document.body.appendChild(el);
  const app = createApp(LabEditDrawer, {
    labId: props.labId,
    open: props.open,
    onClose: vi.fn(),
    onSaved: vi.fn()
  });
  app.use(router);
  app.mount(el);
  return { app, el };
}

describe('LabEditDrawer', () => {
  it('renders nothing when closed', () => {
    vi.stubGlobal('fetch', vi.fn());
    const { app, el } = mountDrawer({ labId: 'sorting', open: false });

    expect(document.querySelector('[data-testid="lab-edit-drawer"]')).toBeNull();

    app.unmount();
    el.remove();
  });

  it('shows Edit title and Save changes button for existing lab', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(labDetail)));
    const { app, el } = mountDrawer({ labId: 'sorting', open: true });
    await flush();

    const drawer = document.querySelector('[data-testid="lab-edit-drawer"]')!;
    expect(drawer).not.toBeNull();
    expect(drawer.textContent).toContain('Edit: sorting');
    const saveBtn = Array.from(document.querySelectorAll('button')).find((b) =>
      b.textContent?.trim() === 'Save changes'
    );
    expect(saveBtn).toBeDefined();

    app.unmount();
    el.remove();
  });

  it('shows Lab ID field and Create lab button for new lab', async () => {
    vi.stubGlobal('fetch', vi.fn());
    const { app, el } = mountDrawer({ labId: null, open: true });
    await flush();

    const drawer = document.querySelector('[data-testid="lab-edit-drawer"]')!;
    expect(drawer.textContent).toContain('New lab');
    const createBtn = Array.from(document.querySelectorAll('button')).find((b) =>
      b.textContent?.trim() === 'Create lab'
    );
    expect(createBtn).toBeDefined();

    // Lab ID input must be present (placeholder text)
    const inputs = Array.from(document.querySelectorAll('input[type="text"]')) as HTMLInputElement[];
    const labIdInput = inputs.find((i) => i.placeholder?.includes('lab-2024'));
    expect(labIdInput).toBeDefined();

    app.unmount();
    el.remove();
  });

  it('populates form fields from fetched manifest', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(labDetail)));
    const { app, el } = mountDrawer({ labId: 'sorting', open: true });
    await flush();

    const nameInput = document.querySelector('input[type="text"]') as HTMLInputElement;
    expect(nameInput.value).toBe('Sorting Lab');

    const numberInputs = Array.from(document.querySelectorAll('input[type="number"]')) as HTMLInputElement[];
    const timeoutInput = numberInputs.find((i) => i.value === '120');
    expect(timeoutInput).toBeDefined();

    app.unmount();
    el.remove();
  });

  it('shows both existing metrics and an add-metric form', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(labDetail)));
    const { app, el } = mountDrawer({ labId: 'sorting', open: true });
    await flush();

    const body = document.querySelector('[data-testid="lab-edit-drawer"]')!;
    expect(body.textContent).toContain('score');
    expect(body.textContent).toContain('time_ms');

    // remove buttons: one per metric
    const removeButtons = Array.from(document.querySelectorAll('.metric-row__remove'));
    expect(removeButtons).toHaveLength(2);

    // add-metric form inputs exist
    const addInputs = Array.from(document.querySelectorAll('.metric-add input')) as HTMLInputElement[];
    expect(addInputs.length).toBeGreaterThanOrEqual(2);

    app.unmount();
    el.remove();
  });

  it('removes a metric when its remove button is clicked', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(labDetail)));
    const { app, el } = mountDrawer({ labId: 'sorting', open: true });
    await flush();

    const removeBtns = Array.from(document.querySelectorAll('.metric-row__remove')) as HTMLButtonElement[];
    expect(removeBtns).toHaveLength(2);

    removeBtns[0].click();
    await flush();

    const remainingRemoveBtns = Array.from(document.querySelectorAll('.metric-row__remove'));
    expect(remainingRemoveBtns).toHaveLength(1);
    expect(document.querySelector('[data-testid="lab-edit-drawer"]')!.textContent).not.toContain('score');
    expect(document.querySelector('[data-testid="lab-edit-drawer"]')!.textContent).toContain('time_ms');

    app.unmount();
    el.remove();
  });

  it('adds a new metric via the add-metric form', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(labDetail)));
    const { app, el } = mountDrawer({ labId: 'sorting', open: true });
    await flush();

    const idInput = document.querySelector('.metric-add__id') as HTMLInputElement;
    const nameInput = document.querySelector('.metric-add__name') as HTMLInputElement;
    const unitInput = document.querySelector('.metric-add__unit') as HTMLInputElement;
    idInput.value = 'accuracy';
    idInput.dispatchEvent(new Event('input'));
    nameInput.value = 'Accuracy';
    nameInput.dispatchEvent(new Event('input'));
    unitInput.value = '%';
    unitInput.dispatchEvent(new Event('input'));

    const addForm = document.querySelector('.metric-add') as HTMLFormElement;
    addForm.dispatchEvent(new Event('submit'));
    await flush();

    const metricRows = Array.from(document.querySelectorAll('.metric-row'));
    expect(metricRows).toHaveLength(3);
    expect(document.querySelector('[data-testid="lab-edit-drawer"]')!.textContent).toContain('accuracy');

    app.unmount();
    el.remove();
  });

  it('switches to TOML mode and shows a textarea with generated TOML', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(labDetail)));
    const { app, el } = mountDrawer({ labId: 'sorting', open: true });
    await flush();

    const tomlTab = Array.from(document.querySelectorAll('.drawer__tab')).find((b) =>
      b.textContent?.trim() === 'TOML'
    ) as HTMLButtonElement;
    expect(tomlTab).toBeDefined();
    tomlTab.click();
    await flush();

    const textarea = document.querySelector('.toml-editor') as HTMLTextAreaElement;
    expect(textarea).not.toBeNull();
    expect(textarea.value).toContain('[lab]');
    expect(textarea.value).toContain('name = "Sorting Lab"');
    expect(textarea.value).toContain('[[metric]]');
    expect(textarea.value).toContain('id = "score"');

    app.unmount();
    el.remove();
  });

  it('switches back to form mode by parsing TOML from textarea', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(labDetail)));
    const { app, el } = mountDrawer({ labId: 'sorting', open: true });
    await flush();

    // switch to TOML
    const tomlTab = Array.from(document.querySelectorAll('.drawer__tab')).find((b) =>
      b.textContent?.trim() === 'TOML'
    ) as HTMLButtonElement;
    tomlTab.click();
    await flush();

    // edit TOML to change name
    const textarea = document.querySelector('.toml-editor') as HTMLTextAreaElement;
    textarea.value = textarea.value.replace('name = "Sorting Lab"', 'name = "Renamed Lab"');
    textarea.dispatchEvent(new Event('input'));
    await flush();

    // switch back to form
    const formTab = Array.from(document.querySelectorAll('.drawer__tab')).find((b) =>
      b.textContent?.trim() === 'Form'
    ) as HTMLButtonElement;
    formTab.click();
    await flush();

    // form fields should reflect the TOML edit
    const nameInput = document.querySelector('input[type="text"]') as HTMLInputElement;
    expect(nameInput.value).toBe('Renamed Lab');

    app.unmount();
    el.remove();
  });

  it('shows TOML parse error when TOML is invalid and stays in TOML mode', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(labDetail)));
    const { app, el } = mountDrawer({ labId: 'sorting', open: true });
    await flush();

    const tomlTab = Array.from(document.querySelectorAll('.drawer__tab')).find((b) =>
      b.textContent?.trim() === 'TOML'
    ) as HTMLButtonElement;
    tomlTab.click();
    await flush();

    const textarea = document.querySelector('.toml-editor') as HTMLTextAreaElement;
    textarea.value = '[invalid toml !!!';
    textarea.dispatchEvent(new Event('input'));

    const formTab = Array.from(document.querySelectorAll('.drawer__tab')).find((b) =>
      b.textContent?.trim() === 'Form'
    ) as HTMLButtonElement;
    formTab.click();
    await flush();

    expect(document.querySelector('.drawer__toml-error')).not.toBeNull();
    // still in TOML mode
    expect(document.querySelector('.toml-editor')).not.toBeNull();

    app.unmount();
    el.remove();
  });

  it('sends PUT request when saving an existing lab in form mode', async () => {
    const fetchMock = vi.fn(async (url: string) => {
      if (url.includes('/api/admin/labs/sorting') && !url.includes('PUT')) return jsonResponse(labDetail);
      return jsonResponse({ id: 'sorting', name: 'Sorting Lab', manifest: labDetail.manifest });
    });
    vi.stubGlobal('fetch', fetchMock);
    const { app, el } = mountDrawer({ labId: 'sorting', open: true });
    await flush();

    const saveBtn = Array.from(document.querySelectorAll('button')).find((b) =>
      b.textContent?.trim() === 'Save changes'
    ) as HTMLButtonElement;
    saveBtn.click();
    await flush();

    const saveCalls = fetchMock.mock.calls.filter(([url, opts]) =>
      String(url).includes('/api/admin/labs/sorting') && (opts as RequestInit)?.method === 'PUT'
    );
    expect(saveCalls).toHaveLength(1);
    const body = JSON.parse((saveCalls[0][1] as RequestInit).body as string);
    expect(body.Lab.Name).toBe('Sorting Lab');

    app.unmount();
    el.remove();
  });

  it('sends POST request when creating a new lab in form mode', async () => {
    const fetchMock = vi.fn(async () =>
      jsonResponse({ id: 'new-lab', name: 'New Lab', manifest: {} }, 201)
    );
    vi.stubGlobal('fetch', fetchMock);
    const { app, el } = mountDrawer({ labId: null, open: true });
    await flush();

    const idInput = document.querySelector('input[placeholder*="lab-2024"]') as HTMLInputElement;
    idInput.value = 'new-lab';
    idInput.dispatchEvent(new Event('input'));

    const nameInput = document.querySelector('input[type="text"]') as HTMLInputElement;
    // find the Name input (second text input after Lab ID)
    const textInputs = Array.from(document.querySelectorAll('input[type="text"]')) as HTMLInputElement[];
    const nameField = textInputs[1];
    nameField.value = 'New Lab';
    nameField.dispatchEvent(new Event('input'));
    await flush();

    const createBtn = Array.from(document.querySelectorAll('button')).find((b) =>
      b.textContent?.trim() === 'Create lab'
    ) as HTMLButtonElement;
    createBtn.click();
    await flush();

    const postCalls = fetchMock.mock.calls.filter(([url, opts]) =>
      String(url) === '/api/admin/labs' && (opts as RequestInit)?.method === 'POST'
    );
    expect(postCalls).toHaveLength(1);
    const body = JSON.parse((postCalls[0][1] as RequestInit).body as string);
    expect(body.Lab.ID).toBe('new-lab');

    app.unmount();
    el.remove();
  });

  it('sends raw TOML body when saving in TOML mode', async () => {
    const fetchMock = vi.fn(async (url: string) => {
      if (url.includes('/api/admin/labs/sorting')) return jsonResponse(labDetail);
      return jsonResponse({ id: 'sorting' });
    });
    vi.stubGlobal('fetch', fetchMock);
    const { app, el } = mountDrawer({ labId: 'sorting', open: true });
    await flush();

    const tomlTab = Array.from(document.querySelectorAll('.drawer__tab')).find((b) =>
      b.textContent?.trim() === 'TOML'
    ) as HTMLButtonElement;
    tomlTab.click();
    await flush();

    const saveBtn = Array.from(document.querySelectorAll('button')).find((b) =>
      b.textContent?.trim() === 'Save changes'
    ) as HTMLButtonElement;
    saveBtn.click();
    await flush();

    const saveCalls = fetchMock.mock.calls.filter(([url, opts]) =>
      String(url).includes('/api/admin/labs/sorting') && (opts as RequestInit)?.method === 'PUT'
    );
    expect(saveCalls).toHaveLength(1);
    const opts = saveCalls[0][1] as RequestInit;
    const headers = opts.headers as Headers;
    expect(headers.get('Content-Type')).toContain('text/plain');
    expect(typeof opts.body).toBe('string');
    expect(opts.body as string).toContain('[lab]');

    app.unmount();
    el.remove();
  });
});
