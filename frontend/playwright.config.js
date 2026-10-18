import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  testIgnore: '**/production.spec.js',
  fullyParallel: false,
  workers: 1,
  maxFailures: 1,
  timeout: 180000,
  expect: { timeout: 15000 },
  reporter: 'line',
  use: { baseURL: 'http://localhost:8080', browserName: 'chromium', trace: 'off', screenshot: 'off', video: 'off' },
  webServer: {
    command: 'python3 ../scripts/run.py server',
    url: 'http://localhost:8080/api/v1/health/live',
    reuseExistingServer: !process.env.CI,
    timeout: 60000,
  },
});
