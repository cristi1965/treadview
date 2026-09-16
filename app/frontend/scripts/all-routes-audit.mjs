import fs from 'node:fs/promises';
import path from 'node:path';
import { chromium } from 'playwright';

const baseURL = process.env.USABILITY_BASE_URL || 'http://127.0.0.1:8831';
const round = process.env.USABILITY_ROUND || 'r1';
const outputDir = path.resolve(process.cwd(), '../../output/playwright', `all-routes-${round}`);
const staticRoutes = [
	['/', /研究|工作台/],
  ['/dashboard', /研究|工作台/], ['/history', /历史|研究证据/], ['/lab', /实验|研究/],
  ['/tactical', /Paper/], ['/journal', /Paper|PAPER/], ['/settings', /设置|产品承诺/],
  ['/market', /行情|我不是神/], ['/scan', /扫描|列表/], ['/stock/NVDA', /NVDA/],
  ['/etf', /ETF/], ['/etf/510300', /510300|ETF/], ['/etf#cn-tools', /A 股工具箱|ETF/],
  ['/reports', /盘报|报告/], ['/macro', /宏观/], ['/dashboard/macro', /宏观/],
  ['/portfolio', /持仓|Paper|PAPER/], ['/arena', /对决|虚拟盘/], ['/copilot', /AI 交易问答|NEMO/],
  ['/multichart', /图表|同屏/], ['/gpu-prices', /GPU/], ['/notes', /笔记/],
  ['/whales', /聪明钱|机构|国会/], ['/whales/howard-marks', /Howard|Marks|霍华德/],
  ['/whales/congress/nancy-pelosi', /Pelosi|佩洛西|国会/], ['/about', /关于|About/],
  ['/terms', /条款|Terms/], ['/privacy', /隐私|Privacy/], ['/how-to-buy', /交易|买|buy/i],
	['/__dead_page_probe__', /404|未找到|不存在|返回/],
];
const viewports = [
	{ name: 'desktop', width: 1440, height: 1000, isMobile: false, hasTouch: false, deviceScaleFactor: 1 },
	{ name: 'narrow-desktop', width: 1024, height: 900, isMobile: false, hasTouch: false, deviceScaleFactor: 1 },
	{ name: 'mobile', width: 390, height: 844, isMobile: true, hasTouch: true, deviceScaleFactor: 3 },
];

function firstNote(sections) {
	for (const section of sections || []) {
		const note = section.items?.find((item) => item?.id);
		if (note) return note;
	}
	return null;
}

async function discoverDynamicRoutes(request) {
	const [reportsResponse, notesResponse] = await Promise.all([
		request.get(`${baseURL}/api/reports?limit=5`),
		request.get(`${baseURL}/api/notes/toc`),
	]);
	if (!reportsResponse.ok()) throw new Error(`report discovery failed: HTTP ${reportsResponse.status()}`);
	if (!notesResponse.ok()) throw new Error(`note discovery failed: HTTP ${notesResponse.status()}`);

	const reportsPayload = await reportsResponse.json();
	const report = reportsPayload.reports?.find((item) => item?.id);
	const notesPayload = await notesResponse.json();
	const note = firstNote(notesPayload.sections);
	if (!report) throw new Error('report discovery returned no usable report id');
	if (!note) throw new Error('note discovery returned no usable note id');

	return [
		[`/reports/${encodeURIComponent(report.id)}`, /盘报|报告|复盘/],
		[`/notes/${encodeURIComponent(note.id)}`, /笔记/],
	];
}

await fs.mkdir(outputDir, { recursive: true });
const browser = await chromium.launch({ headless: true });
const evidence = [];
const discoveryContext = await browser.newContext();
const dynamicRoutes = await discoverDynamicRoutes(discoveryContext.request);
await discoveryContext.close();
const routes = [...staticRoutes, ...dynamicRoutes];

