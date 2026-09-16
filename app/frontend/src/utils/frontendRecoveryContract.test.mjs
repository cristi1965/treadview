import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const read = (...parts) => fs.readFileSync(path.join(root, ...parts), 'utf8');

test('routes recover inside their product shell and load pages lazily', () => {
  const routes = read('AppRoutes.tsx');
  assert.match(routes, /React\.lazy/);
  assert.match(routes, /Suspense/);
  assert.match(routes, /RouteErrorBoundary surface="cockpit"/);
  assert.match(routes, /RouteErrorBoundary surface="stockgod"/);
  assert.match(routes, /RouteErrorBoundary surface="notes"/);
});

test('navigation overlays expose modal semantics and cannot leave hidden controls focusable', () => {
  const shell = read('components', 'layout', 'StockGodShell.tsx');
  const layout = read('components', 'layout', 'Layout.tsx');
  assert.match(shell, /role="dialog"/);
  assert.match(shell, /aria-modal="true"/);
  assert.match(shell, /inert=\{!open/);
  assert.match(layout, /role="dialog"/);
  assert.match(layout, /aria-label=\{language === 'zh' \? '\u5173\u95ed\u5bfc\u822a\u83dc\u5355'/);
});

test('route metadata and recovery copy match the visible destination', () => {
  const html = read('..', 'index.html');
  const layout = read('components', 'layout', 'Layout.tsx');
  const notes = read('pages', 'Notes.tsx');
  const staticPage = read('pages', 'StaticPage.tsx');
  assert.match(html, /<html lang="zh-CN">/);
  assert.match(html, /<title>我不是神 · Not a Stock God<\/title>/);
  assert.match(layout, /document\.documentElement\.lang/);
  assert.match(layout, /document\.title/);
  assert.match(notes, /document\.documentElement\.lang/);
  assert.match(staticPage, /to="\/market"[\s\S]{0,120}?stock\.backHeat/);
});

test('Pine helper copy does not claim buy signals or free-tier circumvention', () => {
  const pine = read('components', 'PineScriptModal.tsx');
  assert.doesNotMatch(pine, /Free Tier Buster|\u7834\u9650\u5236|\u4f4e\u5438|\u4e70\u5356\u5171\u632f|\u6b62\u76c8/);
});

test('settings form controls have programmatic labels', () => {
  const settings = read('pages', 'Settings.tsx');
  assert.match(settings, /htmlFor="settings-provider"/);
  assert.match(settings, /id="settings-provider"/);
  assert.match(settings, /htmlFor="settings-debate-rounds"/);
  assert.match(settings, /id="settings-debate-rounds"/);
});
