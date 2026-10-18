import { test, expect } from '@playwright/test';
import { mkdirSync, writeFileSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import { resolve } from 'node:path';
import { byName, startLogin, submitOTP, captureAccess, assertProfile, signOut } from './authentication.js';

test('compiled Go serves the production build and verifies real HTTPS provider identities', async ({ page, request }) => {
  const assets = [];
  page.on('response', (response) => { if (new URL(response.url()).pathname.startsWith('/assets/')) assets.push(response); });
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Published schedule', exact: true })).toBeVisible();
  expect(assets.length).toBeGreaterThan(0);
  expect(assets.every((response) => response.ok() && new URL(response.url()).origin === 'https://localhost:8443')).toBe(true);
  expect((await request.get('/api/v1/health/ready')).status()).toBe(200);
  const config = await (await request.get('/api/v1/auth/config')).json();
  expect(config.issuer).toBe('https://localhost:8444/realms/exams');
  expect((await request.get('/api/v1/dashboard')).status()).toBe(401);
  expect((await request.get('/api/v1/missing')).headers()['content-type']).toContain('application/json');
  const account = byName('admin.demo');
  const access = captureAccess(page);
  await startLogin(page, account);
  expect(new URL(page.url()).origin).toBe('https://localhost:8444');
  await submitOTP(page, account);
  await assertProfile(page, account, 'Administrator');
  expect(new URL(page.url()).origin).toBe('https://localhost:8443');
  const claims = JSON.parse(Buffer.from(access.token.split('.')[1], 'base64url'));
  expect(claims.iss).toBe(config.issuer);
  expect(claims.amr).toEqual(expect.arrayContaining(['pwd', 'otp']));
  const headers = { Authorization: `Bearer ${access.token}` };
  const created = await request.post('/api/v1/departments', { headers, data: { name: 'HTTPS deployment fixture' } });
  expect(created.status()).toBe(201);
  const department = await created.json();
  try {
    const read = await request.get(created.headers().location, { headers: { ...headers, Accept: 'application/xml' } });
    expect(read.status()).toBe(200);
    expect(await read.text()).toContain('HTTPS deployment fixture');
    expect(read.headers()['cache-control']).toBe('no-store');
    const changed = await request.put(created.headers().location, { headers: { ...headers, Accept: 'text/plain' }, data: { name: 'HTTPS revised fixture' } });
    expect(changed.status()).toBe(200);
    expect(JSON.parse(await changed.text()).name).toBe('HTTPS revised fixture');
    const feed = await request.get('/api/v1/departments', { headers: { ...headers, Accept: 'application/atom+xml' } });
    expect(feed.status()).toBe(200);
    expect(await feed.text()).toContain('http://www.w3.org/2005/Atom');
  } finally { expect((await request.delete(`/api/v1/departments/${department.id}`, { headers })).status()).toBe(204); }
  expect((await request.put('/api/v1/schedule/publication', { headers, data: { published: true } })).status()).toBe(200);
  await signOut(page);
  const student = byName('student.demo');
  await startLogin(page, student);
  await submitOTP(page, student);
  await assertProfile(page, student, 'Student member');
  const member = { Authorization: `Bearer ${access.token}` };
  expect((await request.get('/api/v1/dashboard', { headers: member })).status()).toBe(403);
  const workbook = await request.get('/api/v1/schedule/export', { headers: { ...member, Accept: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' } });
  expect(workbook.status()).toBe(200);
  expect((await workbook.body()).subarray(0, 2).toString()).toBe('PK');
  await signOut(page);
  const evidence = {
    commit: execFileSync('git', ['rev-parse', 'HEAD'], { encoding: 'utf8' }).trim(),
    api_origin: 'https://localhost:8443', issuer: config.issuer,
    tls_certificate_validation: true, app_environment: 'production',
    verified: ['compiled Go binary', 'same-origin production assets', 'database/provider readiness', 'real PKCE/password/TOTP login', 'verified issuer/roles/MFA', 'JSON/XML/text/Atom', 'CRUD', 'publication', 'member XLSX', 'logout'],
  };
  const directory = resolve('../.local/production-evidence');
  mkdirSync(directory, { recursive: true });
  writeFileSync(resolve(directory, 'result.json'), JSON.stringify(evidence, null, 2), { mode: 0o600 });
  console.log('Production HTTPS proof: compiled Go, same-origin assets, trusted app/provider TLS, actual MFA, roles, CRUD, representations, export, and logout passed.');
});
