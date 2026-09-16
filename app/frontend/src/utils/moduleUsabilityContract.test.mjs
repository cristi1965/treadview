import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

const srcRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const repoRoot = path.resolve(srcRoot, '../../..');
const read = (...parts) => fs.readFileSync(path.join(srcRoot, ...parts), 'utf8');

test('Copilot has a request budget compatible with the backend LLM deadline', () => {
  const api = read('utils', 'api.ts');
  const copilot = read('pages', 'TradingCopilot.tsx');
  assert.match(api, /options: \{ timeoutMs\?: number \}/);
  assert.match(copilot, /timeoutMs: 95_000/);
});

test('invalid bearer tokens recover through the local HttpOnly session', () => {
  const api = read('utils', 'api.ts');
  const hook = read('hooks', 'useAdminSession.ts');
  assert.match(api, /errorData\?\.error === 'invalid admin token'/);
  assert.match(api, /clearAdminToken\(\);[\s\S]{0,220}?establishLocalAdminSession/);
  assert.match(hook, /ADMIN_SESSION_STATE_EVENT/);
});

test('settings distinguishes saved-but-unavailable LLM configuration', () => {
  const store = read('stores', 'analysisStore.ts');
  const settings = read('pages', 'Settings.tsx');
  assert.match(store, /Promise<ConfigUpdateResult>/);
  assert.match(store, /const result = await resp\.json\(\) as ConfigUpdateResult/);
  assert.match(settings, /updated_with_unavailable_llm/);
  assert.match(settings, /设置已保存，但 LLM 当前不可用/);
});

