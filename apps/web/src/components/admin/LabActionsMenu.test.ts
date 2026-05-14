import { afterEach, describe, expect, it, vi } from 'vitest';
import { createApp, defineComponent, h, nextTick, ref } from 'vue';
import LabActionsMenu from './LabActionsMenu.vue';

async function flush() {
  await Promise.resolve();
  await nextTick();
}

function mount(onPick: (action: string) => void) {
  const el = document.createElement('div');
  document.body.appendChild(el);
  const lastAction = ref<string | null>(null);
  const app = createApp(
    defineComponent({
      render: () =>
        h(LabActionsMenu, {
          labId: 'sorting',
          onPick: (action: string) => {
            lastAction.value = action;
            onPick(action);
          },
        }),
    }),
  );
  app.mount(el);
  return {
    el,
    lastAction,
    trigger: () => document.querySelector('[data-testid="lab-actions-trigger-sorting"]') as HTMLButtonElement,
    menu: () => document.querySelector('[role="menu"]'),
    item: (action: string) =>
      document.querySelector(`[data-testid="lab-actions-sorting-${action}"]`) as HTMLButtonElement | null,
    unmount() {
      app.unmount();
      el.remove();
    },
  };
}

afterEach(() => {
  document.body.innerHTML = '';
  vi.restoreAllMocks();
});

describe('LabActionsMenu', () => {
  it('toggles the menu and exposes three quota actions', async () => {
    const view = mount(() => {});
    expect(view.menu()).toBeNull();

    view.trigger().click();
    await flush();
    expect(view.menu()).not.toBeNull();
    expect(view.item('reset-daily')).not.toBeNull();
    expect(view.item('grant-bonus')).not.toBeNull();
    expect(view.item('reset-bonus')).not.toBeNull();

    view.unmount();
  });

  it('emits the picked action and closes the menu', async () => {
    const pick = vi.fn();
    const view = mount(pick);

    view.trigger().click();
    await flush();
    view.item('grant-bonus')!.click();
    await flush();

    expect(pick).toHaveBeenCalledWith('grant-bonus');
    expect(view.menu()).toBeNull();
    view.unmount();
  });

  it('closes when clicking outside', async () => {
    const view = mount(() => {});
    view.trigger().click();
    await flush();

    document.body.click();
    await flush();
    expect(view.menu()).toBeNull();
    view.unmount();
  });

  it('closes on Escape', async () => {
    const view = mount(() => {});
    view.trigger().click();
    await flush();

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    await flush();
    expect(view.menu()).toBeNull();
    view.unmount();
  });
});
