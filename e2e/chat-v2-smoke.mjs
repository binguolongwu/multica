import { chromium } from 'playwright';

async function main() {
  const browser = await chromium.launch({
    headless: true,
    args: ['--no-proxy-server'],
  });
  const page = await browser.newPage();

  const errors = [];
  page.on('console', msg => { if (msg.type() === 'error') errors.push(msg.text()); });

  await page.goto('http://127.0.0.1:3000/login', { waitUntil: 'domcontentloaded', timeout: 15000 });
  await page.waitForTimeout(2000);

  console.log('URL:', page.url());
  const hmrErrors = errors.filter(e => e.includes('webpack-hmr'));
  const otherErrors = errors.filter(e => !e.includes('webpack-hmr') && !e.includes('hmr'));
  console.log('HMR WebSocket errors:', hmrErrors.length, hmrErrors.length === 0 ? '✅' : '❌');
  console.log('Other errors:', otherErrors.length, otherErrors.length === 0 ? '✅' : '❌');
  if (otherErrors.length > 0) otherErrors.forEach(e => console.log(' -', e.substring(0, 200)));

  await browser.close();
}
main();
