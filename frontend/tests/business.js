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

async function navigate(page, label) {
  const menu = page.getByRole('button', { name: 'Open menu', exact: true });
  if (await menu.isVisible()) await menu.click();
  await page.getByRole('navigation', { name: 'Main navigation', exact: true }).getByRole('button', { name: label, exact: true }).click();
  await expect(page.locator('.management .results-toolbar')).toContainText('records found');
}

async function save(page, resource, method) {
  const response = page.waitForResponse((value) => new URL(value.url()).pathname.startsWith(`/api/v1/${resource}`) && value.request().method() === method);
  await page.getByRole('dialog').getByRole('button', { name: 'Save changes', exact: true }).click();
  const result = await response;
  expect(result.status(), await result.text()).toBe(method === 'POST' ? 201 : 200);
  await expect(page.getByRole('dialog')).toBeHidden();
  await expect(page.locator('.management .results-toolbar')).toContainText('records found');
  return result.json();
}

export async function verifyBusinessUI(page, request, token) {
  const fixtures = [];
  const headers = { Authorization: `Bearer ${token}` };
  const field = (name) => page.getByRole('dialog').locator(`#record-${name}`);
  const add = async (resource, label, singular, fill) => {
    await navigate(page, label);
    await page.getByRole('button', { name: `Add ${singular}`, exact: true }).click();
    await expect(page.getByRole('dialog')).toBeVisible();
    await fill();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    const record = await save(page, resource, 'POST');
    fixtures.push({ resource, label, singular, record });
    return record;
  };
  await page.setViewportSize({ width: 390, height: 844 });
  try {
    const department = await add('departments', 'Departments', 'department', async () => {
      await field('name').fill('Computer Engineering');
      const failure = page.waitForResponse((value) => value.request().method() === 'POST' && new URL(value.url()).pathname === '/api/v1/departments');
      await page.getByRole('dialog').getByRole('button', { name: 'Save changes' }).click();
      expect((await failure).status()).toBe(409);
      await expect(page.getByRole('dialog').getByRole('alert')).toContainText('already exists');
      await expect(field('name')).toHaveValue('Computer Engineering');
      await field('name').fill('UI temporary department');
    });
    const course = await add('courses', 'Courses', 'course', async () => {
      await field('code').fill('UI101'); await field('name').fill('UI course'); await field('kind').selectOption('elective');
      await field('enrollment').fill('30'); await field('term').fill('1'); await field('department_id').selectOption(String(department.id));
    });
    await add('classrooms', 'Classrooms', 'classroom', async () => {
      await field('name').fill('UI temporary room'); await field('capacity').fill('20'); await field('type').fill('classroom');
      await field('floor').fill('0'); await field('active').uncheck();
    });
    const session = await add('sessions', 'Sessions', 'session', async () => {
      await field('date').fill('2026-11-06'); await field('start_time').fill('09:00'); await field('end_time').fill('10:00');
      await field('description').fill('UI temporary session');
    });
    const personnel = await add('personnel', 'Personnel', 'personnel record', async () => {
      await field('title').fill('Dr.'); await field('first_name').fill('UI'); await field('last_name').fill('Temporary'); await field('department_id').selectOption(String(department.id));
    });
    await add('unavailability-requests', 'Requests', 'request', async () => {
      await field('personnel_id').selectOption(String(personnel.id)); await field('date').fill('2026-11-07');
      await field('type').selectOption('leave'); await field('reason').fill('UI temporary request');
    });
    const exam = await add('exams', 'Exams', 'exam', async () => {
      await field('course_id').selectOption(String(course.id)); await field('session_id').selectOption(String(session.id));
    });
    await add('invigilator-assignments', 'Assignments', 'assignment', async () => {
      await field('id').selectOption(String(exam.rooms[0].id)); await field('personnel_id').selectOption(String(personnel.id));
    });
    for (const { resource, label, record } of fixtures) {
      await navigate(page, label);
      const card = page.locator('.record-card').filter({ has: page.getByText(`#${record.id}`, { exact: true }) });
      await card.getByRole('button', { name: 'Details', exact: true }).click();
      await expect(page.getByRole('dialog').getByRole('heading', { name: 'Record details', exact: true })).toBeVisible();
      await page.getByRole('dialog').getByRole('button', { name: 'Edit', exact: true }).click();
      if (resource === 'departments') await field('name').fill('UI revised department');
      if (resource === 'courses') await field('enrollment').fill('40');
      if (resource === 'classrooms') { await field('capacity').fill('25'); await field('active').check(); }
      if (resource === 'sessions') await field('description').fill('UI revised session');
      if (resource === 'personnel') await field('last_name').fill('Revised');
      if (resource === 'unavailability-requests') { await expect(field('personnel_id')).toBeDisabled(); await field('reason').fill('UI revised request'); }
      if (resource === 'invigilator-assignments') { await expect(field('id')).toHaveCount(0); await field('personnel_id').selectOption('1'); }
      await save(page, resource, 'PUT');
      for (const width of [320, 768, 1440]) {
        await page.setViewportSize({ width, height: 844 });
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      }
      await page.setViewportSize({ width: 390, height: 844 });
    }
    await navigate(page, 'Departments');
    await page.locator('.resource-filters').getByLabel('Department name', { exact: true }).fill('UI revised');
    await page.getByRole('button', { name: 'Apply filters', exact: true }).click();
    await expect(page.locator('.record-card')).toHaveCount(1);
    for (const fixture of [...fixtures].reverse()) {
      await navigate(page, fixture.label);
      await page.getByRole('button', { name: `Delete ${fixture.singular} #${fixture.record.id}`, exact: true }).click();
      const dialog = page.getByRole('dialog');
      await expect(dialog).toBeVisible();
      await expect(dialog.getByRole('button', { name: 'Cancel', exact: true })).toBeFocused();
      const removed = page.waitForResponse((value) => value.request().method() === 'DELETE' && new URL(value.url()).pathname === `/api/v1/${fixture.resource}/${fixture.record.id}`);
      await dialog.getByRole('button', { name: 'Delete record', exact: true }).click();
      expect((await removed).status()).toBe(204);
      await expect(dialog).toBeHidden();
      await expect(page.getByRole('button', { name: `Add ${fixture.singular}`, exact: true })).toBeFocused();
    }
    fixtures.length = 0;
  } finally {
    for (const { resource, record } of fixtures.reverse()) await request.delete(`/api/v1/${resource}/${record.id}`, { headers });
    await page.setViewportSize({ width: 1280, height: 720 });
  }
}

