import { defineConfig } from '@playwright/test';
import standard from './playwright.config.js';

export default defineConfig({
  ...standard,
  testIgnore: [],
  testMatch: '**/production.spec.js',
  use: { ...standard.use, baseURL: 'https://localhost:8443', ignoreHTTPSErrors: false },
  webServer: {
    command: 'python3 ../scripts/run.py serve',
    url: 'https://localhost:8443/api/v1/health/live',
    reuseExistingServer: false,
    timeout: 60000,
    ignoreHTTPSErrors: false,
  },
});
