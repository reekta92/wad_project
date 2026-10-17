import { afterEach, mock, test } from 'node:test';
import assert from 'node:assert/strict';
import { fetchSchedule } from './api.js';

afterEach(() => mock.restoreAll());

test('uses the JSON API and preserves pagination and filter results', async () => {
  const page = { data: [{ id: 1, course_code: 'WAD301', course_name: 'Web', department: 'Computer Engineering', term: 5, date: '2026-11-02', start_time: '09:00:00', end_time: '10:00:00', rooms: [] }], total: 1, page: 2, page_size: 1 };
  mock.method(globalThis, 'fetch', async (url, options) => {
    assert.equal(url, '/api/v1/schedule?page=2&q=Web');
    assert.equal(options.headers.Accept, 'application/json');
    return new Response(JSON.stringify(page));
  });
  assert.deepEqual(await fetchSchedule('page=2&q=Web'), page);
});

test('shows the safe server error instead of treating HTTP errors as empty schedules', async () => {
  mock.method(globalThis, 'fetch', async () => new Response(JSON.stringify({ error: { message: 'Schedule data is temporarily unavailable.' } }), { status: 503 }));
  await assert.rejects(fetchSchedule(''), /temporarily unavailable/);
});

test('rejects HTML fallback and malformed pagination', async () => {
  mock.method(globalThis, 'fetch', async () => new Response('<html>SPA fallback</html>'));
  await assert.rejects(fetchSchedule(''), /unreadable response/);
  mock.restoreAll();
  mock.method(globalThis, 'fetch', async () => new Response(JSON.stringify({ data: [], total: -1, page: 1, page_size: 20 })));
  await assert.rejects(fetchSchedule(''), /incomplete information/);
});

test('rejects incomplete exam data before rendering', async () => {
  mock.method(globalThis, 'fetch', async () => new Response(JSON.stringify({ data: [{ id: 1, rooms: null }], total: 1, page: 1, page_size: 20 })));
  await assert.rejects(fetchSchedule(''), /incomplete information/);
});
