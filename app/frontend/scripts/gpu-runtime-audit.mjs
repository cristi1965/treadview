import fs from 'node:fs/promises';
import path from 'node:path';
import { chromium } from 'playwright';

if (process.env.GPU_RUNTIME_WRITES !== 'true') {
  throw new Error('Refusing GPU runtime writes. Set GPU_RUNTIME_WRITES=true only against the temporary GPU fixture.');
}

const baseURL = process.env.GPU_BASE_URL || 'http://127.0.0.1:8831';
const parsedBaseURL = new URL(baseURL);
if (parsedBaseURL.protocol !== 'http:' || !['127.0.0.1', '::1'].includes(parsedBaseURL.hostname)) {
  throw new Error(`Refusing GPU runtime writes against non-loopback URL: ${baseURL}`);
}
const device = process.env.GPU_VIEWPORT === 'mobile' ? 'mobile' : 'desktop';
const outputDir = path.resolve(process.cwd(), `../../output/playwright/gpu-runtime-${device}`);
await fs.mkdir(outputDir, { recursive: true });

const evidence = [];
const assert = (condition, claim, detail = '') => {
  evidence.push({ claim, pass: Boolean(condition), detail });
  if (!condition) throw new Error(`${claim}${detail ? `: ${detail}` : ''}`);
};
const sleep = (milliseconds) => new Promise((resolve) => setTimeout(resolve, milliseconds));

const browser = await chromium.launch({ headless: true });
const context = await browser.newContext(device === 'mobile'
  ? { viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true, deviceScaleFactor: 3 }
  : { viewport: { width: 1440, height: 1000 } });
const page = await context.newPage();
page.on('pageerror', (error) => evidence.push({ claim: 'no page errors', pass: false, detail: error.message }));
const activate = (locator) => device === 'mobile' ? locator.tap() : locator.click();

async function readJSON(url, options) {
  const response = await context.request.fetch(`${baseURL}${url}`, options);
  return { response, body: await response.json().catch(() => ({})) };
}

async function waitForLiveSnapshot() {
  let latest = {};
  for (let attempt = 0; attempt < 100; attempt += 1) {
    const result = await readJSON('/api/gpu-prices');
    latest = result.body;
    if (result.response.ok() && latest.status === 'live' && latest.count === 4) return latest;
    await sleep(100);
  }
  throw new Error(`GPU fixture did not publish four live quotes: ${JSON.stringify(latest)}`);
}

try {
  await page.goto(`${baseURL}/settings`, { waitUntil: 'domcontentloaded' });
  await page.getByRole('button', { name: /本机会话已启用/ }).waitFor({ timeout: 10_000 });

  const initial = await waitForLiveSnapshot();
	assert(initial.testOnly === true && initial.dataMode === 'acceptance-fixture', 'backend identifies the acceptance fixture as test-only');
  assert(initial.providers?.length === 4 && initial.providers.every((item) => item.status === 'live'), 'fixture service reports all four acceptance providers available');
  assert(initial.quotes?.every((item) => item.observedAt && item.sourceUrl?.startsWith('https://fixture.invalid/')), 'fixture quotes passed service validation and timestamping');

  const firstHistory = await readJSON('/api/gpu-prices/history?provider=runpod&gpuModel=H100%20SXM&days=1&limit=10');
	assert(firstHistory.response.ok() && firstHistory.body.testOnly === true && firstHistory.body.dataMode === 'acceptance-fixture' && firstHistory.body.count >= 1, 'scheduled fixture refresh persisted dated test-only history', `HTTP ${firstHistory.response.status()} count=${firstHistory.body.count}`);

  await page.goto(`${baseURL}/gpu-prices`, { waitUntil: 'domcontentloaded' });
	await page.getByRole('heading', { name: 'GPU 租金验收夹具' }).waitFor({ timeout: 10_000 });
  await page.getByText('H100 SXM').first().waitFor({ timeout: 10_000 });
	const fixtureBody = await page.locator('body').innerText();
	assert(fixtureBody.includes('验收夹具') && fixtureBody.includes('测试专用数据') && !fixtureBody.includes('已验证快照'), `${device} renders an explicit test-only fixture boundary`);

  await page.getByLabel('筛选厂商').selectOption('vast');
  assert(await page.getByText('Vast.ai').first().isVisible(), `${device} provider filter works`);
  await page.getByLabel('历史天数').selectOption('7');
	await page.getByText(/条验收夹具观测/).waitFor({ timeout: 10_000 });
  assert(true, `${device} reads persisted fixture history through the real router`);

  const reloadResponse = page.waitForResponse((response) => response.url().endsWith('/api/gpu-prices/reload-config') && response.request().method() === 'POST');
  await activate(page.getByRole('button', { name: '重读凭证' }));
  const reloaded = await reloadResponse;
	const reloadedBody = await reloaded.json().catch(() => ({}));
	assert(reloaded.status() === 200 && reloadedBody.testOnly === true && reloadedBody.dataMode === 'acceptance-fixture', `${device} admin credential reload preserves fixture provenance`, `HTTP ${reloaded.status()}`);
  await page.getByText(/已重新读取后端凭证/).waitFor({ timeout: 10_000 });

  const refreshButton = page.getByRole('button', { name: '刷新上游' });
  await page.waitForFunction(() => {
    const button = [...document.querySelectorAll('button')].find((item) => item.textContent?.includes('刷新上游'));
    return button && !button.disabled;
  }, undefined, { timeout: 10_000 });
  const refreshResponse = page.waitForResponse((response) => response.url().endsWith('/api/gpu-prices/refresh') && response.request().method() === 'POST');
  await activate(refreshButton);
  const refreshed = await refreshResponse;
	const refreshedBody = await refreshed.json().catch(() => ({}));
	assert(refreshed.status() === 200 && refreshedBody.snapshot?.testOnly === true && refreshedBody.snapshot?.dataMode === 'acceptance-fixture', `${device} admin fixture refresh preserves provenance through the real service`, `HTTP ${refreshed.status()}`);

  const finalSnapshot = await waitForLiveSnapshot();
  assert(Boolean(finalSnapshot.nextRefreshAt), 'scheduler publishes its next refresh time');
  const finalHistory = await readJSON('/api/gpu-prices/history?days=1&limit=20');
	assert(finalHistory.response.ok() && finalHistory.body.testOnly === true && finalHistory.body.count >= 4, 'history endpoint retains test-only observations from every fixture provider', `count=${finalHistory.body.count}`);

  if (device === 'mobile') {
    for (const button of [page.getByRole('button', { name: '重读凭证' }), refreshButton]) {
      const box = await button.boundingBox();
      assert(box && box.height >= 36, 'mobile admin action has a stable touch target', JSON.stringify(box));
    }
  }
  const horizontalOverflow = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth);
  assert(!horizontalOverflow, `${device} GPU page has no page-level horizontal overflow`);
  await page.screenshot({ path: path.join(outputDir, `gpu-runtime-${device}.png`), fullPage: true });
	if (evidence.some((item) => !item.pass)) throw new Error('GPU runtime acceptance recorded failed evidence');
} finally {
  await fs.writeFile(path.join(outputDir, 'audit.json'), `${JSON.stringify({ baseURL, checkedAt: new Date().toISOString(), evidence }, null, 2)}\n`);
  await context.close();
  await browser.close();
}

console.log(JSON.stringify({ outputDir, passed: evidence.filter((item) => item.pass).length, total: evidence.length }, null, 2));
