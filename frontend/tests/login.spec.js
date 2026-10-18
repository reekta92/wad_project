import { test, expect } from '@playwright/test';
import { execFileSync } from 'node:child_process';
import { resolve } from 'node:path';
import { readFileSync } from 'node:fs';
import { verifyIdentityTheme, verifyConsoleTheme } from './provider-theme.js';
import { verifyCollection } from './collection.js';
import { otp, byName, submitOTP, passwordSteps, startLogin, captureAccess, assertProfile, signOut } from './authentication.js';
import { verifyBusinessAPI, verifyMemberAPI, verifyBusinessUI, verifyOwnedRequestUI } from './business.js';
import { verifyRepresentations, verifyMemberDownloads, verifyAdministratorDownloads } from './downloads.js';
import { verifyWorkflowAPI, verifyWorkflowPermissions, verifyOwnWorkflowUI, verifyWorkflowUI } from './workflows.js';

const root = resolve('..');

test('identity, MFA, error, account and operator pages use the shared workspace theme', async ({ page }) => {
  const account = byName('student.demo');
  await page.goto('/');
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await expect(page.locator('#username')).toBeVisible();
  await verifyIdentityTheme(page);
  await page.locator('#username').fill(account.username);
  await page.locator('#kc-login').click();
  await expect(page.locator('#password')).toBeVisible();
  await verifyIdentityTheme(page);
  await page.locator('#password').fill(account.password);
  await page.locator('#kc-login').click();
  await expect(page.locator('#otp')).toBeVisible();
  await verifyIdentityTheme(page);
  await submitOTP(page, account);
  await assertProfile(page, account, 'Student member');
  await page.goto('http://localhost:8081/realms/exams/account/');
  await verifyConsoleTheme(page, 'account');
  await page.goto('http://localhost:8081/realms/exams/protocol/openid-connect/auth?client_id=missing-client');
  await expect(page.locator('#kc-error-message')).toBeVisible();
  await verifyIdentityTheme(page);

  await page.goto('http://localhost:8081/admin/master/console/');
  await expect(page.locator('#username')).toBeVisible();
  await verifyIdentityTheme(page);
  const password = readFileSync(resolve(root, '.env'), 'utf8').split('\n').find((line) => line.startsWith('KEYCLOAK_ADMIN_PASSWORD=')).slice('KEYCLOAK_ADMIN_PASSWORD='.length);
  await page.locator('#username').fill('local-admin');
  await page.locator('#password').fill(password);
  await page.locator('#kc-login').click();
  await verifyConsoleTheme(page, 'admin');
});

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
    await verifyRepresentations(page, request, access.token, false);
    await verifyMemberDownloads(page, request, access.token);
    await verifyMemberAPI(request, access.token, Boolean(personnel));
    await verifyWorkflowPermissions(request, access.token, Boolean(personnel));
    if (personnel) { await verifyOwnedRequestUI(page); await verifyOwnWorkflowUI(page); await verifyCollection(request, access.token, false); }
    expect(response.headers()['cache-control']).toBe('no-store');
    const persisted = await page.evaluate(() => [...Object.values(localStorage), ...Object.values(sessionStorage)]);
    expect(persisted.some((value) => value.includes(access.token))).toBe(false);
    await signOut(page);
  });
}

test('administrator requires TOTP, refreshes, rejects disabled accounts, and ends the provider session', async ({ page, request }) => {
  await page.clock.install();
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
  await verifyBusinessAPI(request, access.token);
  await verifyBusinessUI(page, request, access.token);
  await verifyWorkflowAPI(request, access.token);
  await verifyWorkflowUI(page, request, access.token);
  await verifyRepresentations(page, request, access.token, true);
  await verifyAdministratorDownloads(page, request, access.token);
  await verifyCollection(request, access.token, true);
  const claims = JSON.parse(Buffer.from(access.token.split('.')[1], 'base64url'));
  expect(claims.amr).toEqual(expect.arrayContaining(['pwd', 'otp']));

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
  await verifyIdentityTheme(page);
  page.on('dialog', (dialog) => dialog.accept('CI passkey'));
  await page.locator('#registerWebAuthn').click();
  await assertProfile(page, account, 'Administrator');
  await signOut(page);
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await page.locator('#username').fill(account.username);
  await page.locator('#kc-login').click();
  await expect(page.locator('#authenticateWebAuthnButton').or(page.locator('#password'))).toBeVisible();
  if (await page.locator('#password').isVisible()) {
    await page.getByRole('link', { name: /try another way/i }).click();
    await verifyIdentityTheme(page);
    await page.getByRole('button', { name: /passkey|security key|webauthn/i }).click();
  }
  await expect(page.locator('#authenticateWebAuthnButton')).toBeVisible();
  await verifyIdentityTheme(page);
  await page.locator('#authenticateWebAuthnButton').click();
  await assertProfile(page, account, 'Administrator');
  const claims = JSON.parse(Buffer.from(access.token.split('.')[1], 'base64url'));
  expect(claims.amr).toContain('passkey');
  expect((await request.get('/api/v1/me', { headers: { Authorization: `Bearer ${access.token}` } })).status()).toBe(200);
  await signOut(page);
});