for (const viewport of viewports) {
	const context = await browser.newContext({
		viewport: { width: viewport.width, height: viewport.height },
		isMobile: viewport.isMobile,
		hasTouch: viewport.hasTouch,
		deviceScaleFactor: viewport.deviceScaleFactor,
	});
  const sessionPage = await context.newPage();
  await sessionPage.goto(`${baseURL}/settings`, { waitUntil: 'domcontentloaded', timeout: 15_000 });
  await sessionPage.waitForTimeout(500);
  await sessionPage.close();

  for (const [route, identity] of routes) {
    const page = await context.newPage();
    const row = { viewport: viewport.name, route, passed: false, assertions: [], pageErrors: [], serverErrors: [], durationMs: 0 };
    const started = Date.now();
    page.on('pageerror', (error) => row.pageErrors.push(error.message));
    page.on('response', (response) => {
      if (response.status() >= 500 && new URL(response.url()).origin === baseURL) row.serverErrors.push(`${response.status()} ${new URL(response.url()).pathname}`);
    });
    try {
      await page.goto(`${baseURL}${route}`, { waitUntil: 'domcontentloaded', timeout: 15_000 });
      await page.waitForTimeout(900);
	      const result = await page.evaluate(() => {
			const text = document.body.innerText.trim();
			const recoveryControls = [...document.querySelectorAll('button:not([disabled]), a[href]')]
				.filter((element) => element instanceof HTMLElement && element.offsetParent !== null)
				.filter((element) => /(重试|重新|刷新|诊断|修复|返回|历史)/.test(element.textContent?.trim() || ''));
			const recoveryRegions = recoveryControls.map((element) => (
				element.closest('[role="alert"], [role="status"]')?.innerText
				|| element.parentElement?.innerText
				|| ''
			).trim());
			const explicitlyHandledRegions = recoveryRegions.filter((region) =>
				/(来源|source|数据时间|data time)/i.test(region)
				&& /(失败|不可用|过期|降级|stale|unavailable|error|degraded)/i.test(region)
			);
		      return {
				text,
				interactive: document.querySelectorAll('button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), a[href]').length,
				overflow: document.documentElement.scrollWidth > document.documentElement.clientWidth,
				hasExplicitFailureRegion: explicitlyHandledRegions.length > 0,
				explicitlyHandledRegions: explicitlyHandledRegions.map((region) => region.slice(0, 500)),
				mobileChromeIssues: window.innerWidth > 500 ? [] : [...document.querySelectorAll('.wails-mobile-top button, nav.fixed a[href]')]
					.filter((element) => element instanceof HTMLElement && element.offsetParent !== null)
					.map((element) => {
						const rect = element.getBoundingClientRect();
						const inViewport = rect.bottom > 0 && rect.top < window.innerHeight;
						if (!inViewport) return null;
						const center = document.elementFromPoint(rect.left + rect.width / 2, rect.top + rect.height / 2);
						const blocked = center != null && center !== element && !element.contains(center);
						return rect.width < 44 || rect.height < 44 || blocked
							? `${element.textContent?.trim() || element.getAttribute('aria-label') || element.tagName}: ${Math.round(rect.width)}x${Math.round(rect.height)} blocked=${blocked}`
							: null;
					})
					.filter(Boolean),
			};
		  });
	      const serverErrorsExplicitlyHandled = row.serverErrors.length === 0 || (
			result.hasExplicitFailureRegion
		  );
      row.assertions = [
        { name: 'route identity visible', pass: identity.test(result.text) },
        { name: 'substantial content rendered', pass: result.text.length >= 120, detail: `${result.text.length} chars` },
        { name: 'interactive surface available', pass: result.interactive > 0, detail: `${result.interactive} controls` },
        { name: 'no page-level horizontal overflow', pass: !result.overflow },
		{ name: 'mobile chrome touch targets and centers are usable', pass: result.mobileChromeIssues.length === 0, detail: result.mobileChromeIssues.join('; ') },
        { name: 'no runtime exception', pass: row.pageErrors.length === 0 },
		{ name: 'server errors are explicitly handled', pass: serverErrorsExplicitlyHandled, detail: row.serverErrors.length ? JSON.stringify({ serverErrors: row.serverErrors, explicitlyHandledRegions: result.explicitlyHandledRegions }) : 'no 5xx response' },
      ];
      row.passed = row.assertions.every((item) => item.pass);
    } catch (error) {
      row.pageErrors.push(error instanceof Error ? error.message : String(error));
    }
    row.durationMs = Date.now() - started;
    if (!row.passed || ['/dashboard', '/tactical', '/journal', '/gpu-prices'].includes(route)) {
      const name = route.replace(/[^a-z0-9]+/gi, '-').replace(/^-|-$/g, '') || 'root';
      await page.screenshot({ path: path.join(outputDir, `${viewport.name}-${name}.png`), fullPage: false }).catch(() => undefined);
    }
    evidence.push(row);
    await page.close();
  }
  await context.close();
}

await browser.close();
const summary = {
  baseURL,
  round,
  checkedAt: new Date().toISOString(),
  passed: evidence.filter((row) => row.passed).length,
  failed: evidence.filter((row) => !row.passed).length,
  total: evidence.length,
  routesWithHandledServerErrors: evidence.filter((row) => row.serverErrors.length).length,
  evidence,
};
await fs.writeFile(path.join(outputDir, 'audit.json'), `${JSON.stringify(summary, null, 2)}\n`);
console.log(JSON.stringify({ outputDir, passed: summary.passed, failed: summary.failed, total: summary.total, routesWithHandledServerErrors: summary.routesWithHandledServerErrors }, null, 2));
if (summary.failed) process.exitCode = 1;
