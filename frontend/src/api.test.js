import { afterEach, mock, test } from 'node:test';
import assert from 'node:assert/strict';
import { fetchProfile, fetchSchedule, requestResource } from './api.js';

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

test('profile uses a bearer token and the verified server identity', async () => {
  const profile = { id: 1, username: 'member.demo', role: 'member', permissions: [], personnel_id: null };
  mock.method(globalThis, 'fetch', async (url, options) => {
    assert.equal(url, '/api/v1/me');
    assert.equal(options.headers.Authorization, 'Bearer test-token');
    assert.equal(options.cache, 'no-store');
    return new Response(JSON.stringify(profile));
  });
  assert.deepEqual(await fetchProfile('test-token'), profile);
});

test('profile preserves denied-session status and rejects malformed permissions', async () => {
  mock.method(globalThis, 'fetch', async () => new Response(JSON.stringify({ error: { message: 'Account disabled.' } }), { status: 403 }));
  await assert.rejects(fetchProfile('test-token'), (error) => error.status === 403 && error.message === 'Account disabled.');
  mock.restoreAll();
  mock.method(globalThis, 'fetch', async () => new Response(JSON.stringify({ id: 1, username: 'demo', role: 'administrator', permissions: ['invented'], personnel_id: null })));
  await assert.rejects(fetchProfile('test-token'), /incomplete/);
});

test('resource writes send authenticated JSON and support empty 204 responses', async () => {
  mock.method(globalThis, 'fetch', async (url, options) => {
    assert.equal(url, '/api/v1/departments/7');
    assert.equal(options.headers.Authorization, 'Bearer transient-token');
    assert.equal(options.cache, 'no-store');
    if (options.method === 'PUT') {
      assert.equal(options.headers['Content-Type'], 'application/json');
      assert.deepEqual(JSON.parse(options.body), { name: 'Updated' });
      return new Response(JSON.stringify({ id: 7, name: 'Updated' }));
    }
    assert.equal(options.body, undefined);
    return new Response(null, { status: 204 });
  });
  assert.deepEqual(await requestResource('departments/7', 'transient-token', { method: 'PUT', body: { name: 'Updated' } }), { id: 7, name: 'Updated' });
  assert.equal(await requestResource('departments/7', 'transient-token', { method: 'DELETE' }), null);
});

test('resource validation errors retain field details and malformed lists fail visibly', async () => {
  mock.method(globalThis, 'fetch', async () => new Response(JSON.stringify({ error: { message: 'Check fields.', fields: { name: 'Required.' } } }), { status: 400 }));
  await assert.rejects(requestResource('departments', 'token'), (error) => error.status === 400 && error.fields.name === 'Required.');
  mock.restoreAll();
  mock.method(globalThis, 'fetch', async () => new Response(JSON.stringify({ data: [{ id: 1 }], page: 1, page_size: 0, total: 1 })));
  await assert.rejects(requestResource('departments', 'token'), /incomplete resource/);
});
