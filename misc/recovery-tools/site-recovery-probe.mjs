#!/usr/bin/env node
import fs from 'node:fs/promises';
import path from 'node:path';
import playwright from '../app/frontend/node_modules/playwright/index.js';

const { chromium } = playwright;

function arg(name, fallback = '') {
  const idx = process.argv.indexOf(name);
  return idx >= 0 ? process.argv[idx + 1] : fallback;
}

const origin = arg('--origin').replace(/\/$/, '');
const routes = arg('--routes', '/').split(',').map((r) => r.trim()).filter(Boolean);
const out = arg('--out', 'recovered_source/site-recovery/probe');
const timeout = Number(arg('--timeout', '45000'));
const waitUntil = arg('--wait-until', 'domcontentloaded');
const settleMs = Number(arg('--settle-ms', '1200'));

if (!origin) {
  console.error('Usage: node scripts/site-recovery-probe.mjs --origin https://example.com --routes /,/x --out output-dir');
  process.exit(2);
}

function routeToURL(route) {
  if (/^https?:\/\//.test(route)) return route;
  return `${origin}${route.startsWith('/') ? route : `/${route}`}`;
}

function safeName(route) {
  return route.replace(/^\/$/, 'root').replace(/[^a-zA-Z0-9._-]+/g, '_').replace(/^_+|_+$/g, '') || 'root';
}

function healthURL() {
  try {
    const url = new URL(origin);
    if (!['127.0.0.1', 'localhost'].includes(url.hostname)) return '';
    return `${url.origin}/api/health`;
  } catch {
    return '';
  }
}

async function checkLocalHealth() {
  const url = healthURL();
  if (!url) return { checked: false };
  try {
    const res = await fetch(url, { signal: AbortSignal.timeout(2500) });
    return { checked: true, ok: res.ok, status: res.status };
  } catch (err) {
    return { checked: true, ok: false, error: err instanceof Error ? err.message : String(err) };
  }
}

await fs.mkdir(out, { recursive: true });

const browser = await chromium.launch({ headless: true });
const context = await browser.newContext({ viewport: { width: 1440, height: 1100 } });
const results = [];

for (const route of routes) {
  const health = await checkLocalHealth();
  if (health.checked && !health.ok) {
    const url = routeToURL(route);
    results.push({
      route,
      url,
      status: 0,
      durationMs: 0,
      error: `local health check failed: ${health.error || health.status || 'unknown'}`,
      health,
      screenshot: '',
      requests: [],
      failedRequests: [],
      consoleMessages: [],
      snapshot: { evaluateError: 'skipped because local origin is unavailable' },
    });
    continue;
  }

  const page = await context.newPage();
  const requests = [];
  const failedRequests = [];
  const consoleMessages = [];

  page.on('request', (req) => {
    const url = req.url();
    if (url.includes('/api/') || url.includes('/data/') || url.includes('/_next/') || url.includes('/assets/')) {
      requests.push({ method: req.method(), url });
    }
  });
  page.on('requestfailed', (req) => {
    failedRequests.push({ method: req.method(), url: req.url(), failure: req.failure()?.errorText || '' });
  });
  page.on('console', (msg) => {
    if (['error', 'warning'].includes(msg.type())) {
      consoleMessages.push({ type: msg.type(), text: msg.text().slice(0, 500) });
    }
  });

  const url = routeToURL(route);
  const started = Date.now();
  let status = 0;
  let error = '';
  try {
    const response = await page.goto(url, { waitUntil, timeout });
    status = response?.status() || 0;
    if (settleMs > 0) {
      await page.waitForTimeout(settleMs);
    }
  } catch (err) {
    error = err instanceof Error ? err.message : String(err);
  }

  const snapshot = await page.evaluate(() => {
    const text = (el) => (el?.innerText || el?.textContent || '').replace(/\s+/g, ' ').trim();
    const attr = (el, name) => el.getAttribute(name) || '';
    return {
      title: document.title,
      url: location.href,
      h1: Array.from(document.querySelectorAll('h1')).map(text).filter(Boolean).slice(0, 10),
      buttons: Array.from(document.querySelectorAll('button,[role="button"]')).map((el) => ({
        text: text(el).slice(0, 120),
        aria: attr(el, 'aria-label'),
        disabled: el.disabled || attr(el, 'aria-disabled') === 'true',
      })).slice(0, 200),
      links: Array.from(document.querySelectorAll('a[href]')).map((el) => ({
        text: text(el).slice(0, 120),
        href: el.href,
      })).slice(0, 200),
      inputs: Array.from(document.querySelectorAll('input,textarea,select')).map((el) => ({
        tag: el.tagName.toLowerCase(),
        type: attr(el, 'type'),
        placeholder: attr(el, 'placeholder'),
        name: attr(el, 'name'),
        aria: attr(el, 'aria-label'),
      })).slice(0, 80),
      textSample: text(document.body).slice(0, 3000),
    };
  }).catch((err) => ({ evaluateError: err instanceof Error ? err.message : String(err) }));

  const screenshot = path.join(out, `${safeName(route)}.png`);
  await page.screenshot({ path: screenshot, fullPage: true }).catch(() => {});
  await page.close();

  results.push({
    route,
    url,
    status,
    durationMs: Date.now() - started,
    error,
    health,
    waitUntil,
    settleMs,
    screenshot,
    requests,
    failedRequests,
    consoleMessages,
    snapshot,
  });
}

await browser.close();

const report = {
  origin,
  checkedAt: new Date().toISOString(),
  routes,
  results,
};

await fs.writeFile(path.join(out, 'site-probe.json'), JSON.stringify(report, null, 2));
console.log(`Wrote ${path.join(out, 'site-probe.json')}`);
