import fs from 'node:fs/promises';
import path from 'node:path';
import { chromium } from 'playwright';

if (process.env.PAPER_LIFECYCLE_WRITES !== 'true') {
  throw new Error('Refusing Paper writes. Set PAPER_LIFECYCLE_WRITES=true against the temporary Paper runtime fixture.');
}

const baseURL = process.env.PAPER_BASE_URL || 'http://127.0.0.1:8831';
const device = process.env.PAPER_VIEWPORT === 'mobile' ? 'mobile' : 'desktop';
const outputDir = path.resolve(process.cwd(), `../../output/playwright/paper-lifecycle-${device}`);
const fixtureNow = Date.parse('2026-09-08T15:00:00Z');
await fs.mkdir(outputDir, { recursive: true });

const evidence = [];
const assert = (condition, claim, detail = '') => {
  evidence.push({ claim, pass: Boolean(condition), detail });
  if (!condition) throw new Error(`${claim}${detail ? `: ${detail}` : ''}`);
};

const browser = await chromium.launch({ headless: true });
const context = await browser.newContext(device === 'mobile'
  ? { viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true, deviceScaleFactor: 3 }
  : { viewport: { width: 1440, height: 1000 } });
const activate = (locator) => device === 'mobile' ? locator.tap() : locator.click();
await context.addInitScript((now) => {
  const started = performance.now();
  Date.now = () => Math.floor(now + performance.now() - started);
}, fixtureNow);
const page = await context.newPage();
page.on('pageerror', (error) => evidence.push({ claim: 'no page errors', pass: false, detail: error.message }));
page.on('console', (message) => {
  const text = message.text();
  const expectedHandled503 = text.includes('Failed to load resource: the server responded with a status of 503');
  const navigationCancelledStatus = text.startsWith('Failed to fetch system status: TypeError: Failed to fetch');
  if (message.type() === 'error' && !expectedHandled503 && !navigationCancelledStatus) {
    evidence.push({ claim: 'no unexpected console errors', pass: false, detail: text });
  }
});

const setFixtureQuote = async (price) => {
  const response = await page.request.post(`${baseURL}/api/_test/paper-runtime/quote`, {
    data: { symbol: 'NVDA', price },
  });
  assert(response.ok(), `fixture quote ${price}`, `HTTP ${response.status()}`);
};

const readJSON = async (route) => {
  const response = await context.request.get(`${baseURL}${route}`);
  const body = await response.json().catch(() => ({}));
  assert(response.ok(), `read ${route}`, `HTTP ${response.status()}`);
  return body;
};

const confirmSimulatedFill = async () => {
  await activate(page.getByRole('button', { name: '\u590d\u6838\u6a21\u62df\u6210\u4ea4' }));
  const confirm = page.getByRole('button', { name: '\u786e\u8ba4\u6807\u8bb0\u6210\u4ea4' });
  await confirm.waitFor({ timeout: 10_000 });
  await activate(confirm);
};

