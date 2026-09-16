import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

const srcRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const read = (...parts) => fs.readFileSync(path.join(srcRoot, ...parts), 'utf8');

test('report cards expose permalinks and deep links fetch the requested report', () => {
  const page = read('pages', 'Reports.tsx');
  const store = read('stores', 'reportsStore.ts');

  assert.match(page, /to=\{`\/reports\/\$\{report\.id\}`\}/);
  assert.match(page, /navigator\.clipboard\.writeText/);
  assert.match(page, /fetchReportById\(routeId\)/);
  assert.match(store, /\/api\/reports\/\$\{encodeURIComponent\(id\)\}/);
});

test('scan applies market and query parameters from inbound links', () => {
  const page = read('pages', 'Scan.tsx');

  assert.match(page, /useSearchParams/);
  assert.match(page, /searchParams\.get\('market'\)/);
  assert.match(page, /searchParams\.get\('q'\)/);
  assert.match(page, /setMarket\(requestedMarket\)/);
  assert.match(page, /setSearchQuery\(requestedQuery\)/);
});

test('congress detail titles are not labeled as 13F holdings', () => {
  const shell = read('components', 'layout', 'StockGodShell.tsx');

  assert.match(shell, /path\.startsWith\('\/whales\/congress\/'\)/);
  assert.match(shell, /\$\{title\}.*\u56fd\u4f1a\u4ea4\u6613\u62ab\u9732/);
});

test('all-routes acceptance covers dynamic details, recovery route, three viewports, and handled 5xx', () => {
  const audit = fs.readFileSync(path.join(srcRoot, '..', 'scripts', 'all-routes-audit.mjs'), 'utf8');

	for (const route of ["'/'", "'/etf/510300'", "'/__dead_page_probe__'"]) assert.ok(audit.includes(route));
	assert.match(audit, /\/api\/reports\?limit=5/);
	assert.match(audit, /\/api\/notes\/toc/);
	assert.match(audit, /name: 'narrow-desktop', width: 1024/);
	assert.match(audit, /server errors are explicitly handled/);
	assert.match(audit, /row\.serverErrors\.length === 0 \|\|/);
});
