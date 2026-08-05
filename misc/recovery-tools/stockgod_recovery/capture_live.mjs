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
    origin: 'https://stockgod.xyz',
    out: 'recovered_source/stockgod-live-capture',
    routes: defaultRoutes,
    timeout: 60000,
    concurrency: 1,
    resume: false,
    screenshots: true,
  };

  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === '--origin') args.origin = argv[++i].replace(/\/$/, '');
    else if (arg === '--out') args.out = argv[++i];
    else if (arg === '--routes') args.routes = argv[++i].split(',').map((r) => r.trim()).filter(Boolean);
    else if (arg === '--routes-file') args.routesFile = argv[++i];
    else if (arg === '--timeout') args.timeout = Number(argv[++i] || args.timeout);
    else if (arg === '--concurrency') args.concurrency = Math.max(1, Number(argv[++i] || args.concurrency));
    else if (arg === '--resume') args.resume = true;
    else if (arg === '--no-screenshots') args.screenshots = false;
    else if (arg === '--help' || arg === '-h') {
      console.log(`Usage:
node scripts/stockgod_recovery/capture_live.mjs --origin https://stockgod.xyz --out recovered_source/stockgod-live-capture

Captures HTML, Next/RSC/API response bodies, DOM summaries, links, controls, and screenshots for StockGod recovery.`);
      process.exit(0);
    }
  }

  return args;
}

async function hydrateRoutes(args) {
  const routes = [...args.routes];
  if (args.routesFile) {
    const text = await fs.readFile(args.routesFile, 'utf8');
    routes.push(...text.split(/\r?\n/).map((line) => line.trim()).filter((line) => line && !line.startsWith('#')));
  }
  return [...new Set(routes)];
}

