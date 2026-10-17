import { test, expect } from '@playwright/test';
import { createHmac } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import { resolve } from 'node:path';

const root = resolve('..');
const accounts = JSON.parse(readFileSync(resolve(root, '.local/demo-accounts.json'))).accounts;
const byName = (name) => accounts.find((account) => account.username === name);
const usedCounters = new Map();

function otp(secret, counter) {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
  const bits = [...secret].map((char) => alphabet.indexOf(char).toString(2).padStart(5, '0')).join('');
  const key = Buffer.from(bits.match(/.{8}/g).map((byte) => parseInt(byte, 2)));
  const time = Buffer.alloc(8);
  time.writeBigUInt64BE(BigInt(counter));
  const digest = createHmac('sha1', key).update(time).digest();
  const offset = digest[digest.length - 1] & 15;
  return String((digest.readUInt32BE(offset) & 0x7fffffff) % 1000000).padStart(6, '0');
}

async function submitOTP(page, account) {
  let counter = Math.floor(Date.now() / 30000);
  if (usedCounters.get(account.username) === counter) {
    await new Promise((done) => setTimeout(done, 30200 - Date.now() % 30000));
    counter = Math.floor(Date.now() / 30000);
  }
  usedCounters.set(account.username, counter);
  await page.locator('#otp').fill(otp(account.totp_secret, counter));
  await page.locator('#kc-login').click();
}

async function passwordSteps(page, account) {
  await page.locator('#username:visible, #password:visible').first().waitFor();
  if (await page.locator('#username').isVisible()) {
    await page.locator('#username').fill(account.username);
    await page.locator('#kc-login').click();
  }
  await page.locator('#password').waitFor();
  await page.locator('#password').fill(account.password);
  await page.locator('#kc-login').click();
  await page.locator('#otp').waitFor();
}

async function startLogin(page, account) {
  await page.goto('/');
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await passwordSteps(page, account);
}

function captureAccess(page) {
  const state = {};
  page.on('request', (request) => {
    if (new URL(request.url()).pathname === '/api/v1/me') {
      const value = request.headers().authorization;
      if (value?.startsWith('Bearer ')) state.token = value.slice(7);
    }
  });
  return state;
}

async function assertProfile(page, account, label) {
  await expect(page.getByRole('heading', { name: label, exact: true })).toBeVisible();
  await expect(page.locator('#account')).toContainText(account.username);
}

