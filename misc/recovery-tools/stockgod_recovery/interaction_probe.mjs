#!/usr/bin/env node
import fs from 'node:fs/promises';
import path from 'node:path';
import { createRequire } from 'node:module';

const defaultRoutes = ['/', '/scan', '/whales', '/arena', '/reports', '/portfolio', '/stock/AAPL?market=us'];

function parseArgs(argv) {
  const args = {
    origin: 'http://localhost:8765',
    routes: defaultRoutes,
    out: 'recovered_source/stockgod-interaction-probe.json',
    timeout: 45000,
    maxLinks: 8,
    maxButtons: 12,
    maxInputs: 5,
    waitUntil: 'domcontentloaded',
    settleMs: 1200,
  };
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === '--origin') args.origin = argv[++i].replace(/\/$/, '');
    else if (arg === '--routes') args.routes = argv[++i].split(',').map((route) => route.trim()).filter(Boolean);
    else if (arg === '--out') args.out = argv[++i];
    else if (arg === '--timeout') args.timeout = Number(argv[++i] || args.timeout);
    else if (arg === '--max-links') args.maxLinks = Number(argv[++i] || args.maxLinks);
    else if (arg === '--max-buttons') args.maxButtons = Number(argv[++i] || args.maxButtons);
    else if (arg === '--max-inputs') args.maxInputs = Number(argv[++i] || args.maxInputs);
    else if (arg === '--wait-until') args.waitUntil = argv[++i] || args.waitUntil;
    else if (arg === '--settle-ms') args.settleMs = Number(argv[++i] || args.settleMs);
  }
  return args;
}

async function importPlaywright() {
  const unwrap = (mod) => (mod.chromium ? mod : mod.default);
  try {
    return unwrap(await import('playwright'));
  } catch (error) {
    const require = createRequire(import.meta.url);
    for (const root of [process.cwd(), path.join(process.cwd(), 'app/frontend'), path.join(process.cwd(), 'frontend')]) {
      try {
        return unwrap(await import(require.resolve('playwright', { paths: [root] })));
      } catch {}
    }
    throw error;
  }
}

async function pageState(page) {
  return page.evaluate(() => ({
    url: location.href,
    title: document.title,
    h1: document.querySelector('h1')?.textContent?.trim() || '',
    textLength: document.body?.innerText?.length || 0,
  }));
}

function sameOriginInternal(href, origin) {
  try {
    const url = new URL(href);
    const base = new URL(origin);
    return url.origin === base.origin && !url.pathname.startsWith('/_next') && !url.pathname.startsWith('/api/');
  } catch {
    return false;
  }
}

function healthURL(origin) {
  try {
    const url = new URL(origin);
    if (!['127.0.0.1', 'localhost'].includes(url.hostname)) return '';
    return `${url.origin}/api/health`;
  } catch {
    return '';
  }
}

