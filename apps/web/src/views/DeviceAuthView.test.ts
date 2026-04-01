import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createApp, nextTick } from 'vue';
import DeviceAuthView from './DeviceAuthView.vue';

async function flush() {
  await Promise.resolve();
  await Promise.resolve();
  await nextTick();
}

async function mountView(url = '/auth/device') {
  const el = document.createElement('div');
  document.body.appendChild(el);
  window.history.pushState({}, '', url);
  const app = createApp(DeviceAuthView);
  app.mount(el);
  await flush();
  return {
    unmount() {
      app.unmount();
      el.remove();
    }
  };
}

beforeEach(() => {
  document.body.innerHTML = '';
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe('DeviceAuthView', () => {
  it('prefills the user code from the query string', async () => {
    const view = await mountView('/auth/device?user_code=ABCD-EFGH');

    const inputs = Array.from(document.querySelectorAll('.code-input')) as HTMLInputElement[];
    expect(inputs).toHaveLength(8);
    expect(inputs.map((input) => input.value).join('')).toBe('ABCDEFGH');
    expect(document.querySelectorAll('[data-testid="user-code-cell"]').length).toBe(8);
    expect(document.body.textContent).toContain('设备授权');
    expect(document.body.textContent).toContain('请输入终端中');
    expect(document.body.textContent).toContain('labkit auth');
    expect(document.body.textContent).not.toContain('通过学校 SSO 认证');
    expect(document.querySelector('svg')).not.toBeNull();

    view.unmount();
  });

  it('redirects to the api verification endpoint when submitted', async () => {
    const assign = vi.fn();
    Object.defineProperty(window, 'location', {
      value: { assign },
      configurable: true
    });

    const view = await mountView('/auth/device');

    const inputs = Array.from(document.querySelectorAll('.code-input')) as HTMLInputElement[];
    expect(inputs).toHaveLength(8);
    'ABCDEFGH'.split('').forEach((char, index) => {
      inputs[index].value = char;
      inputs[index].dispatchEvent(new Event('input', { bubbles: true }));
    });
    await flush();

    const form = document.querySelector('form');
    expect(form).not.toBeNull();
    form!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
    await flush();

    expect((document.querySelector('button[type="submit"]') as HTMLButtonElement | null)?.disabled).toBe(true);
    expect(assign).toHaveBeenCalledWith('/api/device/verify?user_code=ABCD-EFGH');

    view.unmount();
  });

  it('shows validation feedback for an empty user code', async () => {
    const assign = vi.fn();
    Object.defineProperty(window, 'location', {
      value: { assign },
      configurable: true
    });

    const view = await mountView('/auth/device');

    const form = document.querySelector('form');
    form!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
    await flush();

    expect(document.body.textContent).toContain('验证码无效或已过期');
    expect(assign).not.toHaveBeenCalled();

    view.unmount();
  });
});
