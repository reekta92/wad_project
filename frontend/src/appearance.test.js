import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { runInNewContext } from 'node:vm';

const script = readFileSync(new URL('../public/appearance.js', import.meta.url), 'utf8');

for (const saved of [null, 'system', 'light', 'dark', 'invalid']) {
  test(`appearance startup validates saved theme ${saved}`, () => {
    const document = { documentElement: { dataset: {} } };
    runInNewContext(script, { document, localStorage: { getItem: () => saved } });
    assert.equal(document.documentElement.dataset.theme, ['light', 'dark'].includes(saved) ? saved : 'system');
  });
}

test('appearance startup tolerates blocked browser storage', () => {
  const document = { documentElement: { dataset: {} } };
  runInNewContext(script, { document, localStorage: { getItem: () => { throw new Error('Storage is blocked'); } } });
  assert.equal(document.documentElement.dataset.theme, 'system');
});