function routeToUrl(origin, route) {
  if (/^https?:\/\//.test(route)) return route;
  return `${origin}${route.startsWith('/') ? route : `/${route}`}`;
}

function safeName(routeOrUrl) {
  const url = new URL(routeOrUrl, 'https://stockgod.xyz');
  const raw = `${url.pathname}${url.search}` || 'root';
  return raw.replace(/^\/$/, 'root').replace(/[^a-zA-Z0-9._-]+/g, '_').replace(/^_+|_+$/g, '') || 'root';
}

async function importPlaywright() {
  const unwrap = (mod) => (mod.chromium ? mod : mod.default);
  try {
    const mod = await import('playwright');
    return unwrap(mod);
  } catch (error) {
    const require = createRequire(import.meta.url);
    for (const root of [process.cwd(), path.join(process.cwd(), 'app/frontend'), path.join(process.cwd(), 'frontend')]) {
      try {
        const resolved = require.resolve('playwright', { paths: [root] });
        const mod = await import(resolved);
        return unwrap(mod);
      } catch {}
    }
    throw error;
  }
}

function extractFlightScripts(html) {
  const chunks = [];
  const re = /<script[^>]*>\s*self\.__next_f\.push\(\[1,([\s\S]*?)\]\)\s*<\/script>/g;
  let match;
  while ((match = re.exec(html))) {
    chunks.push(match[1].slice(0, 200000));
  }
  return chunks;
}

function isInterestingResponse(url, contentType) {
  return (
    /\/api\/|\/data\/|_rsc=|\/_next\/data\//i.test(url) ||
    /json|text\/x-component|rsc/i.test(contentType)
  );
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  args.routes = await hydrateRoutes(args);
  await fs.mkdir(args.out, { recursive: true });
  await fs.mkdir(path.join(args.out, 'routes'), { recursive: true });

  const playwright = await importPlaywright();
  const browser = await playwright.chromium.launch({ headless: true });
  const context = await browser.newContext({
    viewport: { width: 1440, height: 1100 },
    ignoreHTTPSErrors: true,
  });

  const index = {
    origin: args.origin,
    startedAt: new Date().toISOString(),
    routes: [],
  };
  const indexByRoute = new Map();

  async function captureRoute(route) {
    const url = routeToUrl(args.origin, route);
    const name = safeName(route);
    const routeDir = path.join(args.out, 'routes', name);

    if (args.resume) {
      try {
        const existing = JSON.parse(await fs.readFile(path.join(routeDir, 'summary.json'), 'utf8'));
        return {
          route,
          name,
          title: existing.title,
          textLength: existing.textLength,
          pageError: existing.pageError,
          htmlBytes: existing.htmlBytes,
          flightChunkCount: existing.flightChunkCount,
          responseCount: existing.responses?.length || 0,
          apiBodyCount: existing.apiBodies?.filter((body) => body.file).length || 0,
          failedCount: existing.failedRequests?.length || 0,
          consoleErrorCount: existing.consoleErrors?.length || 0,
          skipped: true,
        };
      } catch {}
    }

    if (!args.resume) {
      await fs.rm(routeDir, { recursive: true, force: true });
    }
    await fs.mkdir(routeDir, { recursive: true });

    const page = await context.newPage();
    const responses = [];
    const apiBodies = [];
    const failedRequests = [];
    const consoleErrors = [];

    page.on('console', (msg) => {
      if (msg.type() === 'error') consoleErrors.push(msg.text());
    });
    page.on('requestfailed', (req) => {
      failedRequests.push({ url: req.url(), error: req.failure()?.errorText || 'failed' });
    });
    page.on('response', async (res) => {
      const responseUrl = res.url();
      const status = res.status();
      const contentType = res.headers()['content-type'] || '';
      const item = { status, url: responseUrl, contentType };
      responses.push(item);

      if (!isInterestingResponse(responseUrl, contentType) || apiBodies.length >= 80) return;
      const bodyIndex = apiBodies.length + 1;
      apiBodies.push({ ...item, pending: true });
      try {
        const body = await res.text();
        const bodyName = `response_${String(bodyIndex).padStart(3, '0')}.${/json/i.test(contentType) ? 'json' : 'txt'}`;
        await fs.writeFile(path.join(routeDir, bodyName), body);
        apiBodies[bodyIndex - 1] = { ...item, file: bodyName, bytes: body.length, sample: body.slice(0, 1000) };
      } catch (error) {
        apiBodies[bodyIndex - 1] = { ...item, error: String(error?.message || error) };
      }
    });

    let pageError = null;
    try {
      await page.goto(url, { waitUntil: 'networkidle', timeout: args.timeout });
    } catch (error) {
      pageError = String(error?.message || error);
    }

    const html = await page.content();
    await fs.writeFile(path.join(routeDir, 'page.html'), html);
    const flightChunks = extractFlightScripts(html);
    await fs.writeFile(path.join(routeDir, 'next-flight-chunks.json'), JSON.stringify(flightChunks, null, 2));

    const dom = await page.evaluate(() => {
      const text = document.body?.innerText || '';
      return {
        finalUrl: location.href,
        title: document.title,
        textLength: text.length,
        textSample: text.slice(0, 4000),
        headings: [...document.querySelectorAll('h1,h2,h3')].map((h) => ({
          tag: h.tagName.toLowerCase(),
          text: (h.textContent || '').trim().replace(/\s+/g, ' ').slice(0, 200),
        })),
        links: [...document.querySelectorAll('a[href]')].map((a) => ({
          text: (a.textContent || '').trim().replace(/\s+/g, ' ').slice(0, 160),
          href: a.href,
        })),
        controls: [...document.querySelectorAll('button,[role="button"],input,select,textarea')].map((el) => ({
          tag: el.tagName.toLowerCase(),
          type: el.getAttribute('type') || '',
          text: (el.textContent || el.getAttribute('aria-label') || el.getAttribute('placeholder') || '').trim().replace(/\s+/g, ' ').slice(0, 160),
          disabled: Boolean(el.disabled || el.getAttribute('aria-disabled') === 'true'),
        })),
      };
    });

    let screenshot = null;
    if (args.screenshots) {
      screenshot = 'screenshot.png';
      await page.screenshot({ path: path.join(routeDir, screenshot), fullPage: true }).catch(() => {});
    }

    const routeSummary = {
      route,
      url,
      name,
      pageError,
      screenshot,
      htmlBytes: html.length,
      flightChunkCount: flightChunks.length,
      consoleErrors,
      failedRequests,
      responses,
      apiBodies,
      ...dom,
    };
    await fs.writeFile(path.join(routeDir, 'summary.json'), JSON.stringify(routeSummary, null, 2));
    const indexItem = {
      route,
      name,
      title: dom.title,
      textLength: dom.textLength,
      pageError,
      htmlBytes: html.length,
      flightChunkCount: flightChunks.length,
      responseCount: responses.length,
      apiBodyCount: apiBodies.length,
      failedCount: failedRequests.length,
      consoleErrorCount: consoleErrors.length,
    };

    await page.close();
    return indexItem;
  }

  let cursor = 0;
  async function worker() {
    while (cursor < args.routes.length) {
      const route = args.routes[cursor];
      cursor += 1;
      try {
        const item = await captureRoute(route);
        indexByRoute.set(route, item);
        const done = indexByRoute.size;
        if (done === args.routes.length || done % 25 === 0) {
          console.log(`Captured ${done}/${args.routes.length}`);
        }
      } catch (error) {
        indexByRoute.set(route, {
          route,
          name: safeName(route),
          title: '',
          textLength: 0,
          pageError: String(error?.message || error),
          htmlBytes: 0,
          flightChunkCount: 0,
          responseCount: 0,
          apiBodyCount: 0,
          failedCount: 0,
          consoleErrorCount: 0,
        });
      }
    }
  }

  await Promise.all(Array.from({ length: Math.min(args.concurrency, args.routes.length) }, () => worker()));
  index.routes = args.routes.map((route) => indexByRoute.get(route)).filter(Boolean);

  index.finishedAt = new Date().toISOString();
  await fs.writeFile(path.join(args.out, 'capture-index.json'), JSON.stringify(index, null, 2));
  await browser.close();
  console.log(`Wrote ${path.join(args.out, 'capture-index.json')}`);
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
