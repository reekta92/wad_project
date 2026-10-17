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

async function navigate(page, label, management = false) {
  const menu = page.getByRole('button', { name: 'Open menu', exact: true });
  if (await menu.isVisible()) await menu.click();
  await page.getByRole('navigation', { name: 'Main navigation', exact: true }).getByRole('button', { name: label, exact: true }).click();
  await expect(page.locator(management ? '.management .results-toolbar' : '.workflow > p[role="status"]').first()).toContainText(management ? 'records found' : label === 'Dashboard' ? 'Overview updated.' : label === 'My workload' ? 'Workload updated.' : 'records found');
  await expect(page.locator('#page-title')).toBeFocused();
}

async function widths(page) {
  for (const width of [320, 768, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  }
  await page.setViewportSize({ width: 390, height: 844 });
}

export async function verifyOwnWorkflowUI(page) {
  await navigate(page, 'My duties');
  await expect(page.getByRole('cell', { name: 'WAD301 · Web Application Development', exact: true })).toBeVisible();
  await expect(page.getByRole('cell', { name: 'Lecture Hall', exact: true })).toBeVisible();
  await expect(page.getByLabel('Personnel ID', { exact: true })).toHaveCount(0);
  await widths(page);
  await page.getByLabel('Course name or code', { exact: true }).fill('no matching course');
  await page.getByRole('button', { name: 'Apply filters', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'No matching records', exact: true })).toBeVisible();
  await navigate(page, 'My workload');
  await expect(page.locator('.record-details')).toContainText('Assigned duties1');
  await page.getByLabel('From date', { exact: true }).fill('2026-11-03');
  const filtered = page.waitForResponse((r) => new URL(r.url()).pathname === '/api/v1/me/workload' && new URL(r.url()).searchParams.has('date_from'));
  await page.getByRole('button', { name: 'Apply filters', exact: true }).click();
  expect((await filtered).status()).toBe(200);
  await expect(page.locator('.record-details')).toContainText('Assigned duties0');
  await widths(page);
}

