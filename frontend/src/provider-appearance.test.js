import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { runInNewContext } from 'node:vm';

const script = readFileSync(new URL('../../infrastructure/keycloak-themes/exams/common/resources/js/appearance.js', import.meta.url), 'utf8');

function appearance(saved, blocked = false) {
  const root = { dataset: {}, classList: { toggle: (_name, enabled) => { root.dark = enabled; } } };
  const media = { matches: true, addEventListener: (_event, callback) => { media.changed = callback; } };
  runInNewContext(script, {
    document: { documentElement: root, readyState: 'complete', getElementById: () => null },
    window: { matchMedia: () => media },
    localStorage: { getItem: () => { if (blocked) throw new Error('Storage blocked'); return saved; } },
  });
  return { root, media };
}

test('provider appearance validates saved values and follows device changes only in system mode', () => {
  for (const saved of [null, 'invalid', 'system', 'light', 'dark']) {
    const { root, media } = appearance(saved);
    assert.equal(root.dataset.theme, ['light', 'dark'].includes(saved) ? saved : 'system');
    assert.equal(root.dark, saved !== 'light');
    media.matches = false;
    media.changed();
    assert.equal(root.dark, saved === 'dark');
  }
});

test('provider appearance remains usable without storage', () => {
  const { root } = appearance('light', true);
  assert.equal(root.dataset.theme, 'system');
  assert.equal(root.dark, true);
});