async function signOut(page) {
  await page.getByRole('button', { name: 'Sign out', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Guest access', exact: true })).toBeVisible();
}

test('TOTP matches the RFC 6238 SHA-1 vector', () => {
  expect(otp('GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ', 1)).toBe('287082');
});

test('guest browsing survives an unavailable sign-in service', async ({ page, request }) => {
  await page.route('**/api/v1/auth/config', (route) => route.abort());
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Published schedule', exact: true })).toBeVisible();
  await expect(page.getByRole('alert')).toContainText('Sign-in is temporarily unavailable');
  expect((await request.get('/api/v1/me?role=administrator&personnel_id=1')).status()).toBe(401);
});

for (const [name, label, role, personnel] of [
  ['student.demo', 'Student member', 'member', null],
  ['invigilator.demo', 'Invigilator', 'member', 1],
]) {
  test(`${label} signs in with an identity fixed by the provider`, async ({ page, request }) => {
    const account = byName(name);
    const access = captureAccess(page);
    await startLogin(page, account);
    await submitOTP(page, account);
    await assertProfile(page, account, label);
    const response = await request.get('/api/v1/me?role=administrator&personnel_id=3', { headers: { Authorization: `Bearer ${access.token}` } });
    expect(response.status()).toBe(200);
    const profile = await response.json();
    expect(profile.role).toBe(role);
    expect(profile.personnel_id).toBe(personnel);
    expect(profile.permissions).toEqual(personnel ? ['invigilator'] : []);
    expect(response.headers()['cache-control']).toBe('no-store');
    const persisted = await page.evaluate(() => [...Object.values(localStorage), ...Object.values(sessionStorage)]);
    expect(persisted.some((value) => value.includes(access.token))).toBe(false);
    await signOut(page);
  });
}

test('administrator requires TOTP, refreshes, rejects disabled accounts, and ends the provider session', async ({ page, request }) => {
  const account = byName('admin.demo');
  const access = captureAccess(page);
  await startLogin(page, account);
  expect((await request.get('/api/v1/me')).status()).toBe(401);
  const counter = Math.floor(Date.now() / 30000);
  const valid = new Set([-1, 0, 1].map((offset) => otp(account.totp_secret, counter + offset)));
  let wrong = '000000';
  while (valid.has(wrong)) wrong = String(Number(wrong) + 1).padStart(6, '0');
  await page.locator('#otp').fill(wrong);
  const rejected = page.waitForResponse((response) => response.request().method() === 'POST' && response.url().includes('/login-actions/'));
  await page.locator('#kc-login').click();
  await rejected;
  await expect(page.locator('#otp')).toBeVisible();
  await submitOTP(page, account);
  await assertProfile(page, account, 'Administrator');
  const claims = JSON.parse(Buffer.from(access.token.split('.')[1], 'base64url'));
  expect(claims.amr).toEqual(expect.arrayContaining(['pwd', 'otp']));

  await page.clock.install();
  const refresh = page.waitForResponse((response) => response.url().endsWith('/protocol/openid-connect/token') && response.request().postData()?.includes('grant_type=refresh_token'));
  await page.clock.fastForward(310000);
  const refreshed = await refresh;
  expect(refreshed.status()).toBe(200);
  const tokens = await refreshed.json();
  expect((await request.get('/api/v1/me', { headers: { Authorization: `Bearer ${tokens.access_token}` } })).status()).toBe(200);

  if (!/^[a-f0-9-]{36}$/i.test(account.subject)) throw new Error('Unexpected provider subject format');
  const setActive = (active) => execFileSync('docker', ['compose', 'exec', '-T', 'postgres', 'psql', '-U', 'exam_owner', '-d', 'exams', '-v', 'ON_ERROR_STOP=1', '-c', `UPDATE app_users SET active = ${active} WHERE subject = '${account.subject}'`], { cwd: root, stdio: 'pipe' });
  try {
    setActive(false);
    expect((await request.get('/api/v1/me', { headers: { Authorization: `Bearer ${tokens.access_token}` } })).status()).toBe(403);
  } finally { setActive(true); }
  await signOut(page);
  // Access JWTs retain their short lifetime; logout revokes the refresh capability.
  expect((await request.get('/api/v1/me', { headers: { Authorization: `Bearer ${tokens.access_token}` } })).status()).toBe(200);
  const revoked = await request.post('http://localhost:8081/realms/exams/protocol/openid-connect/token', { form: { grant_type: 'refresh_token', client_id: 'exam-web', refresh_token: tokens.refresh_token } });
  expect(revoked.status()).toBe(400);
});

test('administrator enrolls and signs in with a user-verified passkey', async ({ page, request }) => {
  const account = byName('admin.demo');
  const access = captureAccess(page);
  const cdp = await page.context().newCDPSession(page);
  await cdp.send('WebAuthn.enable');
  await cdp.send('WebAuthn.addVirtualAuthenticator', { options: { protocol: 'ctap2', transport: 'internal', hasResidentKey: true, hasUserVerification: true, isUserVerified: true, automaticPresenceSimulation: true } });
  await startLogin(page, account);
  await submitOTP(page, account);
  await assertProfile(page, account, 'Administrator');
  await page.getByRole('button', { name: 'Register a passkey', exact: true }).click();
  await passwordSteps(page, account);
  await submitOTP(page, account);
  await page.locator('#registerWebAuthn').waitFor();
  page.on('dialog', (dialog) => dialog.accept('CI passkey'));
  await page.locator('#registerWebAuthn').click();
  await assertProfile(page, account, 'Administrator');
  await signOut(page);
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await page.locator('#username').fill(account.username);
  await page.locator('#kc-login').click();
  await page.locator('#authenticateWebAuthnButton').click();
  await assertProfile(page, account, 'Administrator');
  const claims = JSON.parse(Buffer.from(access.token.split('.')[1], 'base64url'));
  expect(claims.amr).toContain('passkey');
  expect((await request.get('/api/v1/me', { headers: { Authorization: `Bearer ${access.token}` } })).status()).toBe(200);
  await signOut(page);
});
