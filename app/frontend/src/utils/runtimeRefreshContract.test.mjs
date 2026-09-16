import assert from 'node:assert/strict';
import fs from 'node:fs';
import test from 'node:test';

const read = (path) => fs.readFileSync(new URL(path, import.meta.url), 'utf8');

test('GPU credentials reload is backend-only and never stores secrets in browser state', () => {
  const hook = read('../hooks/useGPUPrices.ts');
  const page = read('../pages/GPUPrices.tsx');
  assert.match(hook, /gpu-prices\/reload-config/);
  assert.doesNotMatch(hook + page, /localStorage\.setItem|sessionStorage\.setItem/);
  assert.match(page, /页面不接收、不回显也不存储密钥/);
});

test('ETF upstream refresh is explicit, protected, and separate from historical mode', () => {
  const page = read('../pages/ETF.tsx');
  const store = read('../stores/etfStore.ts');
  assert.match(page, /post<\{ status\?: string \}>\('\/api\/etf\/refresh'\)/);
  assert.match(page, /disabled=\{!admin\.authorized \|\| refreshQueuing\}/);
  assert.match(store, /mode=\$\{dataMode\}/);
  assert.doesNotMatch(store, /mode=best/);
});
