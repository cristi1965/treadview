#!/usr/bin/env node
import fs from 'node:fs/promises';
import http from 'node:http';
import https from 'node:https';
import path from 'node:path';

function arg(name, fallback = '') {
  const idx = process.argv.indexOf(name);
  return idx >= 0 ? process.argv[idx + 1] : fallback;
}

const root = process.cwd();
const baseUrl = arg('--base-url', process.env.BASE_URL || 'http://127.0.0.1:8765').replace(/\/$/, '');
const warnHours = Number(arg('--warn-hours', '36'));
const checkApi = process.argv.includes('--api');

const files = [
  { label: 'US universe', path: 'app/frontend/public/data/us-stocks.json', freshness: 'generated_at', maxHours: warnHours },
  { label: 'US panel', path: 'app/frontend/public/data/us-panel-summary.json', freshness: 'generated_at', maxHours: 168 },
  { label: 'US market fallback', path: 'app/backend/data/market.json', freshness: 'ts', maxHours: warnHours },
  { label: 'A-share market', path: 'app/backend/data/a-market.json', freshness: 'ts', maxHours: warnHours },
  { label: 'A-share public copy', path: 'app/frontend/public/data/a-market.json', freshness: 'ts', maxHours: warnHours },
  { label: 'Macro strip', path: 'app/backend/data/macro.json', freshness: 'ts', maxHours: warnHours },
  { label: 'Premarket movers', path: 'app/backend/data/premarket-movers.json', freshness: 'ts', maxHours: warnHours },
];

function parseGeneratedAt(value) {
  if (!value || typeof value !== 'string') return null;
  const normalized = value
    .replace(/\bET\b/i, '-04:00')
    .replace(/\bEST\b/i, '-05:00')
    .replace(/\bEDT\b/i, '-04:00');
  const date = new Date(normalized);
  return Number.isNaN(date.getTime()) ? null : date;
}

function parseFreshness(json, stat, kind) {
  if (kind === 'mtime') return stat.mtime;
  if (kind === 'generated_at') return parseGeneratedAt(json.generated_at) || stat.mtime;
  if (kind === 'updated') return parseGeneratedAt(json.updated) || stat.mtime;
  if (kind === 'ts') {
    const ts = Number(json.ts || json.generated_ts || 0);
    if (ts > 1_000_000_000_000) return new Date(ts);
    if (ts > 1_000_000_000) return new Date(ts * 1000);
    return stat.mtime;
  }
  return stat.mtime;
}

function hoursAgo(date) {
  return (Date.now() - date.getTime()) / 36e5;
}

async function inspectFile(item) {
  const absolute = path.join(root, item.path);
  const stat = await fs.stat(absolute);
  const raw = await fs.readFile(absolute, 'utf8');
  let json = {};
  try {
    json = JSON.parse(raw);
  } catch {}
  const freshAt = parseFreshness(json, stat, item.freshness);
  const ageHours = hoursAgo(freshAt);
  const count =
    json.count ??
    json.n ??
    json.stocks?.length ??
    (json.quotes && Object.keys(json.quotes).length) ??
    (Array.isArray(json.reports) ? json.reports.length : undefined);
  return {
    label: item.label,
    path: item.path,
    freshAt: freshAt.toISOString(),
    ageHours: Number(ageHours.toFixed(2)),
    count,
    status: ageHours <= item.maxHours ? 'ok' : 'stale',
    maxHours: item.maxHours,
  };
}

async function getJson(url) {
  const res = await fetchCompat(url);
  const text = res.text;
  const header = (name) => {
    if (res.headers && typeof res.headers.get === 'function') {
      return res.headers.get(name) || '';
    }
    return '';
  };
  let json = null;
  try {
    json = JSON.parse(text);
  } catch {}
  return {
    ok: res.ok,
    status: res.status,
    source: header('x-data-source'),
    cacheControl: header('cache-control'),
    stale: header('x-data-stale'),
    json,
  };
}

