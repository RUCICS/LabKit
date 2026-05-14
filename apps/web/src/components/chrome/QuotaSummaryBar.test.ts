import { afterEach, describe, expect, it } from 'vitest';
import { createApp, h } from 'vue';
import QuotaSummaryBar from './QuotaSummaryBar.vue';
import type { QuotaSummary } from '../board/types';

function mount(quota: QuotaSummary | null) {
  const el = document.createElement('div');
  document.body.appendChild(el);
  const app = createApp({ render: () => h(QuotaSummaryBar, { quota }) });
  app.mount(el);
  return {
    text: () => document.body.textContent ?? '',
    hint: () => document.querySelector('[data-testid="quota-bonus-hint"]'),
    bonus: () => document.querySelector('[data-testid="quota-bonus"]'),
    unmount() {
      app.unmount();
      el.remove();
    },
  };
}

afterEach(() => {
  document.body.innerHTML = '';
});

describe('QuotaSummaryBar', () => {
  it('renders bonus tail when bonus.remaining > 0', () => {
    const view = mount({
      daily: 3,
      used: 1,
      left: 2,
      reset_hint: '00:00 Asia/Shanghai',
      bonus: { remaining: 5 },
    });
    expect(view.text()).toContain('2 left today');
    expect(view.bonus()).not.toBeNull();
    expect(view.bonus()?.textContent).toContain('5 bonus left');
    expect(view.hint()).toBeNull();
    view.unmount();
  });

  it('shows the exhaustion hint when daily is empty but bonus remains', () => {
    const view = mount({
      daily: 3,
      used: 3,
      left: 0,
      reset_hint: '00:00 Asia/Shanghai',
      bonus: { remaining: 4 },
    });
    expect(view.hint()).not.toBeNull();
    expect(view.hint()?.textContent).toContain('next submission will spend 1 bonus credit');
    view.unmount();
  });

  it('omits bonus output when no bonus quota is set', () => {
    const view = mount({
      daily: 3,
      used: 0,
      left: 3,
      reset_hint: '00:00 Asia/Shanghai',
    });
    expect(view.bonus()).toBeNull();
    expect(view.hint()).toBeNull();
    view.unmount();
  });

  it('omits hint when daily exhausted but no bonus available', () => {
    const view = mount({
      daily: 3,
      used: 3,
      left: 0,
      reset_hint: '00:00 Asia/Shanghai',
      bonus: { remaining: 0 },
    });
    expect(view.bonus()).toBeNull();
    expect(view.hint()).toBeNull();
    view.unmount();
  });
});
