import { test, expect } from '@playwright/test';

// Public layout fixtures keep visual checks independent of operator edits and account secrets.
const exams = Array.from({ length: 12 }, (_, index) => ({
  id: index + 1,
  course_code: `${['MAT', 'CSE', 'PHY'][index % 3]}${101 + index}`,
  course_name: ['Calculus I', 'Introduction to Programming', 'Physics I'][index % 3],
  department: ['Mathematics', 'Computer Engineering', 'Physics'][index % 3],
  term: index % 3 + 1,
  date: '2026-11-02', start_time: '09:00:00', end_time: '11:00:00',
  rooms: [{ name: 'A-101' }],
}));

test.beforeEach(async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.route('**/api/v1/schedule*', (route) => {
    const params = new URL(route.request().url()).searchParams;
    const query = (params.get('q') || '').toLowerCase();
    const filtered = exams.filter((exam) => `${exam.course_code} ${exam.course_name}`.toLowerCase().includes(query));
    const page = Number(params.get('page') || 1);
    const page_size = Number(params.get('page_size') || 20);
    return route.fulfill({ json: { data: filtered.slice((page - 1) * page_size, page * page_size), total: filtered.length, page, page_size } });
  });
});

test('rounded workspace fits phone, tablet and desktop in both themes', async ({ page }, testInfo) => {
  await page.goto('/');
  await expect(page.getByRole('status')).toHaveText('12 exams found');
  await expect(page.getByRole('button', { name: 'Sign in', exact: true })).toBeEnabled();
  for (const theme of ['light', 'dark']) {
    await page.getByLabel('Color theme').selectOption(theme);
    for (const width of [320, 375, 390, 639, 640, 768, 1023, 1024, 1440]) {
      await page.setViewportSize({ width, height: 1000 });
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      const menu = page.getByRole('button', { name: 'Open menu', exact: true });
      if (width < 1024) await expect(menu).toBeVisible();
      else await expect(menu).toBeHidden();
      for (const control of [page.getByRole('button', { name: 'Sign in', exact: true }), page.getByRole('button', { name: 'Apply filters' }), page.getByLabel('Course name or code')]) {
        const box = await control.boundingBox();
        expect(box.height).toBeGreaterThanOrEqual(44);
        expect(box.x).toBeGreaterThanOrEqual(0);
        expect(box.x + box.width).toBeLessThanOrEqual(width);
        await expect(control).toHaveCSS('border-top-width', '0px');
      }
      if (width === 390 || width === 1440) {
        await page.evaluate(() => window.scrollTo(0, 0));
        await page.screenshot({ path: testInfo.outputPath(`guest-${width}-${theme}.jpg`), type: 'jpeg', quality: 45 });
      }
    }
  }
});

test('theme follows the device, persists overrides and keeps readable contrast', async ({ page }) => {
  await page.emulateMedia({ colorScheme: 'dark' });
  await page.goto('/');
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'system');
  await expect(page.locator('html')).toHaveCSS('background-color', 'rgb(32, 33, 36)');
  await page.emulateMedia({ colorScheme: 'light' });
  await expect(page.locator('html')).toHaveCSS('background-color', 'rgb(246, 248, 252)');
  for (const theme of ['light', 'dark']) {
    await page.getByLabel('Color theme').selectOption(theme);
    await page.reload();
    await expect(page.getByLabel('Color theme')).toHaveValue(theme);
    await expect(page.locator('html')).toHaveAttribute('data-theme', theme);
    await expect(page.getByRole('status')).toHaveText('12 exams found');
    const ratios = await page.evaluate(() => {
      const luminance = (color) => {
        const channels = color.match(/[\d.]+/g).slice(0, 3).map((value) => {
          const channel = Number(value) / 255;
          return channel <= .04045 ? channel / 12.92 : ((channel + .055) / 1.055) ** 2.4;
        });
        return channels[0] * .2126 + channels[1] * .7152 + channels[2] * .0722;
      };
      return ['.button.primary', '.course-code', '.publication-chip', '.nav-link.selected', '.content-surface', '.exam-card'].map((selector) => {
        const style = getComputedStyle(document.querySelector(selector));
        const values = [luminance(style.color), luminance(style.backgroundColor)].sort((a, b) => b - a);
        return (values[0] + .05) / (values[1] + .05);
      });
    });
    for (const ratio of ratios) expect(ratio).toBeGreaterThanOrEqual(4.5);
  }
  await page.getByLabel('Color theme').selectOption('system');
  await page.reload();
  await expect(page.getByLabel('Color theme')).toHaveValue('system');
});

test('mobile navigation, keyboard focus and informational dialog remain usable', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/');
  const menu = page.locator('.menu-toggle');
  await menu.click();
  await expect(menu).toHaveAttribute('aria-expanded', 'true');
  await page.getByRole('link', { name: 'Find an exam' }).focus();
  await page.keyboard.press('Escape');
  await expect(menu).toBeFocused();
  await expect(page.getByRole('navigation', { name: 'Main navigation' })).toBeHidden();
  await expect(menu).toHaveCSS('outline-style', 'solid');
  await menu.click();
  await page.getByRole('button', { name: 'Schedule information', exact: true }).click();
  const dialog = page.getByRole('dialog');
  await expect(dialog).toBeVisible();
  await expect(dialog.getByRole('button', { name: 'Got it' })).toBeFocused();
  await page.keyboard.press('Escape');
  await expect(dialog).toBeHidden();
  await menu.click();
  await page.getByRole('link', { name: 'Find an exam' }).click();
  await expect(page.getByLabel('Course name or code')).toBeFocused();
});

test('filters, paging, calendar, empty state and retry survive the redesign', async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 844 });
  await page.goto('/');
  await expect(page.getByRole('status')).toHaveText('12 exams found');
  await page.getByLabel('Per page').selectOption('10');
  await page.getByRole('button', { name: 'Apply filters' }).click();
  await expect(page.locator('.exam-grid article')).toHaveCount(10);
  await page.getByRole('button', { name: 'Next page', exact: true }).click();
  await expect(page.locator('.exam-grid article')).toHaveCount(2);
  await expect(page.getByRole('button', { name: 'Next page', exact: true })).toBeDisabled();
  await page.getByLabel('Course name or code').fill('MAT101');
  await page.getByLabel('From date').fill('2026-11-01');
  await page.getByLabel('To date').fill('2026-11-03');
  await page.getByRole('button', { name: 'Apply filters' }).click();
  await expect(page.getByRole('status')).toHaveText('1 exam found');
  const params = new URL(page.url()).searchParams;
  expect(Object.fromEntries(params)).toEqual({ page: '1', page_size: '10', q: 'MAT101', date_from: '2026-11-01', date_to: '2026-11-03' });
  await page.getByRole('button', { name: 'Calendar', exact: true }).click();
  await expect(page.getByRole('table')).toContainText('Calculus I');
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.getByLabel('Course name or code').fill('no-match');
  await page.getByRole('button', { name: 'Apply filters' }).click();
  await expect(page.getByRole('heading', { name: 'No published exams match these filters' })).toBeVisible();
  await page.route('**/api/v1/schedule*', (route) => route.fulfill({ status: 503, json: { error: { message: 'Please try again shortly.' } } }), { times: 1 });
  await page.getByLabel('Course name or code').fill('MAT101');
  await page.getByRole('button', { name: 'Apply filters' }).click();
  await expect(page.getByRole('alert')).toContainText('Please try again shortly.');
  await page.getByRole('button', { name: 'Try again', exact: true }).click();
  await expect(page.getByRole('table')).toContainText('Calculus I');
});
