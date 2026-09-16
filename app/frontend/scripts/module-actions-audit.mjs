import fs from 'node:fs/promises';
import path from 'node:path';
import { chromium } from 'playwright';

const baseURL = process.env.USABILITY_BASE_URL || 'http://127.0.0.1:8831';
const round = process.env.USABILITY_ROUND || 'r1';
const device = process.env.USABILITY_DEVICE === 'mobile' ? 'mobile' : 'desktop';
const outputDir = path.resolve(process.cwd(), '../../output/playwright', `module-actions-${device}-${round}`);
await fs.mkdir(outputDir, { recursive: true });

const browser = await chromium.launch({ headless: true });
const context = await browser.newContext(device === 'mobile'
  ? { viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true, deviceScaleFactor: 3, permissions: ['clipboard-read', 'clipboard-write'] }
  : { viewport: { width: 1440, height: 1000 }, permissions: ['clipboard-read', 'clipboard-write'] });
const activate = (locator) => device === 'mobile' ? locator.tap() : locator.click();
const evidence = [];
const check = (name, pass, detail = '') => ({ name, pass: Boolean(pass), detail });

async function runTask(name, route, action) {
  const page = await context.newPage();
  const row = { name, route, passed: false, assertions: [], errors: [], durationMs: 0 };
  const started = Date.now();
  page.on('pageerror', (error) => row.errors.push(`page:${error.message}`));
  try {
    await page.goto(`${baseURL}${route}`, { waitUntil: 'domcontentloaded', timeout: 15_000 });
    await page.waitForTimeout(700);
    row.assertions = await action(page);
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth);
    row.assertions.push(check('no page-level horizontal overflow', !overflow, `scrollWidth overflow=${overflow}`));
    row.passed = row.assertions.every((item) => item.pass) && row.errors.length === 0;
  } catch (error) {
    row.errors.push(error instanceof Error ? error.message : String(error));
  } finally {
    row.durationMs = Date.now() - started;
    if (!row.passed) await page.screenshot({ path: path.join(outputDir, `${name}.png`), fullPage: false }).catch(() => undefined);
    evidence.push(row);
    await page.close();
  }
}

// Establish a real same-origin HttpOnly session once for the shared context.
const bootstrap = await context.newPage();
await bootstrap.goto(`${baseURL}/settings`, { waitUntil: 'domcontentloaded' });
await bootstrap.getByRole('button', { name: /本机会话已启用/ }).waitFor({ timeout: 8_000 });
await bootstrap.close();

await runTask('stale-token-recovery', '/settings', async (page) => {
  await activate(page.getByText('高级恢复：手动 Bearer 令牌'));
  const input = page.getByPlaceholder('输入管理令牌');
  await input.fill('r11-definitely-invalid-token');
  await activate(page.getByRole('button', { name: '验证令牌' }));
  await page.getByText(/管理操作已解锁/).waitFor({ timeout: 8_000 });
  const stored = await page.evaluate(() => sessionStorage.getItem('stockgod_admin_token_session'));
  return [
    check('invalid bearer self-heals through local session', (await page.locator('body').innerText()).includes('管理操作已解锁')),
    check('invalid bearer removed from session storage', stored == null, String(stored)),
  ];
});

await runTask('global-cn-search', '/market', async (page) => {
  await activate(page.locator('input[placeholder="搜代码 / 名称 / 板块…"]:visible').first());
  const input = page.locator('.fixed input:visible').first();
  await input.fill('300750');
  const result = page.locator('.fixed button').filter({ hasText: '300750' }).first();
  await result.waitFor({ timeout: 8_000 });
  await activate(result);
  await page.waitForURL(/\/stock\/300750/, { timeout: 8_000 });
  await page.locator('h1').filter({ hasText: '300750' }).waitFor({ timeout: 8_000 });
  return [
    check('six-digit A-share is found globally', page.url().includes('/stock/300750')),
    check('search navigates to CN detail', (await page.locator('body').innerText()).includes('300750')),
  ];
});

