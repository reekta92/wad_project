import { expect } from '@playwright/test';

export async function verifyIdentityTheme(page) {
  await expect(page.locator('link[href*="/common/exams/css/provider.css"]')).toHaveCount(1);
  const card = page.locator('.pf-v5-c-login__main');
  for (const theme of ['light', 'dark']) {
    await page.getByLabel('Color theme').selectOption(theme);
    await expect(page.locator('html')).toHaveAttribute('data-theme', theme);
    await expect(page.locator('body')).toHaveCSS('background-color', theme === 'dark' ? 'rgb(32, 33, 36)' : 'rgb(246, 248, 252)');
    await expect(card).toHaveCSS('background-color', theme === 'dark' ? 'rgb(41, 42, 45)' : 'rgb(255, 255, 255)');
    await expect(card).toHaveCSS('border-radius', '32px');
    await expect(card).toHaveCSS('border-top-width', '0px');
    for (const width of [320, 639, 640, 1024]) {
      await page.setViewportSize({ width, height: 900 });
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      for (const control of await page.locator('input:visible:not([type="hidden"]), button:visible, select:visible').all()) {
        expect((await control.boundingBox()).height).toBeGreaterThanOrEqual(44);
      }
    }
    const submit = page.locator('#kc-login:visible, #registerWebAuthn:visible, #authenticateWebAuthnButton:visible').first();
    if (await submit.count()) {
      await expect(submit).toHaveCSS('border-radius', '999px');
      await expect(submit).toHaveCSS('background-color', theme === 'dark' ? 'rgb(97, 175, 239)' : 'rgb(40, 92, 153)');
    }
  }
  await page.getByLabel('Color theme').selectOption('system');
  await page.setViewportSize({ width: 1280, height: 900 });
}

export async function verifyConsoleTheme(page, kind) {
  await expect(page.locator(`body[data-page-id="${kind}"]`)).toBeVisible();
  await expect(page.locator('.pf-v5-c-page')).toBeVisible();
  await expect(page.locator('link[href*="/common/exams/css/console.css"]')).toHaveCount(1);
  for (const scheme of ['dark', 'light']) {
    await page.emulateMedia({ colorScheme: scheme });
    await expect(page.locator('body')).toHaveCSS('background-color', scheme === 'dark' ? 'rgb(32, 33, 36)' : 'rgb(246, 248, 252)');
    await expect(page.locator('.pf-v5-c-page__main-section').first()).toHaveCSS('border-radius', '28px');
    for (const width of [375, 768, 1440]) {
      await page.setViewportSize({ width, height: 1000 });
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    }
  }
}
