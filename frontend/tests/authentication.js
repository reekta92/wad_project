import { expect } from '@playwright/test';
import { createHmac } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const root = resolve('..');
const accounts = JSON.parse(readFileSync(resolve(root, '.local/demo-accounts.json'))).accounts;
export const byName = (name) => accounts.find((account) => account.username === name);
const usedCounters = new Map();

export function otp(secret, counter) {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
  const bits = [...secret].map((char) => alphabet.indexOf(char).toString(2).padStart(5, '0')).join('');
  const key = Buffer.from(bits.match(/.{8}/g).map((byte) => parseInt(byte, 2)));
  const time = Buffer.alloc(8);
  time.writeBigUInt64BE(BigInt(counter));
  const digest = createHmac('sha1', key).update(time).digest();
  const offset = digest[digest.length - 1] & 15;
  return String((digest.readUInt32BE(offset) & 0x7fffffff) % 1000000).padStart(6, '0');
}

export async function submitOTP(page, account) {
  let counter = Math.floor(Date.now() / 30000);
  if (usedCounters.get(account.username) === counter) {
    await new Promise((done) => setTimeout(done, 30200 - Date.now() % 30000));
    counter = Math.floor(Date.now() / 30000);
  }
  usedCounters.set(account.username, counter);
  await page.locator('#otp').fill(otp(account.totp_secret, counter));
  await page.locator('#kc-login').click();
}

export async function passwordSteps(page, account) {
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

export async function startLogin(page, account) {
  await page.goto('/');
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await passwordSteps(page, account);
}

export function captureAccess(page) {
  const state = {};
  page.on('request', (request) => {
    if (new URL(request.url()).pathname === '/api/v1/me') {
      const value = request.headers().authorization;
      if (value?.startsWith('Bearer ')) state.token = value.slice(7);
    }
  });
  return state;
}

export async function assertProfile(page, account, label) {
  await expect(page.getByRole('heading', { name: label, exact: true })).toBeVisible();
  await expect(page.locator('#account')).toContainText(account.username);
}

export async function signOut(page) {
  await page.getByRole('button', { name: 'Sign out', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Guest access', exact: true })).toBeVisible();
}

