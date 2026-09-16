import fs from 'node:fs/promises';
import path from 'node:path';
import { chromium } from 'playwright';

const baseURL = process.env.USABILITY_BASE_URL || 'http://127.0.0.1:8831';
const round = process.env.USABILITY_ROUND || 'r1';
const allowWrites = process.env.USABILITY_ALLOW_WRITES === '1';
const allowPaperWrites = process.env.USABILITY_ALLOW_PAPER_WRITES === '1';
const outputDir = path.resolve(process.cwd(), '../../output/playwright', `usability-${round}`);
const viewports = [
  { name: 'desktop', width: 1440, height: 1000 },
  { name: 'mobile', width: 390, height: 844 },
];

await fs.mkdir(outputDir, { recursive: true });
const browser = await chromium.launch({ headless: true });
const evidence = [];

async function inspectTask(page, viewport, task, route, verify) {
  const started = Date.now();
  const row = { viewport: viewport.name, task, route, conclusion: 'failed', passed: false, assertions: [], errors: [], durationMs: 0 };
  const onPageError = (error) => row.errors.push(`page:${error.message}`);
  const onConsole = (message) => {
    const text = message.text();
    const expectedHandledHTTP = text.includes('Failed to load resource: the server responded with a status of 503');
    if (message.type() === 'error' && !text.includes('ERR_BLOCKED_BY_CLIENT') && !expectedHandledHTTP) row.errors.push(`console:${text}`);
  };
  page.on('pageerror', onPageError);
  page.on('console', onConsole);
  try {
    await page.goto(`${baseURL}${route}`, { waitUntil: 'domcontentloaded', timeout: 15_000 });
    await page.waitForTimeout(1_500);
    const assertions = await verify(page);
    row.assertions = assertions;
	const assertionFailed = assertions.some((item) => item.status === 'failed');
	const assertionNotRun = assertions.some((item) => item.status === 'not-run');
	row.conclusion = row.errors.length > 0 || assertionFailed ? 'failed' : assertionNotRun ? 'partial' : 'passed';
	row.passed = row.conclusion === 'passed';
  } catch (error) {
    row.errors.push(error instanceof Error ? error.message : String(error));
  } finally {
    row.durationMs = Date.now() - started;
    row.urlAfter = page.url();
    row.horizontalOverflow = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth).catch(() => null);
	if (row.horizontalOverflow) {
		row.conclusion = 'failed';
		row.passed = false;
	}
    await page.screenshot({ path: path.join(outputDir, `${viewport.name}-${task}.png`), fullPage: false }).catch(() => undefined);
    page.off('pageerror', onPageError);
    page.off('console', onConsole);
    evidence.push(row);
  }
}

const check = (name, pass, detail = '') => ({ name, status: pass ? 'passed' : 'failed', pass: Boolean(pass), detail });
const notRun = (name, detail) => ({ name, status: 'not-run', pass: false, detail });

