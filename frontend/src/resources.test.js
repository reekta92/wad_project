import { test } from 'node:test';
import assert from 'node:assert/strict';
import { resourceBody, resourceFields } from './resources.js';

test('form bodies preserve zero/false and clear an optional session with null', () => {
  const fields = resourceFields('classrooms', true, false);
  assert.deepEqual(resourceBody(fields, { name: ' Room ', capacity: '40', type: 'classroom', floor: '0', active: false, id: 123 }), { name: 'Room', capacity: 40, type: 'classroom', floor: 0, active: false });
  const request = resourceBody(resourceFields('unavailability-requests', false, false), { date: '2026-11-02', session_id: '', type: 'leave', reason: ' Leave ', status: 'approved', personnel_id: 999 });
  assert.deepEqual(request, { date: '2026-11-02', session_id: null, type: 'leave', reason: 'Leave' });
});

test('only assignment creation includes its allocation ID; owner fields stay administrator-only', () => {
  assert.deepEqual(resourceFields('invigilator-assignments', true, true).map((field) => field.name), ['personnel_id']);
  assert.deepEqual(resourceFields('invigilator-assignments', true, false).map((field) => field.name), ['id', 'personnel_id']);
  assert.equal(resourceFields('unavailability-requests', true, true).find((field) => field.name === 'personnel_id').immutable, true);
  assert.equal(resourceFields('unavailability-requests', false, true).some((field) => field.name === 'personnel_id'), false);
});