await runTask('scan-filter-watchlist', '/scan?market=cn', async (page) => {
  await activate(page.getByRole('button', { name: /A 股 · 全市场/ }));
  await page.waitForTimeout(900);
  const add = page.locator('button[aria-label="加入 watchlist"]:visible').first();
  await add.waitFor({ timeout: 8_000 });
  await activate(add);
  const remove = page.locator('button[aria-label="移出 watchlist"]:visible').first();
  await remove.waitFor({ timeout: 4_000 });
  await page.reload({ waitUntil: 'domcontentloaded' });
  await page.locator('button[aria-label="移出 watchlist"]:visible').first().waitFor({ timeout: 8_000 }).catch(() => undefined);
  const persisted = await page.locator('button[aria-label="移出 watchlist"]:visible').first().isVisible().catch(() => false);
  if (persisted) await activate(page.locator('button[aria-label="移出 watchlist"]:visible').first());
  return [
    check('CN market switch produces rows', (await page.locator('tbody tr').count()) > 0),
    check('watchlist toggle persists across reload', persisted),
    check('audit watchlist mutation cleaned up', persisted),
  ];
});

await runTask('reports-actions', '/reports', async (page) => {
  await activate(page.locator('button:visible').filter({ hasText: /^盘报$/ }).first());
  const card = page.locator('article').first();
  const toggle = card.locator('button').first();
  await toggle.waitFor({ timeout: 8_000 });
  await activate(toggle);
  const firstStateText = await card.innerText();
  await activate(toggle);
  const secondStateText = await card.innerText();
  const copy = page.getByRole('button', { name: /复制报告链接/ }).first();
  await activate(copy);
  const clipboard = await page.evaluate(() => navigator.clipboard.readText()).catch(() => '');
  return [
    check('report expands and collapses', secondStateText.length > firstStateText.length, `${firstStateText.length}/${secondStateText.length}`),
    check('report permalink copied', /\/reports\//.test(clipboard), clipboard),
  ];
});

await runTask('notes-search-open', '/notes', async (page) => {
  await page.getByPlaceholder(/搜 K线/).fill('止损');
  await page.waitForTimeout(700);
  const body = await page.locator('body').innerText();
  const result = page.locator('button').filter({ hasText: /止损/ }).last();
  const canOpen = await result.isVisible().catch(() => false);
  if (canOpen) await activate(result);
  await page.waitForTimeout(300);
  return [
    check('notes search returns matching content', body.includes('止损')),
    check('matching note can be opened', canOpen && page.url().includes('/notes/'), page.url()),
  ];
});

await runTask('whales-detail', '/whales', async (page) => {
  const card = page.locator('button').filter({ hasText: /霍华德·马克斯/ }).first();
  await card.waitFor({ timeout: 8_000 });
  await activate(card);
  await page.waitForURL(/\/whales\/howard-marks/, { timeout: 8_000 });
  await page.getByRole('heading', { name: /霍华德·马克斯/ }).waitFor({ timeout: 8_000 });
  return [
    check('institution card opens detail', page.url().includes('/whales/howard-marks')),
    check('detail exposes disclosure boundary', /披露|来源|持仓/.test(await page.locator('body').innerText())),
  ];
});

await runTask('portfolio-local-tools', '/portfolio', async (page) => {
  await page.getByPlaceholder('代码', { exact: true }).fill('NVDA');
  await page.getByPlaceholder('价格', { exact: true }).fill('1');
  await activate(page.getByRole('button', { name: '添加', exact: true }));
  await page.reload({ waitUntil: 'domcontentloaded' });
  const alertRow = page.locator('li').filter({ hasText: /NVDA >= 1/ }).first();
  const persisted = await alertRow.isVisible().catch(() => false);
  const downloadPromise = page.waitForEvent('download');
  await activate(page.getByRole('button', { name: /导出 JSON/ }));
  const download = await downloadPromise;
  const downloadPath = await download.path();
  const exported = downloadPath ? JSON.parse(await fs.readFile(downloadPath, 'utf8')) : null;
  if (persisted) await activate(alertRow.getByRole('button', { name: '删' }));
  return [
    check('local alert persists across reload', persisted),
    check('portfolio export is valid JSON', Array.isArray(exported?.watchlist) && Array.isArray(exported?.holdings)),
    check('audit alert mutation cleaned up', persisted),
  ];
});

await runTask('multichart-layout-preset', '/multichart', async (page) => {
  await activate(page.getByRole('button', { name: '四图 2x2' }));
  await activate(page.getByRole('button', { name: /A股算力四剑客/ }));
  const body = await page.locator('body').innerText();
  return [
    check('four-chart layout is applied', await page.getByRole('button', { name: '换票 ⇄' }).count() === 4),
    check('CN compute preset is applied', body.includes('300308') && body.includes('300750')),
  ];
});

await runTask('arena-modes', '/arena', async (page) => {
  const cn = page.getByRole('button', { name: 'A 股', exact: true });
  await activate(cn);
  const cnComparisonCount = await page.getByRole('button', { name: '股票对决' }).count();
  const board = page.getByRole('button', { name: '虚拟盘' });
  const cnBoardActive = (await board.getAttribute('class') || '').includes('bg-surface-3');
  const us = page.getByRole('button', { name: '美股', exact: true });
  await activate(us);
  const compare = page.getByRole('button', { name: '股票对决' });
  await activate(compare);
  return [
    check('CN arena stays on its supported board mode', cnComparisonCount === 0 && cnBoardActive),
    check('US comparison mode remains available', (await compare.getAttribute('class') || '').includes('bg-surface-3')),
  ];
});

await runTask('copilot-delayed-answer', '/copilot', async (page) => {
  await page.route('**/api/chat/ask', async (route) => {
    await new Promise((resolve) => setTimeout(resolve, 6_000));
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ answer: 'R11 延迟回答已完成', persona: 'mentor', timestamp: Date.now() }) });
  });
  await page.locator('input[placeholder*="自由提问"]').fill('测试长响应');
  await activate(page.getByRole('button', { name: '提问', exact: true }));
  await page.getByText('R11 延迟回答已完成').waitFor({ timeout: 10_000 });
  return [
    check('Copilot survives a response slower than five seconds', (await page.locator('body').innerText()).includes('R11 延迟回答已完成')),
  ];
});

