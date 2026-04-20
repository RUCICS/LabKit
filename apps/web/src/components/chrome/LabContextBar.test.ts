import { beforeEach, describe, expect, it } from 'vitest';
import { createApp } from 'vue';
import LabContextBar from './LabContextBar.vue';

function mountContextBar(props: {
  title: string;
  labId: string;
  remainingValue: string;
  closesAtValue?: string;
}) {
  const el = document.createElement('div');
  document.body.appendChild(el);
  const app = createApp(LabContextBar, props);
  app.mount(el);
  return {
    el,
    unmount() {
      app.unmount();
      el.remove();
    }
  };
}

describe('LabContextBar', () => {
  beforeEach(() => {
    document.body.innerHTML = '';
  });

  it('renders title, lab id, and remaining', () => {
    const view = mountContextBar({
      title: 'CoLab 调度器竞赛',
      labId: 'colab-2026-p2',
      remainingValue: '34d'
    });

    expect(document.body.textContent).toContain('CoLab 调度器竞赛');
    expect(document.body.textContent).toContain('colab-2026-p2');
    expect(document.body.textContent).toContain('REMAINING');
    expect(document.body.textContent).toContain('34d');

    view.unmount();
  });

  it('renders closes value as secondary context when provided', () => {
    const view = mountContextBar({
      title: 'CoLab 调度器竞赛',
      labId: 'colab-2026-p2',
      remainingValue: '34d',
      closesAtValue: '06/01'
    });

    expect(document.body.textContent).toContain('CLOSES');
    expect(document.body.textContent).toContain('06/01');

    view.unmount();
  });
});

