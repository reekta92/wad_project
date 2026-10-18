import { expect } from '@playwright/test';
import { execFileSync } from 'node:child_process';
import { mkdtempSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

const xlsx = 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet';
const proof = resolve('../scripts/verify_download.py');

async function xmlValue(page, text) {
  return page.evaluate((source) => {
    const document = new DOMParser().parseFromString(source, 'application/xml');
    if (document.querySelector('parsererror')) throw new Error('Invalid XML');
    function read(node) {
      const kind = node.getAttribute('type');
      if (kind === 'object') return Object.fromEntries([...node.children].map((child) => [child.getAttribute('name'), read(child)]));
      if (kind === 'array') return [...node.children].map(read);
      if (kind === 'number') return Number(node.textContent);
      if (kind === 'boolean') return node.textContent === 'true';
      if (kind === 'null') return null;
      return node.getAttribute('encoding') === 'base64' ? new TextDecoder().decode(Uint8Array.from(atob(node.textContent), (c) => c.charCodeAt(0))) : node.textContent;
    }
    return read(document.documentElement);
  }, text);
}

export async function verifyRepresentations(page, request, token, administrator) {
  const headers = { Authorization: `Bearer ${token}` };
  const paths = administrator ? ['me', 'dashboard', 'reports/workload', 'reports/availability', 'audit-logs', 'departments', 'courses', 'classrooms', 'sessions', 'personnel', 'unavailability-requests', 'exams', 'invigilator-assignments', 'schedule'] : ['me', 'schedule'];
  for (const path of paths) {
    const json = await request.get(`/api/v1/${path}`, { headers });
    expect(json.status()).toBe(200);
    const expected = await json.json();
    for (const media of ['application/xml', 'text/plain', ...(Array.isArray(expected.data) ? ['application/atom+xml'] : [])]) {
      const response = await request.get(`/api/v1/${path}`, { headers: { ...headers, Accept: media } });
      expect(response.status(), `${path}: ${await response.text()}`).toBe(200);
      expect(response.headers()['content-type']).toContain(media);
      expect(response.headers().vary).toContain('Accept');
      if (media === 'application/xml') expect(await xmlValue(page, await response.text())).toEqual(expected);
      if (media === 'text/plain') expect(JSON.parse(await response.text())).toEqual(expected);
      if (media === 'application/atom+xml') {
        const feed = await page.evaluate((source) => {
          const document = new DOMParser().parseFromString(source, 'application/xml');
          if (document.querySelector('parsererror')) throw new Error('Invalid Atom');
          const metadata = document.getElementsByTagNameNS('urn:wad:pagination', 'pagination')[0];
          return { namespace: document.documentElement.namespaceURI, total: Number(metadata.getAttribute('total')), page: Number(metadata.getAttribute('page')), rows: [...document.getElementsByTagNameNS('http://www.w3.org/2005/Atom', 'entry')].map((row) => JSON.parse(row.getElementsByTagName('content')[0].textContent)) };
        }, await response.text());
        expect(feed).toEqual({ namespace: 'http://www.w3.org/2005/Atom', total: expected.total, page: expected.page, rows: expected.data });
      }
    }
  }
  if (!administrator) return;
  const input = { name: 'Öğrenci & <API> =2+2' };
  expect((await request.post('/api/v1/departments', { headers: { ...headers, Accept: 'text/html' }, data: input })).status()).toBe(406);
  expect((await (await request.get(`/api/v1/departments?name=${encodeURIComponent(input.name)}`, { headers })).json()).total).toBe(0);
  const created = await request.post('/api/v1/departments', { headers: { ...headers, Accept: 'application/xml' }, data: input });
  expect(created.status()).toBe(201);
  const record = await xmlValue(page, await created.text());
  try {
    expect(record.name).toBe(input.name);
    expect(created.headers().location).toBe(`/api/v1/departments/${record.id}`);
    expect((await request.get(created.headers().location, { headers: { ...headers, Accept: 'application/atom+xml' } })).status()).toBe(406);
    const updated = await request.put(created.headers().location, { headers: { ...headers, Accept: 'text/plain' }, data: { name: input.name + ' revised' } });
    expect(updated.status()).toBe(200);
    expect(JSON.parse(await updated.text())).toMatchObject({ id: record.id, name: input.name + ' revised' });
  } finally { expect((await request.delete(`/api/v1/departments/${record.id}`, { headers: { ...headers, Accept: 'text/html' } })).status()).toBe(204); }
}

async function verifyWorkbook(file, expected) {
  process.stdout.write(execFileSync('python3', [proof, 'xlsx', file], { input: JSON.stringify(expected), stdio: ['pipe', 'pipe', 'pipe'] }));
}

export async function verifyMemberDownloads(page, request, token) {
  const headers = { Authorization: `Bearer ${token}` };
  expect((await request.post('/api/v1/backups', { headers })).status()).toBe(403);
  expect((await request.get('/api/v1/schedule/export?page_size=1', { headers: { ...headers, Accept: xlsx } })).status()).toBe(400);
  const directory = mkdtempSync(join(tmpdir(), 'wad-member-export-'));
  try {
    const expected = await (await request.get('/api/v1/schedule?department_id=1', { headers })).json();
    const response = await request.get('/api/v1/schedule/export?department_id=1', { headers: { ...headers, Accept: xlsx } });
    expect(response.status()).toBe(200);
    expect(response.headers()['cache-control']).toBe('no-store');
    const file = join(directory, 'filtered.xlsx');
    writeFileSync(file, await response.body(), { mode: 0o600 });
    await verifyWorkbook(file, expected.data);
    await page.getByLabel('Course name or code', { exact: true }).fill('WAD301');
    await page.getByRole('button', { name: 'Apply filters', exact: true }).click();
    const ready = page.waitForEvent('download');
    await page.getByRole('button', { name: 'Download XLSX', exact: true }).click();
    const download = await ready;
    expect(download.suggestedFilename()).toBe('exam-schedule.xlsx');
    await download.saveAs(file);
    const filtered = await (await request.get('/api/v1/schedule?q=WAD301', { headers })).json();
    await verifyWorkbook(file, filtered.data);
  } finally { rmSync(directory, { recursive: true, force: true }); }
}

export async function verifyAdministratorDownloads(page, request, token) {
  const headers = { Authorization: `Bearer ${token}` };
  await request.put('/api/v1/schedule/publication', { headers, data: { published: false } });
  await page.getByRole('button', { name: 'Refresh', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Timetable unpublished', exact: true })).toBeVisible();
  const directory = mkdtempSync(join(tmpdir(), 'wad-admin-download-'));
  try {
    const ready = page.waitForEvent('download');
    await page.getByRole('button', { name: 'Export current XLSX', exact: true }).click();
    const workbook = await ready;
    const file = join(directory, 'current.xlsx');
    await workbook.saveAs(file);
    const expected = await (await request.get('/api/v1/exams', { headers })).json();
    expect(expected.total).toBe(3);
    // Current export uses the same sanitized fields as the public schedule, which is hidden now.
    expect((await (await request.get('/api/v1/schedule')).json()).total).toBe(0);
    const exported = await request.get('/api/v1/schedule/export?department_id=1', { headers: { ...headers, Accept: xlsx } });
    expect(exported.status()).toBe(200);
    // Republish to compare every workbook value through the shared published view.
    await request.put('/api/v1/schedule/publication', { headers, data: { published: true } });
    const published = await (await request.get('/api/v1/schedule', { headers })).json();
    await verifyWorkbook(file, published.data);
    await page.getByRole('button', { name: 'Download database backup', exact: true }).click();
    const dialog = page.getByRole('dialog');
    await expect(dialog.getByRole('button', { name: 'Cancel', exact: true })).toBeFocused();
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
    await expect(page.getByRole('button', { name: 'Download database backup', exact: true })).toBeFocused();
    await page.route('**/api/v1/backups', (route) => route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ error: { message: 'Backup temporarily unavailable.' } }) }), { times: 1 });
    await page.getByRole('button', { name: 'Download database backup', exact: true }).click();
    await dialog.getByRole('button', { name: 'Download private backup', exact: true }).click();
    await expect(dialog.getByRole('alert')).toContainText('temporarily unavailable');
    const backupReady = page.waitForEvent('download');
    const backupResponse = page.waitForResponse((r) => r.request().method() === 'POST' && new URL(r.url()).pathname === '/api/v1/backups');
    await dialog.getByRole('button', { name: 'Download private backup', exact: true }).click();
    const completed = await backupResponse;
    expect(completed.status(), completed.ok() ? 'Completed backup' : await completed.text()).toBe(200);
    const download = await backupReady;
    expect(download.suggestedFilename()).toBe('application.dump');
    const archive = join(directory, 'application.dump');
    await download.saveAs(archive);
    await expect(dialog).toBeHidden();
    if (process.env.CI) process.stdout.write(execFileSync('python3', [proof, 'backup', archive], { stdio: 'pipe' }));
  } finally { rmSync(directory, { recursive: true, force: true }); }
}