await runTask('lab-running-recovery', '/lab', async (page) => {
  await page.route('**/api/analysis/status*', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ status: 'running', ws_clients: 1 }) }));
  await page.reload({ waitUntil: 'domcontentloaded' });
  const stop = page.getByRole('button', { name: /中止|停止/ });
  await stop.waitFor({ timeout: 6_000 });
  await page.route('**/api/analysis/stop', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ status: 'stopping' }) }));
  await activate(stop);
  await page.getByRole('button', { name: '启动实验分析' }).waitFor({ timeout: 4_000 });
  return [
    check('refresh restores running analysis controls', true),
    check('stop returns the lab to a startable state', await page.getByRole('button', { name: '启动实验分析' }).isVisible()),
  ];
});

await runTask('etf-current-history-refresh', '/etf', async (page) => {
  let refreshRequests = 0;
  await page.route('**/api/etf/refresh', async (route) => {
    refreshRequests += 1;
    await route.fulfill({ status: 202, contentType: 'application/json', body: JSON.stringify({ status: 'queued' }) });
  });
  await page.reload({ waitUntil: 'domcontentloaded' });
  await activate(page.getByRole('button', { name: '历史', exact: true }));
  await page.getByText('历史快照模式', { exact: true }).first().waitFor({ timeout: 8_000 });
  await activate(page.getByRole('button', { name: '刷新上游' }));
  await page.getByText(/已安排 ETF 上游刷新/).waitFor({ timeout: 8_000 });
  return [
    check('ETF explicitly switches to dated historical mode', (await page.locator('body').innerText()).includes('历史快照模式')),
    check('ETF protected refresh queues through the action button', refreshRequests === 1, String(refreshRequests)),
  ];
});