test('market board protected writes preserve forms and expose failures', () => {
  const page = read('pages', 'MarketBoard.tsx');
  assert.match(page, /setTradeSaveError\(error instanceof Error/);
  assert.match(page, /setEventSaveError\(error instanceof Error/);
  assert.match(page, /role="alert"[\s\S]{0,100}?eventSaveError/);
  assert.match(page, /role="alert"[\s\S]{0,100}?tradeSaveError/);
  assert.match(page, /disabled=\{eventSaving\}/);
  assert.match(page, /disabled=\{tradeSaving\}/);
});

test('lab restores backend running state and websocket start state', () => {
  const store = read('stores', 'analysisStore.ts');
  const page = read('pages', 'ResearchLab.tsx');
  assert.match(store, /syncAnalysisStatus: \(\) => Promise<void>/);
  assert.match(store, /analysis\/status/);
  assert.match(store, /case 'analysis_start':[\s\S]{0,100}?isRunning: true/);
  assert.match(page, /syncAnalysisStatus\(\)\.catch/);
  assert.match(page, /重新同步状态/);
  assert.doesNotMatch(page, /toISOString\(\)\.slice\(0, 10\)/);
  assert.match(page, /getFullYear\(\)/);
});

test('macro has one direct route and dashboard macro has one active sidebar item', () => {
  const routes = read('AppRoutes.tsx');
  const sidebar = read('components', 'layout', 'Sidebar.tsx');
  assert.match(routes, /path="\/macro" element=\{<Navigate to="\/dashboard\/macro" replace \/>\}/);
  assert.match(sidebar, /moreSpecificMatch/);
});

test('portfolio follows batch jobs and offers an explicit status retry', () => {
  const page = read('pages', 'Portfolio.tsx');
  assert.match(page, /analysis\/batch\/\$\{encodeURIComponent\(id\)\}/);
  assert.match(page, /完成 \$\{completed\}\/\$\{total\}/);
  assert.match(page, /检查批量进度/);
  assert.doesNotMatch(page, /看驾驶舱 \/ws/);
});

test('global search routes six-digit and Chinese queries to the CN universe', () => {
  const shell = read('components', 'layout', 'StockGodShell.tsx');
  assert.match(shell, /\^\\d\{6\}\$/);
  assert.match(shell, /market=\$\{market\}/);
});

test('navigation shells expose the same research and market destinations', () => {
  const shell = read('components', 'layout', 'StockGodShell.tsx');
  const sidebar = read('components', 'layout', 'Sidebar.tsx');
  const layout = read('components', 'layout', 'Layout.tsx');
  for (const route of ['/dashboard', '/history', '/lab', '/copilot', '/tactical', '/journal', '/dashboard/macro', '/settings', '/market', '/scan', '/multichart', '/gpu-prices', '/etf', '/whales', '/arena', '/reports', '/notes', '/portfolio']) {
    assert.ok(shell.includes(`path: '${route}'`), `StockGodShell missing ${route}`);
    assert.ok(sidebar.includes(`path: '${route}'`), `Sidebar missing ${route}`);
    assert.ok(layout.includes(`path: '${route}'`), `mobile Layout missing ${route}`);
  }
});

test('research routes do not trigger the global live-market poller', () => {
  const layout = read('components', 'layout', 'Layout.tsx');
  const sync = read('components', 'SystemDataSyncController.tsx');
  for (const route of ['/lab', '/copilot', '/tactical', '/journal']) {
    assert.ok(layout.includes(`'${route}'`), `research surface missing ${route}`);
    assert.ok(sync.includes(`'${route}'`), `research-only sync list missing ${route}`);
  }
});

test('Dashboard history cards deep-link to the selected audited run', () => {
  const dashboard = read('pages', 'Dashboard.tsx');
  const history = read('pages', 'History.tsx');
  assert.match(dashboard, /\/history\?run_id=\$\{encodeURIComponent\(item\.audit\.run_id\)\}/);
  assert.match(history, /searchParams\.get\('run_id'\)/);
  assert.match(history, /item\.audit\?\.run_id === requestedRunID/);
});

test('mobile workspaces use responsive layout, safe areas and direct content focus', () => {
  const journal = read('pages', 'Journal.tsx');
  const notes = read('pages', 'Notes.tsx');
  const copilot = read('pages', 'TradingCopilot.tsx');
  const shell = read('components', 'layout', 'StockGodShell.tsx');
  const pine = read('components', 'PineScriptModal.tsx');
  assert.match(journal, /grid-cols-1[\s\S]{0,80}?lg:grid-cols-/);
  assert.match(notes, /scrollIntoView\(\{ behavior: 'smooth'/);
  assert.match(copilot, /100dvh/);
  assert.match(shell, /safe-area-inset-bottom/);
  assert.match(pine, /max-h-\[100dvh\]/);
});

test('stock detail distinguishes transport failure from an unknown symbol', () => {
  const detail = read('pages', 'StockDetail.tsx');
  assert.match(detail, /setLoadError\(err instanceof Error/);
  assert.ok(detail.includes('这是加载失败，不代表该标的不存在'));
  assert.match(detail, /setRetryVersion\(\(value\) => value \+ 1\)/);
});

test('Journal stores Paper and research provenance received from the handoff', () => {
  const journal = read('pages', 'Journal.tsx');
  const store = read('stores', 'commandStore.ts');
  for (const token of ["searchParams.get('paper_order_id')", "searchParams.get('research_run_id')", 'paperOrderId: sourcePaperOrderID', 'researchRunId: sourceResearchRunID']) {
    assert.ok(journal.includes(token), `Journal provenance missing ${token}`);
  }
  assert.match(store, /paperOrderId\?: string/);
  assert.match(store, /researchRunId\?: string/);
});

test('Notes exposes explicit return paths to both product surfaces', () => {
  const notes = read('pages', 'Notes.tsx');
  assert.match(notes, /to="\/dashboard"/);
  assert.match(notes, /to="\/market"/);
});

test('the launcher uses an IP loopback URL accepted by local session auth', () => {
  const launcher = fs.readFileSync(path.join(repoRoot, 'scripts', 'launch-trading-cockpit.sh'), 'utf8');
  assert.match(launcher, /LOCAL_URL="http:\/\/127\.0\.0\.1:8765"/);
  assert.doesNotMatch(launcher, /LOCAL_URL="http:\/\/localhost:/);
});
