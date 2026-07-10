import { chromium } from 'playwright';

const BASE = 'http://127.0.0.1:3000';

async function main() {
  console.log('=== Chat V2 Full Flow Test ===\n');

  const browser = await chromium.launch({
    headless: true,
    args: ['--no-proxy-server'],
  });

  const context = await browser.newContext({
    ignoreHTTPSErrors: true,
  });

  const page = await context.newPage();

  const errors = [];
  page.on('console', msg => {
    if (msg.type() === 'error') {
      // Skip HMR WebSocket errors (known issue with WSL2)
      if (msg.text().includes('webpack-hmr') || msg.text().includes('hmr')) return;
      errors.push(msg.text());
    }
  });

  // ─── 1. Load login page ───
  console.log('1. Loading login page...');
  await page.goto(`${BASE}/login`, { waitUntil: 'domcontentloaded', timeout: 15000 });
  await page.waitForTimeout(2000);
  console.log(`   URL: ${page.url()}`);
  const loginTitle = await page.title();
  console.log(`   Title: ${loginTitle}`);

  // Check for the auth form
  const emailInput = page.locator('input[type="email"], input[name="email"], input[placeholder*="email"], input[placeholder*="邮箱"]');
  const hasEmailInput = await emailInput.count() > 0;
  console.log(`   Email input present: ${hasEmailInput ? '✅' : '❌'}`);

  // ─── 2. Navigate to the chat page ───
  // First let's see if we can reach the chat page (may redirect to login without session)
  console.log('\n2. Navigating to chat page...');
  await page.goto(`${BASE}/binguosoft/chat`, { waitUntil: 'domcontentloaded', timeout: 15000 });
  await page.waitForTimeout(3000);
  console.log(`   URL: ${page.url()}`);
  console.log(`   Title: ${await page.title()}`);

  // ─── 3. Check for JavaScript errors ───
  console.log('\n3. Checking for JavaScript errors...');
  const appErrors = errors.filter(e =>
    !e.includes('webpack-hmr') &&
    !e.includes('hmr') &&
    !e.includes('favicon') &&
    !e.includes('ResizeObserver') // benign
  );
  console.log(`   App errors: ${appErrors.length} ${appErrors.length === 0 ? '✅' : '❌'}`);
  if (appErrors.length > 0) {
    console.log('   Error details:');
    appErrors.forEach(e => console.log(`   - ${e.substring(0, 300)}`));
  }

  // ─── 4. Take screenshot of what we see ───
  await page.screenshot({ path: '/tmp/chat-v2-login-page.png', fullPage: false });
  console.log('\n4. Screenshot saved to /tmp/chat-v2-login-page.png');

  // ─── 5. Check for the sidebar Chat nav link ───
  console.log('\n5. Checking for sidebar Chat navigation...');
  const chatNavLink = page.locator('nav a[href*="chat"], [role="navigation"] a[href*="chat"], button:has-text("Chat"), button:has-text("聊天")');
  const chatNavCount = await chatNavLink.count();
  console.log(`   Chat nav links found: ${chatNavCount}`);

  // Also check for the sidebar structure
  const sidebar = page.locator('nav, [role="navigation"], aside');
  const sidebarCount = await sidebar.count();
  console.log(`   Sidebar elements found: ${sidebarCount}`);
  if (sidebarCount > 0) {
    const sidebarText = await sidebar.first().textContent();
    console.log(`   Sidebar text excerpt: ${sidebarText.substring(0, 200)}`);
  }

  // ─── 6. Check for the floating chat FAB on dashboard ───
  console.log('\n6. Checking for FloatingChat on dashboard...');
  try {
    await page.goto(`${BASE}/binguosoft/issues`, { waitUntil: 'domcontentloaded', timeout: 15000 });
    await page.waitForTimeout(3000);
    console.log(`   Dashboard URL: ${page.url()}`);

    // Look for the floating chat FAB button
    const fabButton = page.locator('button').filter({ hasText: /MessageSquare|Chat|聊天|message/i });
    const fabCount = await fabButton.count();
    console.log(`   Potential chat FAB buttons: ${fabCount}`);
  } catch (e) {
    console.log(`   Dashboard navigation failed (expected if not logged in): ${e.message.substring(0, 100)}`);
  }

  // ─── 7. Summary ───
  console.log('\n=== Summary ===');
  console.log(`Login page loads: ${loginTitle !== '' ? '✅' : '❌'}`);
  console.log(`Email input present: ${hasEmailInput ? '✅' : '❌'}`);
  console.log(`Chat page accessible: ${page.url().includes('chat') ? '✅' : '⚠️ (may redirect to login)'}`);
  console.log(`JavaScript runtime errors: ${appErrors.length === 0 ? '✅' : '❌'} (${appErrors.length})`);

  // Overall pass/fail
  const passed = loginTitle !== '' && hasEmailInput && appErrors.length === 0;
  console.log(`\n${passed ? '✅ Overall: PASS' : '❌ Overall: FAIL (see details above)'}`);

  await browser.close();
}

main().catch(err => {
  console.error('Test failed with exception:', err.message);
  process.exit(1);
});