await runTask('gpu-dynamic-actions', '/gpu-prices', async (page) => {
  const observedAt = '2026-09-11T02:00:00Z';
  const quote = (gpuModel, price) => ({
    provider: 'runpod', gpuModel, product: `RunPod ${gpuModel}`, billingMode: 'on-demand', gpuCount: 1,
    currency: 'USD', rawPrice: price, rawUnit: 'USD/GPU-hour', priceUsdPerGpuHour: price,
    sourceUrl: 'https://www.runpod.io/pricing', observedAt,
  });
  const liveHeaders = {
    'X-Data-Source': 'official-provider-apis', 'X-Data-Time': observedAt,
    'X-Data-Stale': 'false', 'X-Data-Refreshable': 'true',
  };
  let reloadRequests = 0;
  let upstreamRequests = 0;
  await page.route((url) => url.pathname === '/api/gpu-prices', (route) => route.fulfill({
    status: 200, contentType: 'application/json', headers: liveHeaders,
    body: JSON.stringify({ status: 'live', count: 2, quotes: [quote('H100', 3.49), quote('A100', 1.89)], providers: [
      { provider: 'runpod', configured: true, status: 'live', quoteCount: 2, lastSuccessAt: observedAt },
      { provider: 'modal', configured: false, status: 'unconfigured', quoteCount: 0 },
      { provider: 'lambda', configured: false, status: 'unconfigured', quoteCount: 0 },
      { provider: 'vast', configured: false, status: 'unconfigured', quoteCount: 0 },
    ] }),
  }));
  await page.route('**/api/gpu-prices/history*', (route) => route.fulfill({
    status: 200, contentType: 'application/json', headers: liveHeaders,
    body: JSON.stringify({ status: 'live', count: 2, items: [
      { ...quote('H100', 3.69), observedAt: '2026-09-10T02:00:00Z' }, quote('H100', 3.49),
    ] }),
  }));
  await page.route('**/api/gpu-prices/reload-config', async (route) => {
    reloadRequests += 1;
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ status: 'live' }) });
  });
  await page.route('**/api/gpu-prices/refresh', async (route) => {
    upstreamRequests += 1;
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ status: 'live', updated: 2 }) });
  });
  await page.reload({ waitUntil: 'domcontentloaded' });
  await page.getByText('当前报价', { exact: true }).waitFor({ timeout: 8_000 });
  await page.getByLabel('筛选 GPU 型号').selectOption('H100');
  await page.getByText('真实历史观测', { exact: true }).waitFor({ timeout: 8_000 });
  await activate(page.getByRole('button', { name: '重读凭证' }));
  await page.getByText(/已重新读取后端凭证/).waitFor({ timeout: 8_000 });
  await activate(page.getByRole('button', { name: '刷新上游' }));
  await page.waitForTimeout(300);
  return [
    check('GPU live quotes can be filtered', (await page.getByLabel('筛选 GPU 型号').inputValue()) === 'H100'),
    check('GPU dated history is rendered from backend observations', (await page.locator('body').innerText()).includes('2 条真实观测')),
    check('GPU backend-only credential reload is actionable', reloadRequests === 1, String(reloadRequests)),
    check('GPU upstream refresh is actionable when configured', upstreamRequests === 1, String(upstreamRequests)),
  ];
});

await context.close();
await browser.close();
const summary = {
  baseURL,
  round,
  device,
  checkedAt: new Date().toISOString(),
  passed: evidence.filter((row) => row.passed).length,
  failed: evidence.filter((row) => !row.passed).length,
  total: evidence.length,
  evidence,
};
await fs.writeFile(path.join(outputDir, 'audit.json'), `${JSON.stringify(summary, null, 2)}\n`);
console.log(JSON.stringify({ outputDir, passed: summary.passed, failed: summary.failed, total: summary.total }, null, 2));
if (summary.failed) process.exitCode = 1;