async function fetchCompat(url) {
  if (typeof fetch === 'function') {
    const res = await fetch(url, { cache: 'no-store' });
    return {
      ok: res.ok,
      status: res.status,
      headers: { get: (name) => res.headers.get(name) },
      text: await res.text(),
    };
  }

  return new Promise((resolve, reject) => {
    const parsed = new URL(url);
    const client = parsed.protocol === 'https:' ? https : http;
    const req = client.request(
      parsed,
      {
        method: 'GET',
        timeout: 30000,
        headers: { 'Cache-Control': 'no-store' },
      },
      (res) => {
        const chunks = [];
        res.on('data', (chunk) => chunks.push(chunk));
        res.on('end', () => {
          const headers = Object.fromEntries(
            Object.entries(res.headers).map(([key, value]) => [key.toLowerCase(), Array.isArray(value) ? value.join(', ') : String(value || '')])
          );
          const status = res.statusCode || 0;
          resolve({
            ok: status >= 200 && status < 300,
            status,
            headers: { get: (name) => headers[String(name).toLowerCase()] || '' },
            text: Buffer.concat(chunks).toString('utf8'),
          });
        });
      }
    );
    req.on('error', reject);
    req.on('timeout', () => {
      req.destroy(new Error(`request timeout: ${url}`));
    });
    req.end();
  });
}

function latestUSReportDate(now = new Date()) {
  const et = new Date(now.toLocaleString('en-US', { timeZone: 'America/New_York' }));
  if (et.getHours() < 9) et.setDate(et.getDate() - 1);
  while (et.getDay() === 0 || et.getDay() === 6) et.setDate(et.getDate() - 1);
  return `${et.getFullYear()}-${String(et.getMonth() + 1).padStart(2, '0')}-${String(et.getDate()).padStart(2, '0')}`;
}

function latestReportDate(json) {
  const reports = Array.isArray(json?.reports) ? json.reports : Array.isArray(json) ? json : [];
  return reports.reduce((max, report) => (report?.date > max ? report.date : max), '');
}

async function inspectApi() {
  const checks = [];
  for (const endpoint of [
    '/api/health',
    '/api/market',
    '/api/a-market',
    '/api/macro',
    '/api/premarket-movers',
    '/api/quote?syms=NVDA,SPY,600519',
    '/api/reports?limit=5',
    '/api/etf/sectors',
    '/api/stocks?market=hk&limit=1',
    '/data/market.json',
    '/data/a-market.json',
    '/data/macro.json',
    '/data/premarket-movers.json',
    '/data/us-panel-summary.json',
    '/data/reports.json',
    '/data/etf-analyses.json',
  ]) {
    try {
      const res = await getJson(`${baseUrl}${endpoint}`);
      const staleBlocked =
        (endpoint === '/api/etf/sectors' || endpoint === '/data/etf-analyses.json' || endpoint === '/data/reports.json') &&
        (res.status === 410 || res.status === 503) &&
        res.stale === 'true';
      const contractRejected = endpoint === '/api/stocks?market=hk&limit=1' && res.status === 400;
      const reportDate = endpoint.startsWith('/api/reports') ? latestReportDate(res.json) : '';
      const reportFresh = endpoint.startsWith('/api/reports') ? reportDate >= latestUSReportDate() : true;
      checks.push({
        endpoint,
        ok: res.ok || staleBlocked || contractRejected,
        status: res.status,
        source: res.source,
        cacheControl: res.cacheControl,
        staleBlocked,
        contractRejected,
        reportDate,
        reportFresh,
        quoteSources: res.json?.sources,
        missing: res.json?.missing,
        ts: res.json?.ts,
      });
      if (endpoint.startsWith('/api/reports') && !reportFresh) {
        checks[checks.length - 1].ok = false;
      }
    } catch (error) {
      checks.push({ endpoint, ok: false, error: error instanceof Error ? error.message : String(error) });
    }
  }
  return checks;
}

const fileResults = [];
for (const file of files) {
  try {
    fileResults.push(await inspectFile(file));
  } catch (error) {
    fileResults.push({
      label: file.label,
      path: file.path,
      status: 'missing',
      error: error instanceof Error ? error.message : String(error),
    });
  }
}

const apiResults = checkApi ? await inspectApi() : [];
const stale = fileResults.filter((item) => item.status !== 'ok');
const apiBad = apiResults.filter((item) => !item.ok);

const report = {
  checkedAt: new Date().toISOString(),
  warnHours,
  files: fileResults,
  api: apiResults,
  summary: {
    staleFiles: stale.length,
    apiFailures: apiBad.length,
  },
};

console.log(JSON.stringify(report, null, 2));
if (stale.length || apiBad.length) {
  process.exitCode = 1;
}