try {
  await page.goto(`${baseURL}/settings`, { waitUntil: 'domcontentloaded' });
  await page.getByRole('button', { name: /\u672c\u673a\u4f1a\u8bdd\u5df2\u542f\u7528/ }).waitFor({ timeout: 10_000 });

  // Prove the server is the explicit temporary fixture before any durable Paper write.
  await setFixtureQuote(100);
  await page.goto(`${baseURL}/tactical`, { waitUntil: 'domcontentloaded' });
  await activate(page.getByRole('button', { name: /NVDA \(\u82f1\u4f1f\u8fbe\)/ }));
  await page.locator('select').filter({ has: page.locator('option', { hasText: '\u9884\u8bbe\u56de\u64a4\u4e70\u5165' }) }).selectOption('BUY_LOW');
  const establishBaseline = page.getByRole('button', { name: /\u5efa\u7acb USD \u65e5\u521d\u57fa\u7ebf/ });
  await establishBaseline.waitFor({ timeout: 10_000 });
  await activate(establishBaseline);
  await page.locator('p').filter({ hasText: /USD .*\u65e5\u521d\u6743\u76ca 100000\.00/ }).waitFor({ timeout: 10_000 });
  assert(true, 'missing baseline is established through the UI with server-computed equity');
  const submit = page.getByRole('button', { name: '\u9a8c\u8bc1\u5e76\u63d0\u4ea4 Paper \u8ba2\u5355' });
  await submit.waitFor({ state: 'visible' });
  await page.waitForFunction(() => {
    const button = [...document.querySelectorAll('button')].find((item) => item.textContent?.includes('\u9a8c\u8bc1\u5e76\u63d0\u4ea4 Paper \u8ba2\u5355'));
    return button && !button.disabled;
  }, undefined, { timeout: 10_000 });
  const submitResponse = page.waitForResponse((response) => response.url().endsWith('/api/paper-orders') && response.request().method() === 'POST');
  await activate(submit);
  const submitted = await (await submitResponse).json();
  const parentClientOrderId = submitted.order?.clientOrderId;
  assert(Boolean(parentClientOrderId), 'submitted order exposes a stable clientOrderId');
  assert(!submitted.order?.researchRunId && !submitted.order?.researchTicker,
    'standalone Paper order does not invent research provenance', JSON.stringify(submitted.order));
  await page.getByRole('button', { name: '\u53d6\u6d88 Paper \u8ba2\u5355' }).waitFor({ timeout: 10_000 });
  assert((await page.locator('body').innerText()).includes('\u72b6\u6001 ACCEPTED'), 'BUY order accepted');

  await setFixtureQuote(98);
  const quantity = page.getByLabel('\u6a21\u62df\u6210\u4ea4\u6570\u91cf');
  await quantity.fill('2');
  await confirmSimulatedFill();
  await page.getByText(/\u7d2f\u8ba1\u6a21\u62df\u6210\u4ea4 2 \//).waitFor({ timeout: 10_000 });
  assert((await page.locator('body').innerText()).includes('#1 2 @'), 'first immutable partial fill visible');
  const firstLedger = await readJSON('/api/paper-orders');
  const firstParent = firstLedger.orders?.find((order) => order.clientOrderId === parentClientOrderId);
  assert(firstParent?.fills?.length === 1 && firstParent.fills[0].sequence === 1 && firstParent.fills[0].quantity === 2 && firstParent.fillQty === 2,
    'first partial fill matches the persisted immutable ledger', JSON.stringify(firstParent));

  const remaining = Number(await quantity.getAttribute('max'));
  await quantity.fill(String(remaining));
  await confirmSimulatedFill();
  await page.getByText(/\u72b6\u6001 SIMULATED_FILLED/).waitFor({ timeout: 10_000 });
  assert((await page.locator('body').innerText()).includes('#2 '), 'second immutable partial fill visible');
  const completedLedger = await readJSON('/api/paper-orders');
  const completedParent = completedLedger.orders?.find((order) => order.clientOrderId === parentClientOrderId);
  const completedFillQuantity = completedParent?.fills?.reduce((sum, fill) => sum + fill.quantity, 0);
  assert(completedParent?.status === 'SIMULATED_FILLED' && completedParent?.fills?.length === 2 && new Set(completedParent.fills.map((fill) => fill.fillId)).size === 2 && completedParent.fillQty === completedFillQuantity && completedParent.remainingQty === 0,
    'two persisted fills reconcile to the terminal parent order', JSON.stringify(completedParent));

  const stopRow = page.locator('tbody tr').filter({ hasText: 'auto-stop-' }).first();
  await stopRow.waitFor({ timeout: 10_000 });
  const stopClientOrderId = (await stopRow.locator('td').first().innerText()).trim();
  assert((await stopRow.innerText()).includes('ACCEPTED'), 'automatic protective OCO stop visible');
  await activate(stopRow.getByRole('button', { name: 'ACCEPTED' }));
  await setFixtureQuote(95);
  const stopQuantity = page.getByLabel('\u6a21\u62df\u6210\u4ea4\u6570\u91cf');
  await stopQuantity.fill('2');
  await confirmSimulatedFill();
  const stopRemaining = Number(await stopQuantity.getAttribute('max'));
  await stopQuantity.fill(String(stopRemaining));
  await confirmSimulatedFill();
  await page.getByText(/\u72b6\u6001 SIMULATED_FILLED/).waitFor({ timeout: 10_000 });
  const ocoLedger = await readJSON('/api/paper-orders');
  const filledStop = ocoLedger.orders?.find((order) => order.clientOrderId === stopClientOrderId);
  const cancelledSibling = ocoLedger.orders?.find((order) => order.ocoGroupId && order.ocoGroupId === filledStop?.ocoGroupId && order.clientOrderId !== stopClientOrderId);
  assert(filledStop?.status === 'SIMULATED_FILLED' && cancelledSibling?.status === 'CANCELLED' && filledStop?.parentOrderId === cancelledSibling?.parentOrderId,
    'OCO fill persists exactly one terminal fill and cancels its sibling', JSON.stringify({ filledStop, cancelledSibling }));
  const ocoBody = await page.locator('body').innerText();
  assert(ocoBody.includes('\u5df2\u5b8c\u6210 \u00b7 OCO \u4fdd\u62a4'), 'OCO lifecycle step complete for the selected parent/child chain');

  await setFixtureQuote(100);
  await activate(page.getByRole('button', { name: /513100 \(\u7eb3\u6307ETF\)/ }));
  await activate(page.getByRole('button', { name: /NVDA \(\u82f1\u4f1f\u8fbe\)/ }));
  await page.waitForFunction(() => {
    const button = [...document.querySelectorAll('button')].find((item) => item.textContent?.includes('\u9a8c\u8bc1\u5e76\u63d0\u4ea4 Paper \u8ba2\u5355'));
    return button && !button.disabled;
  }, undefined, { timeout: 10_000 });
  await activate(submit);
  await activate(page.getByRole('button', { name: '\u53d6\u6d88 Paper \u8ba2\u5355' }));
  await page.getByText(/\u72b6\u6001 CANCELLED/).waitFor({ timeout: 10_000 });
  assert((await page.locator('body').innerText()).includes('\u72b6\u6001 CANCELLED'), 'unfilled order cancellation visible');

  const body = await page.locator('body').innerText();
  const usdRisk = page.locator('div').filter({ hasText: /^USD \u00b7 \u603b\u6743\u76ca/ }).last();
  const usdRiskText = await usdRisk.innerText();
  const portfolio = await readJSON('/api/paper-orders/portfolio-risk');
  const usdPortfolio = portfolio.groups?.find((group) => group.currency === 'USD');
  assert(Number.isFinite(usdPortfolio?.dailyPnL) && usdRiskText.includes(`\u65e5PnL ${usdPortfolio.dailyPnL.toFixed(2)}`), 'USD portfolio daily PnL matches the server ledger', usdRiskText);
  assert(body.includes('\u5ba1\u8ba1\u94fe \u6709\u6548 \u00b7 pending 0'), 'audit chain valid with no pending outcomes');
  const auditVerification = await readJSON('/api/admin/audit/verify');
  assert(auditVerification.valid === true && auditVerification.pendingCount === 0, 'audit API verifies the persisted Paper ledger with no pending outcomes', JSON.stringify(auditVerification));
  const journalLink = page.getByRole('link', { name: '\u8bb0\u5f55\u5230\u4ea4\u6613\u590d\u76d8' });
  const journalHref = await journalLink.getAttribute('href');
  assert(Boolean(journalHref?.includes('paper_order_id=') && !journalHref.includes('research_run_id=')),
    'terminal standalone Paper order exposes a truthful Journal handoff', journalHref || '');
  const handoffOrderID = new URL(journalHref, baseURL).searchParams.get('paper_order_id');
  await activate(journalLink);
  await page.waitForURL(/\/journal\?/, { timeout: 10_000 });
  const journalBody = await page.locator('body').innerText();
  assert(journalBody.includes('\u6765\u6e90 Paper \u8ba2\u5355') && !journalBody.includes('\u6765\u6e90\u7814\u7a76 Run'),
    'Journal displays Paper provenance without inventing a research run');
  const journalForm = page.locator('form');
  await journalForm.locator('input[type="number"]').nth(0).fill('100');
  await journalForm.locator('input[type="number"]').nth(1).fill('101');
  await journalForm.locator('input[type="number"]').nth(2).fill('1');
  const tradeResponse = page.waitForResponse((response) => response.url().endsWith('/api/trades') && response.request().method() === 'POST');
  await activate(journalForm.locator('button[type="submit"]'));
  const savedTrade = await (await tradeResponse).json();
  assert(savedTrade.paperOrderId === handoffOrderID && !savedTrade.researchRunId,
    'Journal persists standalone Paper provenance through the API', JSON.stringify(savedTrade));
  const horizontalOverflow = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth);
  assert(!horizontalOverflow, `${device} has no page-level horizontal overflow`);

  await page.screenshot({ path: path.join(outputDir, `paper-lifecycle-${device}.png`), fullPage: true });
} finally {
  await fs.writeFile(path.join(outputDir, 'audit.json'), `${JSON.stringify({ baseURL, checkedAt: new Date(fixtureNow).toISOString(), evidence }, null, 2)}\n`);
  await context.close();
  await browser.close();
}

const failures = evidence.filter((item) => !item.pass);
if (failures.length > 0) throw new Error(`Paper browser acceptance recorded ${failures.length} failure(s): ${JSON.stringify(failures)}`);
console.log(JSON.stringify({ outputDir, passed: evidence.filter((item) => item.pass).length, total: evidence.length }, null, 2));
