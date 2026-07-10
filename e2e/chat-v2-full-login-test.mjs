import { chromium } from 'playwright';

const BASE = 'http://127.0.0.1:3000';
const DEV_CODE = '123456';

async function sleep(ms) { return new Promise(r => setTimeout(r, ms)); }

async function main() {
  console.log('=== Chat V2 完整流程测试 ===\n');

  const browser = await chromium.launch({
    headless: true,
    args: ['--no-proxy-server', '--disable-font-subpixel-positioning'],
  });

  const context = await browser.newContext();
  const page = await context.newPage();

  const realErrors = [];
  page.on('console', msg => {
    if (msg.type() === 'error') {
      const t = msg.text();
      if (t.includes('webpack-hmr') || t.includes('hmr')) return;
      if (t.includes('favicon')) return;
      if (t.includes('ResizeObserver')) return;
      realErrors.push(t);
    }
  });

  let passed = 0, failed = 0;
  function check(desc, ok) {
    if (ok) { passed++; console.log(`  ${desc}: ✅`); }
    else { failed++; console.log(`  ${desc}: ❌`); }
  }

  async function screenshotSafe(path) {
    try { await page.screenshot({ path, fullPage: true, timeout: 5000 }); console.log(`  截图: ${path}`); }
    catch(e) { console.log(`  截图跳过: ${e.message.substring(0, 50)}`); }
  }

  // ─── STEP 1: Load login page ───
  console.log('── 第1步：加载登录页面 ──');
  await page.goto(`${BASE}/login`, { waitUntil: 'networkidle', timeout: 30000 });
  await sleep(2000);

  // ─── STEP 2: Login ───
  console.log('\n── 第2步：登录 ──');
  const emailInput = page.locator('input[type="email"]').first();
  check('邮箱输入框存在', await emailInput.count() > 0);

  if (await emailInput.count() > 0) {
    await emailInput.fill('80081608@qq.com');
    await sleep(300);

    const sendBtn = page.locator('button[type="submit"]').first();
    check('发送按钮存在', await sendBtn.count() > 0);
    await sendBtn.click();
    await sleep(3000);

    // Enter verification code - one digit at a time via keyboard
    console.log('  输入验证码...');
    const codeInput = page.locator('input[maxlength="6"], input[placeholder*="code"], input[placeholder*="Code"], input[placeholder*="验证码"]').first();
    if (await codeInput.count() > 0) {
      await codeInput.click();
      await sleep(200);
      // Type digit by digit
      await page.keyboard.type(DEV_CODE, { delay: 100 });
      await sleep(500);

      // Check if we're already on dashboard or need Enter
      if (page.url().includes('/login') || page.url().includes('/auth')) {
        await page.keyboard.press('Enter');
        await sleep(5000);
      }
    }
  }

  console.log(`  登录后 URL: ${page.url()}`);
  const isLoggedIn = page.url().includes('/binguosoft') && !page.url().includes('/login');
  check('登录成功', isLoggedIn);

  await screenshotSafe('/tmp/chat-v2-02-logged-in.png');

  // ─── STEP 3: Navigate to Chat page via sidebar ───
  console.log('\n── 第3步：通过侧边栏导航到 Chat ──');

  // If not already on a workspace page, navigate to one first
  if (!page.url().includes('/binguosoft')) {
    await page.goto(`${BASE}/binguosoft/issues`, { waitUntil: 'domcontentloaded', timeout: 15000 });
    await sleep(2000);
  }

  // Try clicking Chat in sidebar
  const chatLinks = page.locator('a[href*="chat"]');
  const chatLinkCount = await chatLinks.count();
  console.log(`  侧边栏 Chat 链接数: ${chatLinkCount}`);

  if (chatLinkCount > 0) {
    await chatLinks.first().click();
    await sleep(3000);
    console.log(`  点击侧边栏 Chat 后 URL: ${page.url()}`);
  } else {
    // Navigate directly
    await page.goto(`${BASE}/binguosoft/chat`, { waitUntil: 'domcontentloaded', timeout: 15000 });
    await sleep(3000);
    console.log(`  直接导航 Chat 后 URL: ${page.url()}`);
  }

  const onChatPage = page.url().includes('/chat');
  check('Chat 页面可访问', onChatPage);

  // ─── STEP 4: Check Chat V2 page content ───
  console.log('\n── 第4步：检查 Chat V2 页面内容 ──');
  const pageText = await page.locator('body').textContent();

  check('无 "Module not found"', !pageText.includes('Module not found'));
  check('无 "Internal Server Error"', !pageText.includes('Internal Server Error'));
  check('无 "not a function"', !pageText.includes('not a function'));

  // Check for key Chat V2 elements
  console.log(`  页面文本长度: ${pageText.length}`);
  console.log(`  页面文本片段 (前300字): ${pageText.substring(0, 300).replace(/\s+/g, ' ')}`);

  // Look for specific elements
  const hasChatText = pageText.includes('Chat') || pageText.includes('聊天');
  const hasNewButton = pageText.includes('新对话') || pageText.includes('New');
  const hasConversation = pageText.includes('对话') || pageText.includes('conversation');
  console.log(`  Chat 内容: ${hasChatText ? '✅' : '⚠️'}`);
  console.log(`  新对话按钮: ${hasNewButton ? '✅' : '⚠️'}`);
  console.log(`  对话内容: ${hasConversation ? '✅' : '⚠️'}`);

  await screenshotSafe('/tmp/chat-v2-03-chat-page.png');

  // ─── STEP 5: Check for dual-pane layout ───
  console.log('\n── 第5步：检查双栏布局 ──');
  // Check for the w-80 sidebar panel (thread list)
  const w80Elements = page.locator('[class*="w-80"]');
  const flexElements = page.locator('[class*="flex-1"]');
  const threadElements = page.locator('[class*="thread"], [class*="session"]');
  console.log(`  w-80 元素: ${await w80Elements.count()}`);
  console.log(`  flex-1 元素: ${await flexElements.count()}`);
  console.log(`  thread/session 元素: ${await threadElements.count()}`);

  // ─── STEP 6: Check API calls (from console) ──
  console.log('\n── 第6步：检查 API 调用 ──');
  // The page should have made API calls - check for any 401s that shouldn't be there
  const authErrors = realErrors.filter(e => e.includes('401') || e.includes('unauthorized'));
  const otherErrors = realErrors.filter(e => !e.includes('401') && !e.includes('unauthorized') && !e.includes('auth'));
  console.log(`  401 认证错误: ${authErrors.length} ${authErrors.length === 0 ? '✅' : '⚠️ (登录后不应有401)'}`);
  console.log(`  其他错误: ${otherErrors.length} ${otherErrors.length === 0 ? '✅' : '❌'}`);

  if (authErrors.length > 0) {
    console.log('  401 错误详情:');
    authErrors.forEach(e => console.log(`    - ${e.substring(0, 200)}`));
  }
  if (otherErrors.length > 0) {
    console.log('  其他错误详情:');
    otherErrors.forEach(e => console.log(`    - ${e.substring(0, 200)}`));
  }

  // ─── STEP 7: Test Floating Chat FAB on Issues page ──
  console.log('\n── 第7步：测试 Issues 页面的浮动 Chat ──');
  await page.goto(`${BASE}/binguosoft/issues`, { waitUntil: 'domcontentloaded', timeout: 15000 });
  await sleep(3000);
  console.log(`  Issues 页面 URL: ${page.url()}`);

  // Check for the FloatingChat component (should be a FAB or chat bubble in the corner)
  const fabElements = page.locator('[class*="fab"], [class*="float"], [class*="FloatingChat"], button[class*="fixed"][class*="bottom"]');
  console.log(`  浮动 Chat 元素: ${await fabElements.count()}`);

  // Check page content for errors
  const issuesPageText = await page.locator('body').textContent();
  check('Issues 页面无错误', !issuesPageText.includes('Module not found') && !issuesPageText.includes('not a function'));

  await screenshotSafe('/tmp/chat-v2-04-issues-with-floating-chat.png');

  // ─── STEP 8: Test other routes ──
  console.log('\n── 第8步：测试其他路由 ──');
  const routes = ['/binguosoft/inbox', '/binguosoft/agents', '/binguosoft/settings'];
  for (const path of routes) {
    try {
      await page.goto(`${BASE}${path}`, { waitUntil: 'domcontentloaded', timeout: 15000 });
      await sleep(2000);
      const body = await page.locator('body').textContent();
      const name = path.split('/').pop();
      check(name, !body.includes('Module not found') && !body.includes('not a function') && !body.includes('Internal Server Error'));
    } catch (e) {
      console.log(`  ${path}: ⚠️ ${e.message.substring(0, 60)}`);
      failed++;
    }
  }

  // ─── Summary ──
  console.log('\n=== 测试总结 ===');
  console.log(`通过: ${passed}`);
  console.log(`失败: ${failed}`);
  console.log(`真实 JS 错误: ${otherErrors.length}`);
  console.log(`401 错误(登录后): ${authErrors.length}`);

  const overall = failed === 0 && otherErrors.length === 0;
  console.log(`\n${overall ? '✅ 所有测试通过!' : '❌ 存在问题需要修复'}`);

  if (!overall) {
    console.log('\n── 问题清单 ──');
    if (failed > 0) console.log(`  - ${failed} 个检查失败`);
    if (otherErrors.length > 0) {
      console.log(`  - ${otherErrors.length} 个 JS 错误:`);
      otherErrors.forEach(e => console.log(`    ${e.substring(0, 300)}`));
    }
    if (authErrors.length > 0) console.log(`  - ${authErrors.length} 个登录后 401 错误`);
  }

  await browser.close();
  return overall;
}

main().then(p => process.exit(p ? 0 : 1)).catch(err => {
  console.error('异常:', err.message);
  process.exit(1);
});