export async function verifyWorkflowUI(page, request, token) {
  const headers = { Authorization: `Bearer ${token}` };
  const publicCount = async (count) => expect(await (await request.get('/api/v1/schedule')).json()).toMatchObject({ total: count });
  const dashboardAction = async (path, method, action, status = 200) => {
    const response = page.waitForResponse((r) => r.request().method() === method && new URL(r.url()).pathname === `/api/v1/${path}`);
    const reload = status < 300 ? page.waitForResponse((r) => r.request().method() === 'GET' && new URL(r.url()).pathname === '/api/v1/dashboard') : null;
    await action();
    const value = await response;
    expect(value.status(), await value.text()).toBe(status);
    if (reload) { expect((await reload).status()).toBe(200); await expect(page.locator('.workflow > p[role="status"]').first()).toHaveText('Overview updated.'); }
    return status === 204 ? null : value.json();
  };
  await navigate(page, 'Dashboard');
  await expect(page.getByRole('heading', { name: 'Timetable published', exact: true })).toBeVisible();
  await widths(page);
  await page.getByRole('button', { name: 'Clear schedule…', exact: true }).click();
  const dialog = page.getByRole('dialog');
  await expect(dialog.getByRole('button', { name: 'Cancel', exact: true })).toBeFocused();
  await page.keyboard.press('Escape');
  await expect(dialog).toBeHidden();
  await expect(page.getByRole('button', { name: 'Clear schedule…', exact: true })).toBeFocused();
  await publicCount(3);
  await dashboardAction('schedule/publication', 'PUT', () => page.getByRole('button', { name: 'Unpublish timetable', exact: true }).click());
  await publicCount(0);
  await dashboardAction('schedule', 'DELETE', async () => {
    await page.getByRole('button', { name: 'Clear schedule…', exact: true }).click();
    await dialog.getByRole('button', { name: 'Clear all exams and duties', exact: true }).click();
  }, 204);
  await expect(page.getByRole('heading', { name: 'Unplanned courses (3)', exact: true })).toBeVisible();
  await page.getByLabel('Season', { exact: true }).selectOption('autumn');
  expect((await dashboardAction('schedule/automatic-planning', 'POST', () => page.getByRole('button', { name: 'Plan unplanned courses', exact: true }).click())).count).toBe(3);
  await dashboardAction('schedule/publication', 'PUT', () => page.getByRole('button', { name: 'Publish timetable', exact: true }).click(), 409);
  await expect(page.getByRole('alert')).toContainText('invigilator');
  await expect(page.getByLabel('Season', { exact: true })).toHaveValue('autumn');
  await page.getByLabel('Staffing scope', { exact: true }).selectOption('one');
  const select = page.getByLabel('Exam room allocation', { exact: true });
  await expect(select).toBeEnabled();
  const firstAllocation = await select.locator('option').nth(1).getAttribute('value');
  await select.selectOption(firstAllocation);
  expect((await dashboardAction('invigilator-assignments/automatic', 'POST', () => page.getByRole('button', { name: 'Staff selected rooms', exact: true }).click())).count).toBe(1);
  await page.getByLabel('Staffing scope', { exact: true }).selectOption('all');
  expect((await dashboardAction('invigilator-assignments/automatic', 'POST', () => page.getByRole('button', { name: 'Staff selected rooms', exact: true }).click())).count).toBe(2);
  expect((await dashboardAction('schedule/automatic-planning', 'POST', () => page.getByRole('button', { name: 'Plan unplanned courses', exact: true }).click())).count).toBe(0);
  await dashboardAction('schedule/publication', 'PUT', () => page.getByRole('button', { name: 'Publish timetable', exact: true }).click());
  await publicCount(3);

  await navigate(page, 'Workload');
  await expect(page.getByRole('table')).toContainText('Computer Engineering');
  await page.getByLabel('From date', { exact: true }).fill('2026-11-03');
  await page.getByRole('button', { name: 'Apply filters', exact: true }).click();
  await expect(page.getByRole('cell', { name: '0', exact: true })).toHaveCount(3);
  await widths(page);
  await navigate(page, 'Availability');
  await expect(page.getByRole('table')).toContainText('additional duty');
  await page.getByLabel('Personnel ID', { exact: true }).fill('1');
  await page.getByLabel('Session ID', { exact: true }).fill('1');
  await page.getByRole('button', { name: 'Apply filters', exact: true }).click();
  await expect(page.getByRole('cell', { name: 'No', exact: true })).toHaveCount(1);
  await widths(page);
  await navigate(page, 'Audit log');
  await page.getByLabel('Entity', { exact: true }).fill('exams');
  await page.getByLabel('Action', { exact: true }).fill('clear');
  await page.getByRole('button', { name: 'Apply filters', exact: true }).click();
  await expect(page.getByRole('row')).toHaveCount(3); // API and browser clear operations.
  await page.getByRole('button', { name: /View change #/ }).first().click();
  await expect(dialog.getByRole('heading', { name: 'Before', exact: true })).toBeVisible();
  await expect(dialog.getByLabel('Before values', { exact: true })).toContainText('exams');
  await expect(dialog.getByRole('button', { name: 'Close', exact: true })).toBeFocused();
  await widths(page);
  await page.keyboard.press('Escape');
  await expect(dialog).toBeHidden();
  await expect(page.getByRole('button', { name: /View change #/ }).first()).toBeFocused();

  const create = async (date, reason) => {
    const response = await request.post('/api/v1/unavailability-requests', { headers, data: { personnel_id: 1, date, session_id: null, type: 'leave', reason } });
    expect(response.status()).toBe(201);
    return response.json();
  };
  const conflicting = await create('2026-11-02', 'UI decision conflict');
  const allowed = await create('2026-11-04', 'UI decision approval');
  await navigate(page, 'Requests', true);
  const card = (id) => page.locator('.record-card').filter({ has: page.getByText(`#${id}`, { exact: true }) });
  await card(conflicting.id).getByRole('button', { name: 'Approve', exact: true }).click();
  await expect(dialog.getByRole('button', { name: 'Cancel', exact: true })).toBeFocused();
  await dialog.getByRole('button', { name: 'Approve request', exact: true }).click();
  await expect(dialog.getByRole('alert')).toContainText('unavailability');
  await expect(dialog).toContainText('UI decision conflict');
  await dialog.getByRole('button', { name: 'Cancel', exact: true }).click();
  await expect(card(conflicting.id)).toContainText('pending');
  for (const [id, verb, status] of [[conflicting.id, 'Reject', 'rejected'], [allowed.id, 'Approve', 'approved']]) {
    await card(id).getByRole('button', { name: verb, exact: true }).click();
    await dialog.getByRole('button', { name: `${verb} request`, exact: true }).click();
    await expect(dialog).toBeHidden();
    await expect(card(id)).toContainText(status);
    await expect(card(id).getByRole('button', { name: 'Edit', exact: true })).toBeDisabled();
    await expect(card(id).getByRole('button', { name: 'Approve', exact: true })).toHaveCount(0);
  }
  // Approval hides publication; leave the isolated fixture ready for later login checks.
  await navigate(page, 'Dashboard');
  await dashboardAction('schedule/publication', 'PUT', () => page.getByRole('button', { name: 'Publish timetable', exact: true }).click());
  await page.setViewportSize({ width: 1280, height: 720 });
}
