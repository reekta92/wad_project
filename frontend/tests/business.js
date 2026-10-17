import { expect } from '@playwright/test';

export async function verifyBusinessAPI(request, token) {
  const headers = { Authorization: `Bearer ${token}`, Accept: 'application/json' };
  const created = [];
  const create = async (resource, input, update, query) => {
    const response = await request.post(`/api/v1/${resource}`, { headers, data: input });
    expect(response.status(), `${resource} create: ${await response.text()}`).toBe(201);
    const value = await response.json();
    created.push([resource, value.id]);
    expect(response.headers().location).toBe(`/api/v1/${resource}/${value.id}`);
    expect(response.headers()['cache-control']).toBe('no-store');
    const read = await request.get(response.headers().location, { headers });
    expect(read.status()).toBe(200);
    expect(await read.json()).toEqual(value);
    const changed = await request.put(response.headers().location, { headers, data: update || input });
    expect(changed.status(), `${resource} update: ${await changed.text()}`).toBe(200);
    expect(await changed.json()).toMatchObject(update || input);
    const list = await request.get(`/api/v1/${resource}?page=1&page_size=1&${query}`, { headers });
    expect(list.status()).toBe(200);
    const page = await list.json();
    expect(page.total).toBe(1);
    expect(page.data).toHaveLength(1);
    expect(page.data[0].id).toBe(value.id);
    const empty = await request.get(`/api/v1/${resource}?page=999&page_size=1&${query}`, { headers });
    expect(await empty.json()).toMatchObject({ data: [], total: 1, page: 999, page_size: 1 });
    return changed.json();
  };
  try {
    const department = await create('departments', { name: 'API temporary department' }, { name: 'API revised department' }, 'name=API%20revised');
    const courseInput = { code: 'API101', name: 'API temporary course', kind: 'elective', enrollment: 30, term: 1, department_id: department.id };
    const course = await create('courses', courseInput, { ...courseInput, name: 'API revised course' }, 'q=API101');
    const classroomInput = { name: 'API temporary room', capacity: 20, type: 'classroom', floor: 27, active: false };
    await create('classrooms', classroomInput, { ...classroomInput, active: true }, 'floor=27&active=true&min_capacity=20');
    const sessionInput = { date: '2026-11-06', start_time: '09:00:00', end_time: '10:00:00', description: 'API temporary session' };
    const session = await create('sessions', sessionInput, { ...sessionInput, description: 'API revised session' }, 'date_from=2026-11-06&date_to=2026-11-06');
    const personnelInput = { title: 'Dr.', first_name: 'API', last_name: 'Temporary', department_id: department.id };
    const personnel = await create('personnel', personnelInput, { ...personnelInput, last_name: 'Revised' }, 'q=API%20Revised');
    const requestInput = { personnel_id: personnel.id, date: '2026-11-07', session_id: null, type: 'leave', reason: 'API temporary request' };
    await create('unavailability-requests', requestInput, { ...requestInput, reason: 'API revised request' }, `personnel_id=${personnel.id}&status=pending&type=leave`);
    const exam = await create('exams', { course_id: course.id, session_id: session.id }, null, `course_id=${course.id}`);
    expect(exam.rooms.length).toBeGreaterThan(0);
    await create('invigilator-assignments', { id: exam.rooms[0].id, personnel_id: personnel.id }, { personnel_id: personnel.id }, `exam_id=${exam.id}`);
    expect((await request.delete(`/api/v1/departments/${department.id}`, { headers })).status()).toBe(409);
    expect((await request.post('/api/v1/courses', { headers, data: courseInput })).status()).toBe(409);
    expect((await request.put(`/api/v1/courses/${course.id}`, { headers, data: { name: 'Incomplete replacement' } })).status()).toBe(400);
    expect((await request.post('/api/v1/departments', { headers: { ...headers, 'Content-Type': 'text/plain' }, data: '{}' })).status()).toBe(415);
    expect((await request.get('/api/v1/courses?sort=untrusted', { headers })).status()).toBe(400);
    expect((await request.get('/api/v1/courses/0', { headers })).status()).toBe(400);
    expect((await request.get('/api/v1/courses/9223372036854775807', { headers })).status()).toBe(404);
    expect((await request.post('/api/v1/exams', { headers, data: { course_id: course.id, session_id: session.id } })).status()).toBe(409);
    const schedule = await request.get('/api/v1/schedule');
    expect(await schedule.json()).toMatchObject({ data: [], total: 0 });
  } finally {
    for (const [resource, id] of created.reverse()) {
      const removed = await request.delete(`/api/v1/${resource}/${id}`, { headers });
      expect(removed.status(), `${resource} delete: ${await removed.text()}`).toBe(204);
      expect(await removed.text()).toBe('');
      expect((await request.get(`/api/v1/${resource}/${id}`, { headers })).status()).toBe(404);
    }
  }
}

export async function verifyMemberAPI(request, token, invigilator) {
  const headers = { Authorization: `Bearer ${token}`, Accept: 'application/json' };
  for (const resource of ['departments', 'courses', 'classrooms', 'sessions', 'personnel', 'exams', 'invigilator-assignments', ...(!invigilator ? ['unavailability-requests'] : [])]) {
    for (const [method, suffix] of [['GET', ''], ['GET', '/1'], ['POST', ''], ['PUT', '/1'], ['DELETE', '/1']]) {
      const response = await request.fetch(`/api/v1/${resource}${suffix}`, { method, headers });
      expect(response.status(), `${method} ${resource} should be forbidden`).toBe(403);
    }
  }
  if (!invigilator) return;
  const input = { date: '2026-11-04', session_id: null, type: 'medical', reason: 'A member-owned request' };
  expect((await request.post('/api/v1/unavailability-requests', { headers, data: { ...input, personnel_id: 2 } })).status()).toBe(403);
  expect((await request.post('/api/v1/unavailability-requests', { headers, data: { ...input, status: 'approved' } })).status()).toBe(400);
  const response = await request.post('/api/v1/unavailability-requests', { headers, data: input });
  expect(response.status()).toBe(201);
  const item = await response.json();
  try {
    expect(item).toMatchObject({ ...input, personnel_id: 1, status: 'pending', decided_at: null, decided_by: null });
    const read = await request.get(`/api/v1/unavailability-requests/${item.id}`, { headers });
    expect(await read.json()).toEqual(item);
    const update = await request.put(`/api/v1/unavailability-requests/${item.id}`, { headers, data: { ...input, reason: 'Updated by its owner' } });
    expect(update.status()).toBe(200);
    expect(await update.json()).toMatchObject({ reason: 'Updated by its owner', personnel_id: 1, status: 'pending' });
    const foreign = await request.get('/api/v1/unavailability-requests?personnel_id=2', { headers });
    expect(await foreign.json()).toMatchObject({ data: [], total: 0 });
  } finally {
    expect((await request.delete(`/api/v1/unavailability-requests/${item.id}`, { headers })).status()).toBe(204);
  }
}
