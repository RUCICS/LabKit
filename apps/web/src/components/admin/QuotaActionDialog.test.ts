import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp, defineComponent, h, nextTick, ref } from 'vue';
import QuotaActionDialog from './QuotaActionDialog.vue';
import { adminTokenStorageKey } from '../../lib/admin';

type Action = 'reset-daily' | 'grant-bonus' | 'reset-bonus' | null;

async function flush() {
  await Promise.resolve();
  await Promise.resolve();
  await nextTick();
}

function jsonResponse(body: unknown, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
    text: async () => JSON.stringify(body),
  } as Response;
}

function mount(initialAction: Action) {
  const action = ref<Action>(initialAction);
  const lastSummary = ref<string | null>(null);
  const closed = ref(0);
  const el = document.createElement('div');
  document.body.appendChild(el);
  const app = createApp(
    defineComponent({
      render: () =>
        h(QuotaActionDialog, {
          labId: 'sorting',
          labName: 'Sorting Lab',
          action: action.value,
          onClose: () => {
            closed.value += 1;
            action.value = null;
          },
          onDone: (summary: string) => {
            lastSummary.value = summary;
            action.value = null;
          },
        }),
    }),
  );
  app.mount(el);
  return {
    action,
    lastSummary,
    closed,
    confirm: () => document.querySelector('[data-testid="dialog-confirm"]') as HTMLButtonElement | null,
    cancel: () => Array.from(document.querySelectorAll('button')).find((b) => b.textContent?.trim() === 'Cancel') as HTMLButtonElement | undefined,
    delta: () => document.querySelector('[data-testid="delta-input"]') as HTMLInputElement | null,
    preview: () => document.querySelector('[data-testid="dialog-preview"]'),
    participants: () => document.querySelector('[data-testid="participant-count"]')?.textContent ?? null,
    overlay: () => document.querySelector('[data-testid="quota-action-dialog"]'),
    unmount() {
      app.unmount();
      el.remove();
    },
  };
}

beforeEach(() => {
  window.sessionStorage.setItem(adminTokenStorageKey, 'admintok');
});

afterEach(() => {
  document.body.innerHTML = '';
  window.sessionStorage.clear();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe('QuotaActionDialog', () => {
  it('shows a dry-run participant preview for grant-bonus before applying', async () => {
    const calls: Array<{ url: string; body: unknown }> = [];
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        const body = init?.body ? JSON.parse(String(init.body)) : null;
        calls.push({ url, body });
        if (body?.dry_run) {
          return jsonResponse({ lab_participants: 42, dry_run: true });
        }
        return jsonResponse({ users_affected: 42 });
      }),
    );

    const view = mount('grant-bonus');
    await flush();
    await flush();

    expect(view.participants()).toBe('42');
    expect(view.delta()?.value).toBe('5');
    expect(view.confirm()?.textContent).toContain('Grant +5');

    view.confirm()!.click();
    await flush();
    await flush();

    expect(view.lastSummary.value).toContain('42 row(s) updated');
    // Two POSTs: one dry-run preview, one apply.
    const dryRun = calls.filter((c) => (c.body as { dry_run?: boolean })?.dry_run);
    const real = calls.filter((c) => !(c.body as { dry_run?: boolean })?.dry_run);
    expect(dryRun.length).toBeGreaterThanOrEqual(1);
    expect(real.length).toBe(1);
    expect((real[0].body as { delta: number }).delta).toBe(5);
    expect(real[0].url).toMatch(/quota\/bonus$/);
    view.unmount();
  });

  it('blocks confirm when delta is 0 or non-integer', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse({ lab_participants: 5 })),
    );
    const view = mount('grant-bonus');
    await flush();
    await flush();
    const input = view.delta()!;
    input.value = '0';
    input.dispatchEvent(new Event('input'));
    await flush();
    expect(view.confirm()?.disabled).toBe(true);
    input.value = 'abc';
    input.dispatchEvent(new Event('input'));
    await flush();
    expect(view.confirm()?.disabled).toBe(true);
    input.value = '-3';
    input.dispatchEvent(new Event('input'));
    await flush();
    expect(view.confirm()?.disabled).toBe(false);
    expect(view.confirm()?.textContent).toContain('Revoke 3');
    view.unmount();
  });

  it('applies reset-daily without a delta payload', async () => {
    const calls: Array<{ url: string; init?: RequestInit }> = [];
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        calls.push({ url, init });
        const body = init?.body ? JSON.parse(String(init.body)) : null;
        if (body?.dry_run) {
          return jsonResponse({ lab_participants: 7 });
        }
        return jsonResponse({ rows_affected: 7 });
      }),
    );

    const view = mount('reset-daily');
    await flush();
    await flush();
    expect(view.participants()).toBe('7');
    expect(view.delta()).toBeNull();
    expect(view.confirm()?.textContent).toContain('Reset');

    view.confirm()!.click();
    await flush();
    await flush();

    expect(view.lastSummary.value).toContain('7 row(s) updated');
    const real = calls.find((c) => c.url.endsWith('/quota/reset'));
    expect(real).toBeDefined();
    // reset-daily should NOT send any body
    expect(real?.init?.body).toBeUndefined();
    view.unmount();
  });

  it('renders nothing when action is null', async () => {
    const view = mount(null);
    await flush();
    expect(view.overlay()).toBeNull();
    view.unmount();
  });
});
