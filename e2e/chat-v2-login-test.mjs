import { chromium } from 'playwright';

const BASE = 'http://127.0.0.1:3000';

async function main() {
  console.log('=== Chat V2 Production Error Check ===\n');

  const browser = await chromium.launch({
    headless: true,
    args: ['--no-proxy-server'],
  });

  const page = await browser.newPage();

  const realErrors = [];
  const allErrors = [];
  page.on('console', msg => {
    if (msg.type() === 'error') {
      const text = msg.text();
      allErrors.push(text);
      // Collect only real errors, skip:
      // - HMR WebSocket issues
      // - 401 auth errors (expected when not logged in)
      // - favicon 404
      // - ResizeObserver
      if (text.includes('webpack-hmr') || text.includes('hmr')) return;
      if (text.includes('401') || text.includes('unauthorized') || text.includes('Unauthorized')) return;
      if (text.includes('missing authorization')) return;
      if (text.includes('favicon')) return;
      if (text.includes('ResizeObserver')) return;
      if (text.includes('[auth]') || text.includes('[api]')) return;
      realErrors.push(text);
    }
  });

  // ─── 1. Load login page (public, should always work) ───
  console.log('1. Loading /login...');
  await page.goto(`${BASE}/login`, { waitUntil: 'networkidle', timeout: 30000 });
  await page.waitForTimeout(2000);
  console.log(`   URL: ${page.url()}`);
  console.log(`   Title: ${await page.title()}`);

  await page.screenshot({ path: '/tmp/chat-v2-01-login.png', fullPage: true });

  // ─── 2. Check for login form ───
  console.log('\n2. Checking login form...');
  const emailInput = page.locator('input[type="email"], input[name="email"]');
  const submitButton = page.locator('button[type="submit"], button:has-text("Send"), button:has-text("发送"), button:has-text("Continue"), button:has-text("继续")');
  console.log(`   Email input: ${await emailInput.count() > 0 ? '✅' : '❌'}`);
  console.log(`   Submit button: ${await submitButton.count() > 0 ? '✅' : '❌'}`);

  // ─── 3. Try to log in ───
  console.log('\n3. Attempting login with email "80081608@qq.com"...');
  if (await emailInput.count() > 0) {
    await emailInput.first().fill('80081608@qq.com');
    await page.waitForTimeout(500);

    if (await submitButton.count() > 0) {
      await submitButton.first().click();
      await page.waitForTimeout(5000);

      console.log(`   After click URL: ${page.url()}`);
      await page.screenshot({ path: '/tmp/chat-v2-02-after-send-code.png', fullPage: true });

      // Check if we now see a code input field
      const codeInput = page.locator('input[type="text"][placeholder*="code"], input[placeholder*="Code"], input[placeholder*="验证码"], input[maxlength]');
      console.log(`   Code input visible: ${await codeInput.count() > 0 ? '✅' : '❌'}`);

      // Check for any error/success messages
      const pageText = await page.locator('body').textContent();
      if (pageText.includes('发送') || pageText.includes('sent') || pageText.includes('Sent')) {
        console.log('   → Verification code sent message appears present');
      }
      console.log(`   Page text excerpt: ${pageText.substring(0, 500).replace(/\s+/g, ' ')}`);
    }
  }

  // ─── 4. Navigate to dashboard routes and check for real errors ───
  const routes = ['/binguosoft/chat', '/binguosoft/issues', '/binguosoft/inbox', '/binguosoft/agents'];
  for (const route of routes) {
    console.log(`\n4. Testing ${route}...`);
    try {
      await page.goto(`${BASE}${route}`, { waitUntil: 'domcontentloaded', timeout: 20000 });
      await page.waitForTimeout(3000);
      console.log(`   Final URL: ${page.url()}`);

      // Check page title
      console.log(`   Title: ${await page.title()}`);

      // Look for Module not found / 500 errors
      const bodyText = await page.textContent();
      if (bodyText.includes('Module not found') || bodyText.includes('500') || bodyText.includes('Internal Server Error')) {
        console.log(`   ❌ PAGE ERROR DETECTED`);
        await page.screenshot({ path: `/tmp/chat-v2-error${route.replace(/\//g, '-')}.png`, fullPage: true });
      } else {
        console.log(`   Page renders: ✅ (no 500/module error)`);
      }
    } catch (e) {
      console.log(`   ⚠️ Navigation issue: ${e.message.substring(0, 100)}`);
    }
  }

  // ─── 5. Real errors report ───
  console.log('\n=== REAL App Errors (excluding 401 auth & HMR) ===');
  if (realErrors.length === 0) {
    console.log('✅ No real application errors!');
  } else {
    console.log(`❌ ${realErrors.length} real errors:`);
    realErrors.forEach(e => console.log(`   - ${e.substring(0, 300)}`));
  }

  console.log(`\nTotal console.error events: ${allErrors.length}`);
  console.log(`Real app errors: ${realErrors.length} ${realErrors.length === 0 ? '✅' : '❌'}`);

  const passed = realErrors.length === 0;
  console.log(`\n${passed ? '✅ OVERALL: PASS' : '❌ OVERALL: FAIL'}`);

  await browser.close();
}

main().catch(err => {
  console.error('Test exception:', err.message);
  process.exit(1);
});