export async function verifyOwnedRequestUI(page) {
  await expect(page.getByRole('button', { name: 'Departments', exact: true })).toHaveCount(0);
  await navigate(page, 'My requests');
  await page.getByRole('button', { name: 'Add request', exact: true }).click();
  const dialog = page.getByRole('dialog');
  await expect(dialog.locator('#record-personnel_id')).toHaveCount(0);
  await dialog.locator('#record-date').fill('2026-11-08');
  await dialog.locator('#record-type').selectOption('medical');
  await dialog.locator('#record-reason').fill('Own request from the browser');
  await dialog.locator('#record-session_id').selectOption('1');
  await expect(dialog.locator('#record-session_id option[value="1"]')).toHaveCount(1);
  const value = await save(page, 'unavailability-requests', 'POST');
  expect(value.personnel_id).toBe(1);
  expect(value.session_id).toBe(1);
  expect(value.date).toBe('2026-11-02');
  const card = page.locator('.record-card').filter({ has: page.getByText(`#${value.id}`, { exact: true }) });
  await card.getByRole('button', { name: 'Edit', exact: true }).click();
  await dialog.locator('#record-reason').fill('Updated own request');
  await dialog.locator('#record-session_id').selectOption('');
  await dialog.locator('#record-date').fill('2026-11-08');
  await save(page, 'unavailability-requests', 'PUT');
  await page.getByRole('button', { name: `Delete request #${value.id}`, exact: true }).click();
  const removed = page.waitForResponse((response) => response.request().method() === 'DELETE');
  await dialog.getByRole('button', { name: 'Delete record', exact: true }).click();
  expect((await removed).status()).toBe(204);
  await expect(dialog).toBeHidden();
}
