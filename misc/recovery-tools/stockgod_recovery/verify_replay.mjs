#!/usr/bin/env node
import fs from 'node:fs/promises';
import path from 'node:path';
import { createRequire } from 'node:module';

const defaultRoutes = [
  '/',
  '/scan',
  '/etf',
  '/whales',
  '/whales/howard-marks',
  '/arena',
  '/reports',
  '/portfolio',
  '/stock/NVDA',
  '/stock/UNH-B',
  '/about',
  '/terms',
  '/privacy',
  '/how-to-buy',
];

function parseArgs(argv) {
  const args = {
    origin: 'http://localhost:8765',
    routes: defaultRoutes,
    out: 'recovered_source/stockgod-replay-verify.json',
    timeout: 60000,
    requireReplay: true,
    minTextRatio: 0.9,
  };
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === '--origin') args.origin = argv[++i].replace(/\/$/, '');
    else if (arg === '--routes') args.routes = argv[++i].split(',').map((r) => r.trim()).filter(Boolean);
    else if (arg === '--out') args.out = argv[++i];
    else if (arg === '--timeout') args.timeout = Number(argv[++i] || args.timeout);
    else if (arg === '--baseline-capture') args.baselineCapture = argv[++i];
    else if (arg === '--min-text-ratio') args.minTextRatio = Number(argv[++i] || args.minTextRatio);
    else if (arg === '--allow-no-replay') args.requireReplay = false;
    else if (arg === '--help' || arg === '-h') {
      console.log('Usage: node scripts/stockgod_recovery/verify_replay.mjs --origin http://localhost:8765 --out recovered_source/stockgod-replay-verify.json');
      process.exit(0);
    }
  }
  return args;
}

async function loadBaseline(captureDir) {
  if (!captureDir) return new Map();
  const index = JSON.parse(await fs.readFile(path.join(captureDir, 'capture-index.json'), 'utf8'));
  return new Map(index.routes.map((route) => [route.route, route]));
}

async function importPlaywright() {
  const unwrap = (mod) => (mod.chromium ? mod : mod.default);
  try {
    return unwrap(await import('playwright'));
  } catch (error) {
    const require = createRequire(import.meta.url);
    for (const root of [process.cwd(), path.join(process.cwd(), 'app/frontend'), path.join(process.cwd(), 'frontend')]) {
      try {
        const resolved = require.resolve('playwright', { paths: [root] });
        return unwrap(await import(resolved));
      } catch {}
    }
    throw error;
  }
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  const baseline = await loadBaseline(args.baselineCapture);
  const playwright = await importPlaywright();
  const browser = await playwright.chromium.launch({ headless: true });
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } });
  const results = [];

  for (const route of args.routes) {
    const page = await context.newPage();
    const badResponses = [];
    const consoleErrors = [];
    const replayHits = [];

    page.on('console', (msg) => {
      if (msg.type() === 'error') consoleErrors.push(msg.text());
    });
    page.on('response', (res) => {
      const status = res.status();
      const url = res.url();
      const replay = res.headers()['x-stockgod-replay'] || '';
      if (replay) replayHits.push({ status, url });
      if (status >= 400 && !url.includes('favicon') && !url.includes('assets.parqet.com/logos')) {
        badResponses.push({ status, url });
      }
    });

    let navError = null;
    try {
      await page.goto(`${args.origin}${route}`, { waitUntil: 'networkidle', timeout: args.timeout });
    } catch (error) {
      navError = String(error?.message || error);
    }

    const pageResult = await page.evaluate(() => ({
      title: document.title,
      h1: document.querySelector('h1')?.textContent?.trim() || '',
      textLength: document.body?.innerText?.length || 0,
    })).catch(() => ({ title: '', h1: '', textLength: 0 }));

    const base = baseline.get(route);
    const textRatio = base?.textLength ? pageResult.textLength / base.textLength : null;
    results.push({
      route,
      ...pageResult,
      baselineTextLength: base?.textLength || null,
      textRatio,
      navError,
      replayHitCount: replayHits.length,
      badResponses,
      consoleErrors: consoleErrors
        .filter((msg) => !msg.includes('assets.parqet.com/logos'))
        .filter((msg) => msg !== 'Failed to load resource: the server responded with a status of 404 ()')
        .slice(0, 5),
    });
    await page.close();
  }

  await browser.close();
  await fs.mkdir(path.dirname(args.out), { recursive: true });
  await fs.writeFile(args.out, JSON.stringify({ origin: args.origin, checkedAt: new Date().toISOString(), results }, null, 2));

  const failing = results.filter((item) =>
    item.navError ||
    item.badResponses.length > 0 ||
    item.consoleErrors.length > 0 ||
    item.textLength <= 0 ||
    (args.requireReplay && item.replayHitCount <= 0) ||
    (item.textRatio !== null && item.textRatio < args.minTextRatio)
  );
  console.table(results.map((item) => ({
    route: item.route,
    h1: item.h1,
    text: item.textLength,
    base: item.baselineTextLength || '',
    ratio: item.textRatio === null ? '' : Math.round(item.textRatio * 1000) / 10,
    replay: item.replayHitCount,
    bad: item.badResponses.length,
    errors: item.consoleErrors.length,
  })));
  if (failing.length > 0) {
    console.error(`Verification failed for ${failing.length} route(s).`);
    console.error(JSON.stringify(failing.map((item) => ({
      route: item.route,
      navError: item.navError,
      textLength: item.textLength,
      baselineTextLength: item.baselineTextLength,
      textRatio: item.textRatio,
      replayHitCount: item.replayHitCount,
      badResponses: item.badResponses.length,
      consoleErrors: item.consoleErrors.length,
    })), null, 2));
  }
  console.log(`Wrote ${args.out}`);
  if (failing.length > 0) process.exitCode = 1;
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
