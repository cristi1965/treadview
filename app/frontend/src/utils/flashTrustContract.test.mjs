import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const store = readFileSync(new URL('../stores/flashStore.ts', import.meta.url), 'utf8');
const feed = readFileSync(new URL('../components/flash/FlashFeed.tsx', import.meta.url), 'utf8');
const copy = readFileSync(new URL('../i18n.ts', import.meta.url), 'utf8');

test('flash feed keeps response freshness metadata instead of treating snapshots as live', () => {
  assert.match(store, /getWithMeta<FlashResponse>/);
  assert.match(store, /degraded: response\.meta\.stale/);
  assert.match(store, /source: response\.meta\.source/);
  assert.match(feed, /statusLabel/);
  assert.match(feed, /Data time|数据时间/);
  assert.match(copy, /'flash\.snapshot': '过期快照'/);
  assert.doesNotMatch(copy, /'flash\.title': '7×24 实时快讯'/);
});

test('experimental tools do not promise unverified live quotes', () => {
  const copilot = readFileSync(new URL('../pages/TradingCopilot.tsx', import.meta.url), 'utf8');
  const multichart = readFileSync(new URL('../pages/MultiChart.tsx', import.meta.url), 'utf8');
  const ashare = readFileSync(new URL('../components/AshareMarketRadar.tsx', import.meta.url), 'utf8');
  assert.doesNotMatch(copilot, /实盘毫秒级行情注入/);
  assert.doesNotMatch(multichart, /实时监控|多图实时盯盘/);
  assert.match(multichart, /来源、延迟和交易时段以图内标记为准/);
  assert.doesNotMatch(ashare, /4分屏实时盯盘|全屏盯盘|全能实战指标/);
  assert.match(ashare, /不纳入本系统实时数据门禁/);
});