async function checkLocalHealth(origin) {
  const url = healthURL(origin);
  if (!url) return { checked: false };
  try {
    const res = await fetch(url, { signal: AbortSignal.timeout(2500) });
    return { checked: true, ok: res.ok, status: res.status };
  } catch (error) {
    return { checked: true, ok: false, error: String(error?.message || error) };
  }
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  const playwright = await importPlaywright();
  const browser = await playwright.chromium.launch({ headless: true });
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } });
  const results = [];

  for (const route of args.routes) {
    const health = await checkLocalHealth(args.origin);
    if (health.checked && !health.ok) {
      results.push({
        route,
        health,
        navError: `local health check failed: ${health.error || health.status || 'unknown'}`,
        initial: null,
        linksChecked: [],
        buttonsChecked: [],
        inputsChecked: [],
        badResponses: [],
        failedRequests: [],
        consoleErrors: [],
      });
      continue;
    }

    const page = await context.newPage();
    const responses = [];
    const consoleErrors = [];
    const failedRequests = [];
    page.on('console', (msg) => {
      if (msg.type() === 'error') consoleErrors.push(msg.text());
    });
    page.on('requestfailed', (req) => failedRequests.push({ url: req.url(), error: req.failure()?.errorText || 'failed' }));
    page.on('response', (res) => {
      const status = res.status();
      if (status >= 400 && !res.url().includes('assets.parqet.com/logos') && !res.url().includes('favicon')) {
        responses.push({ url: res.url(), status });
      }
    });

    const startUrl = `${args.origin}${route}`;
    let navError = null;
    try {
      await page.goto(startUrl, { waitUntil: args.waitUntil, timeout: args.timeout });
      if (args.settleMs > 0) await page.waitForTimeout(args.settleMs);
    } catch (error) {
      navError = String(error?.message || error);
    }

    const initial = await pageState(page).catch(() => null);
    const links = await page.$$eval('a[href]', (anchors) => anchors.map((anchor) => ({
      text: (anchor.textContent || '').trim().replace(/\s+/g, ' ').slice(0, 80),
      href: anchor.href,
    }))).catch(() => []);
    const internalLinks = links.filter((link) => sameOriginInternal(link.href, args.origin)).slice(0, args.maxLinks);

    const linkChecks = [];
    for (const link of internalLinks) {
      const check = await context.newPage();
      let error = null;
      try {
        await check.goto(link.href, { waitUntil: 'domcontentloaded', timeout: 15000 });
      } catch (err) {
        error = String(err?.message || err);
      }
      const state = await pageState(check).catch(() => null);
      linkChecks.push({ ...link, error, state });
      await check.close();
    }

    const buttonChecks = [];
    const buttonCount = await page.locator('button,[role="button"]').count().catch(() => 0);
    for (let i = 0; i < Math.min(buttonCount, args.maxButtons); i += 1) {
      const before = await pageState(page).catch(() => null);
      const label = await page.locator('button,[role="button"]').nth(i).textContent({ timeout: 1000 }).catch(() => '');
      let error = null;
      try {
        await page.locator('button,[role="button"]').nth(i).click({ timeout: 2000 });
        await page.waitForTimeout(500);
      } catch (err) {
        error = String(err?.message || err).slice(0, 300);
      }
      const after = await pageState(page).catch(() => null);
      buttonChecks.push({ index: i, label: (label || '').trim().slice(0, 80), error, changed: before?.textLength !== after?.textLength || before?.url !== after?.url, after });
      if (page.url() !== startUrl) {
        await page.goto(startUrl, { waitUntil: 'domcontentloaded', timeout: args.timeout }).catch(() => {});
      }
    }

    const inputChecks = [];
    const inputCount = await page.locator('input,textarea').count().catch(() => 0);
    for (let i = 0; i < Math.min(inputCount, args.maxInputs); i += 1) {
      const locator = page.locator('input,textarea').nth(i);
      const placeholder = await locator.getAttribute('placeholder').catch(() => '');
      let error = null;
      try {
        await locator.fill('AAPL', { timeout: 2000 });
        await page.keyboard.press('Enter');
        await page.waitForTimeout(700);
      } catch (err) {
        error = String(err?.message || err).slice(0, 300);
      }
      inputChecks.push({ index: i, placeholder, error, after: await pageState(page).catch(() => null) });
    }

    results.push({
      route,
      health,
      waitUntil: args.waitUntil,
      settleMs: args.settleMs,
      navError,
      initial,
      linksChecked: linkChecks,
      buttonsChecked: buttonChecks,
      inputsChecked: inputChecks,
      badResponses: responses,
      failedRequests,
      consoleErrors: consoleErrors.filter((msg) => msg !== 'Failed to load resource: the server responded with a status of 404 ()'),
    });
    await page.close();
  }

  await browser.close();
  await fs.mkdir(path.dirname(args.out), { recursive: true });
  await fs.writeFile(args.out, JSON.stringify({ origin: args.origin, checkedAt: new Date().toISOString(), results }, null, 2));
  console.table(results.map((result) => ({
    route: result.route,
    text: result.initial?.textLength || 0,
    links: result.linksChecked.length,
    buttons: result.buttonsChecked.length,
    inputs: result.inputsChecked.length,
    bad: result.badResponses.length,
    errors: result.consoleErrors.length,
  })));
  console.log(`Wrote ${args.out}`);
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