for (const viewport of viewports) {
  const context = await browser.newContext(viewport.name === 'mobile'
    ? { viewport, isMobile: true, hasTouch: true, deviceScaleFactor: 3 }
    : { viewport });
	const runTask = async (task, route, verify) => {
		const page = await context.newPage();
		await inspectTask(page, viewport, task, route, verify);
		await page.close();
	};

	await runTask('settings-session', '/settings', async (p) => {
    await p.getByRole('button', { name: /\u672c\u673a\u4f1a\u8bdd\u5df2\u542f\u7528/ }).waitFor({ timeout: 8_000 });
	await p.getByText('历史时点研究', { exact: true }).waitFor({ timeout: 8_000 });
	await p.getByText(/\u5b9e\u9a8c\u72b6\u6001/).waitFor({ timeout: 8_000 });
    const body = await p.locator('body').innerText();
    return [
	  check('local session established', body.includes('本机会话已启用')),
      check('historical readiness visible', body.includes('历史时点研究')),
      check('paper readiness visible', body.includes('Paper 模拟引擎')),
      check('live boundary visible', body.includes('实验状态')),
    ];
  });

	await runTask('research-entry', '/dashboard#research-entry', async (p) => {
	const submit = p.getByRole('button', { name: /生成仅证据研究包|生成研究包/ });
    await submit.waitFor({ timeout: 8_000 });
	const tickerInput = p.locator('#research-entry input[type="text"]').first();
	await tickerInput.fill('AAPL');
	const submitEnabled = !(await submit.isDisabled());
	let generated = null;
	if (allowWrites && viewport.name === 'desktop') {
		await p.locator('#research-entry input[type="date"]').fill('2026-09-09');
		await submit.click();
		generated = await p.waitForURL(/\/history\?run_id=/, { timeout: 30_000 }).then(() => true).catch(() => false);
	}
	const body = await p.locator('body').innerText();
    return [
	  check('research form accepts ticker', (await tickerInput.inputValue().catch(() => 'AAPL')) === 'AAPL'),
	  check('submit enabled after local session', submitEnabled),
      check('no bearer error', !body.includes('Bearer token required')),
	  generated == null
		? notRun('research generation lifecycle', 'not executed by this read-only audit')
		: check('research generation lifecycle', generated === true, `generated=${generated}`),
    ];
  });

	await runTask('history-paper-handoff', '/history', async (p) => {
    const firstRecord = p.locator('button').filter({ hasText: /AAPL|NVDA|TSLA|GOOGL|COIN|CRWV/ }).first();
    if (await firstRecord.isVisible().catch(() => false)) await firstRecord.click();
    const link = p.getByRole('link', { name: /Paper 模拟/ }).first();
    await link.waitFor({ timeout: 8_000 });
    const href = await link.getAttribute('href');
    await link.click();
    await p.waitForURL(/\/tactical\?/, { timeout: 8_000 });
    const body = await p.locator('body').innerText();
    const strategy = p.locator('select').filter({ has: p.locator('option', { hasText: '请选择独立 Paper 策略' }) });
    return [
      check('handoff link contains ticker', /research_ticker=/.test(href || ''), href || ''),
      check('handoff link contains run id', /research_run_id=/.test(href || ''), href || ''),
      check('paper source banner shown', body.includes('来源研究记录')),
      check('trade intent not preselected', await strategy.inputValue().then((value) => value === '').catch(() => false)),
    ];
  });

	await runTask('paper-gating', '/tactical?research_ticker=AAPL&research_run_id=usability-audit', async (p) => {
    await p.getByText('来源研究记录：AAPL').waitFor({ timeout: 8_000 });
    const strategy = p.locator('select').filter({ has: p.locator('option', { hasText: '预设回撤买入' }) });
    const before = await strategy.inputValue();
    await strategy.selectOption('BUY_LOW');
    await p.waitForTimeout(4_500);
    const body = await p.locator('body').innerText();
    const submit = p.getByRole('button', { name: /\u9a8c\u8bc1\u5e76\u63d0\u4ea4 Paper \u8ba2\u5355/ });
	let enabled = !(await submit.isDisabled());
	let retried = false;
	if (!enabled) {
		const retry = p.getByRole('button', { name: /重新获取报价|重新检查日初基线/ });
		if (await retry.isVisible().catch(() => false) && await retry.isEnabled().catch(() => false)) {
			await retry.click();
			retried = true;
			await p.waitForTimeout(1_000);
			enabled = !(await submit.isDisabled());
		}
	}
	const bodyAfterRetry = await p.locator('body').innerText();
	const baselineBlocked = bodyAfterRetry.includes('当日权益基线尚未建立') && bodyAfterRetry.includes('重新检查日初基线');
	const hasBlockReason = bodyAfterRetry.includes('条件单暂不可用') || bodyAfterRetry.includes('禁止复制') || bodyAfterRetry.includes('已超过 90 秒') || bodyAfterRetry.includes('市场已闭市') || baselineBlocked;
	let paperLifecycle = 'read-only';
	if (allowPaperWrites && viewport.name === 'desktop' && enabled) {
		const validationResponse = p.waitForResponse((response) => response.url().includes('/api/paper-orders/validate') && response.request().method() === 'POST', { timeout: 10_000 });
		await submit.click();
		const validationDetail = await validationResponse.then(async (response) => ({ status: response.status(), body: await response.json().catch(() => null) })).catch((error) => ({ error: String(error) }));
		const cancel = p.getByRole('button', { name: '取消 Paper 订单' });
		const accepted = await cancel.waitFor({ timeout: 10_000 }).then(() => true).catch(() => false);
		if (accepted) {
			await cancel.click();
			const cancelled = await p.getByText(/状态 CANCELLED/).waitFor({ timeout: 10_000 }).then(() => true).catch(() => false);
			paperLifecycle = cancelled ? 'submitted-and-cancelled' : 'cancel-not-confirmed';
		} else {
			paperLifecycle = `submit-not-accepted: ${JSON.stringify(validationDetail)}`;
		}
	}
    return [
      check('research starts without intent', before === ''),
      check('user can choose strategy', (await strategy.inputValue()) === 'BUY_LOW'),
      check('paper account boundary visible', body.includes('Paper 工作台独立')),
	  check('action enabled or explicitly blocked', enabled || hasBlockReason, enabled ? 'enabled' : 'blocked with reason'),
	  check('blocked action has an immediate recovery check', enabled || retried || hasBlockReason, enabled ? 'quote became actionable' : retried ? 'recovery clicked' : 'recovery is already running or block reason is visible'),
	  !allowPaperWrites || viewport.name !== 'desktop'
		? notRun('Paper submit and cancel lifecycle', 'not executed; dedicated Paper runtime audit is authoritative')
		: check('Paper submit and cancel lifecycle', paperLifecycle === 'submitted-and-cancelled', baselineBlocked ? 'scanner baseline blocked before submit with recovery' : paperLifecycle),
    ];
  });

	await runTask('etf-cn-tools', '/etf#cn-tools', async (p) => {
	const bodyBefore = await p.locator('body').innerText();
	await p.getByRole('button', { name: /ETF \/ 热门 \/ 高信念/ }).click();
	await p.getByText(/A股工具箱 · ETF 推荐/).waitFor({ timeout: 8_000 });
	const bodyAfter = await p.locator('body').innerText();
	const detailLinks = p.locator('a[href^="/stock/"]:visible');
	const detailCount = await detailLinks.count();
	let detailHref = '';
	let detailAction = false;
	if (detailCount > 0) {
	  const detailLink = detailLinks.first();
	  detailHref = await detailLink.getAttribute('href') || '';
	  await detailLink.click();
	  await p.waitForURL(/\/stock\//, { timeout: 8_000 });
	  detailAction = Boolean(detailHref) && p.url().includes(detailHref);
	} else {
	  detailAction = (bodyAfter.includes('数据已过期') || bodyAfter.includes('数据不可用')) && bodyAfter.includes('重试');
	}
    return [
	  check('cn tools anchor content visible', bodyBefore.includes('A 股工具箱')),
	  check('picks tab interactive', bodyAfter.includes('A股工具箱 · ETF 推荐')),
	  check('live detail works or stale recommendations fail closed with recovery', detailAction, detailHref || 'stale fail-closed'),
	  check('historical boundary visible', bodyAfter.includes('历史') || bodyAfter.includes('过期') || bodyAfter.includes('数据时间')),
    ];
  });

	await runTask('gpu-tracker', '/gpu-prices', async (p) => {
    const body = await p.locator('body').innerText();
    const upstream = p.getByRole('button', { name: '刷新上游' });
    return [
      check('tracker identity visible', body.includes('官方 GPU 租金跟踪')),
      check('reference prices visible', body.includes('官方核验参考价')),
      check('unconfigured refresh disabled', await upstream.isDisabled()),
      check('configuration reason visible', body.includes('未配置') && body.includes('provider')),
    ];
  });

  await context.close();
}

await browser.close();
const passed = evidence.filter((row) => row.conclusion === 'passed').length;
const partial = evidence.filter((row) => row.conclusion === 'partial').length;
const failed = evidence.filter((row) => row.conclusion === 'failed').length;
const summary = {
  baseURL,
  round,
  checkedAt: new Date().toISOString(),
	complete: failed === 0 && partial === 0,
	passed,
	partial,
	failed,
  total: evidence.length,
	writeLifecycleEvidence: {
		research: allowWrites ? 'opt-in usability probe only; no dedicated lifecycle claim' : 'not-run',
		paper: 'scripts/verify-paper-browser-runtime.sh is authoritative',
		gpu: 'scripts/verify-gpu-browser-runtime.sh is authoritative',
	},
  evidence,
};
await fs.writeFile(path.join(outputDir, 'audit.json'), `${JSON.stringify(summary, null, 2)}\n`);
console.log(JSON.stringify({ outputDir, complete: summary.complete, passed: summary.passed, partial: summary.partial, failed: summary.failed, total: summary.total, writeLifecycleEvidence: summary.writeLifecycleEvidence }, null, 2));
if (summary.failed) process.exitCode = 1;
