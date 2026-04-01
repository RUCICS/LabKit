import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

describe('root index.html', () => {
  it('provides the Vue mount point for the built app', () => {
    const html = readFileSync(resolve(import.meta.dirname, '../index.html'), 'utf8');

    expect(html).toContain('<div id="app"></div>');
    expect(html).toContain('<script type="module" src="/src/main.ts"></script>');
    expect(html).not.toContain('Local deployment placeholder.');
  });
});
