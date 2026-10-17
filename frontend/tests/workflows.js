import { expect } from '@playwright/test';

export async function verifyWorkflowPermissions(request, token, invigilator) {
  const headers = { Authorization: `Bearer ${token}` };
  for (const [method, path] of [['GET', 'dashboard'], ['GET', 'reports/workload'], ['GET', 'reports/availability'], ['GET', 'audit-logs'], ['GET', 'audit-logs/1'], ['POST', 'unavailability-requests/1/decision'], ['POST', 'schedule/automatic-planning'], ['POST', 'invigilator-assignments/automatic'], ['PUT', 'schedule/publication'], ['DELETE', 'schedule']]) {
    expect((await request.fetch(`/api/v1/${path}`, { method, headers })).status(), `${method} ${path}`).toBe(403);
  }
  for (const path of ['me/duties', 'me/workload']) {
    const response = await request.get(`/api/v1/${path}`, { headers });
    expect(response.status()).toBe(invigilator ? 200 : 403);
    if (invigilator) {
      const value = await response.json();
      if (path === 'me/duties') {
        expect(value.total).toBe(1);
        expect(value.data[0]).toMatchObject({ personnel_id: 1, course_code: 'WAD301', classroom: 'Lecture Hall' });
      } else expect(value).toMatchObject({ personnel_id: 1, duties: 1 });
    }
  }
  if (invigilator) {
    expect((await request.get('/api/v1/me/duties?personnel_id=2', { headers })).status()).toBe(400);
    expect((await request.get('/api/v1/me/workload?page=1', { headers })).status()).toBe(400);
    expect(await (await request.get('/api/v1/me/duties?page=999&page_size=1', { headers })).json()).toMatchObject({ data: [], total: 1 });
    expect(await (await request.get('/api/v1/me/workload?date_from=2026-11-03', { headers })).json()).toMatchObject({ duties: 0 });
  }
}

export async function verifyWorkflowAPI(request, token) {
  const headers = { Authorization: `Bearer ${token}` };
  const call = async (path, method = 'GET', data, status = 200) => {
    const response = await request.fetch(`/api/v1/${path}`, { method, headers, ...(data === undefined ? {} : { data }) });
    expect(response.status(), `${method} ${path}: ${await response.text()}`).toBe(status);
    return status === 204 ? null : response.json();
  };
  const profile = await call('me');
  let dashboard = await call('dashboard');
  expect(dashboard.totals).toMatchObject({ courses: 3, exams: 3, unplanned_courses: 0, unstaffed_rooms: 0 });
  expect((await call('schedule/publication', 'PUT', { published: true })).published).toBe(true);
  expect((await call('schedule')).total).toBe(3);
  expect((await call('reports/workload?page=999&page_size=1')).data).toEqual([]);
  expect((await call('reports/workload?page=999&page_size=1')).total).toBe(3);
  const zero = await call('reports/workload?date_from=2026-11-03');
  expect(zero.total).toBe(3);
  expect(zero.data.every((row) => row.duties === 0)).toBe(true);
  const availability = await call('reports/availability?personnel_id=1&session_id=1');
  expect(availability.data[0]).toMatchObject({ personnel_id: 1, session_id: 1, available: false, reason_code: 'personnel_conflict' });
  expect((await call('reports/availability?personnel_id=1&session_id=2')).data[0].available).toBe(true);
  await call('schedule', 'DELETE', undefined, 204);
  dashboard = await call('dashboard');
  expect(dashboard.totals).toMatchObject({ courses: 3, personnel: 3, exams: 0, unplanned_courses: 3, unstaffed_rooms: 0 });
  expect(dashboard.publication.published).toBe(false);
  expect((await call('schedule/automatic-planning', 'POST', { season: 'spring' })).count).toBe(0);
  expect((await call('schedule/automatic-planning', 'POST', { season: 'autumn' })).count).toBe(3);
  expect((await call('schedule/automatic-planning', 'POST', { season: 'all' })).count).toBe(0);
  await call('schedule/publication', 'PUT', { published: true }, 409);
  const exams = await call('exams');
  const allocation = exams.data[0].rooms[0].id;
  expect(await call('invigilator-assignments/automatic', 'POST', { scope: 'one', allocation_id: allocation })).toMatchObject({ count: 1, ids: [allocation] });
  await call('invigilator-assignments/automatic', 'POST', { scope: 'one', allocation_id: allocation }, 409);
  await call('invigilator-assignments/automatic', 'POST', { scope: 'one', allocation_id: 'not-an-id' }, 400);
  await call('invigilator-assignments/automatic', 'POST', { scope: 'one', allocation_id: Number.MAX_SAFE_INTEGER }, 404);
  await call('invigilator-assignments/automatic', 'POST', { scope: 'all', allocation_id: allocation }, 400);
  expect((await call('invigilator-assignments/automatic', 'POST', { scope: 'all' })).count).toBe(2);
  expect((await call('invigilator-assignments/automatic', 'POST', { scope: 'all' })).count).toBe(0);
  await call('schedule/publication', 'PUT', { published: true });
  const conflict = await call('unavailability-requests', 'POST', { personnel_id: 1, date: '2026-11-02', session_id: null, type: 'leave', reason: 'Workflow conflict fixture' }, 201);
  await call(`unavailability-requests/${conflict.id}/decision`, 'POST', { approved: true }, 409);
  expect((await call(`unavailability-requests/${conflict.id}`)).status).toBe('pending');
  const rejected = await call(`unavailability-requests/${conflict.id}/decision`, 'POST', { approved: false });
  expect(rejected.status).toBe('rejected');
  expect(rejected.decided_by).toBe(profile.id);
  await call(`unavailability-requests/${conflict.id}/decision`, 'POST', { approved: true }, 409);
  await call(`unavailability-requests/${conflict.id}`, 'DELETE', undefined, 409);
  const allowed = await call('unavailability-requests', 'POST', { personnel_id: 1, date: '2026-11-03', session_id: null, type: 'medical', reason: 'Workflow approval fixture' }, 201);
  const approved = await call(`unavailability-requests/${allowed.id}/decision`, 'POST', { approved: true });
  expect(approved).toMatchObject({ status: 'approved', decided_by: profile.id });
  expect(approved.decided_at).toEqual(expect.any(String));
  const audited = await call(`audit-logs?actor_user_id=${profile.id}&entity=exams&action=clear`);
  expect(audited.total).toBe(1);
  const detail = await call(`audit-logs/${audited.data[0].id}`);
  expect(detail.old_value.exams).toHaveLength(3);
  expect(detail.new_value.exams).toHaveLength(0);
  await call('audit-logs', 'POST', {}, 404);
  const valid = await call('courses', 'POST', { code: 'WFGOOD', name: 'Workflow good candidate', kind: 'elective', enrollment: 30, term: 2, department_id: 1 }, 201);
  const impossible = await call('courses', 'POST', { code: 'WFBAD', name: 'Workflow oversized candidate', kind: 'elective', enrollment: 1000, term: 2, department_id: 2 }, 201);
  try {
    const failure = await call('schedule/automatic-planning', 'POST', { season: 'spring' }, 409);
    expect(failure.error.code).toBe('planning_unresolved');
    expect((await call('dashboard')).totals).toMatchObject({ exams: 3, unplanned_courses: 2 });
    expect((await call(`exams?course_id=${valid.id}`)).total).toBe(0);
  } finally {
    await call(`courses/${impossible.id}`, 'DELETE', undefined, 204);
    await call(`courses/${valid.id}`, 'DELETE', undefined, 204);
  }
  await call('schedule/publication', 'PUT', { published: true });
}
