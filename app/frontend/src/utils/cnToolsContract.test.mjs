import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const etfPage = readFileSync(new URL('../pages/ETF.tsx', import.meta.url), 'utf8');
const routinesPanel = readFileSync(new URL('../components/cn/CnRoutinesPanel.tsx', import.meta.url), 'utf8');
const shell = readFileSync(new URL('../components/layout/StockGodShell.tsx', import.meta.url), 'utf8');

test('the cn-tools hash is a dedicated view and does not fetch stale ETF sectors', () => {
  assert.match(etfPage, /const isCnTools = location\.hash === '#cn-tools'/);
  assert.match(etfPage, /isCnTools[\s\S]*A 股工具箱/);
  assert.match(etfPage, /if \(!isCnTools\)[\s\S]*fetchSectors/);
  assert.match(shell, /location\.hash === '#cn-tools' \? 'A 股工具箱 \| 我不是神'/);
	assert.equal((etfPage.match(/void fetchSectors\(\)/g) || []).length, 1);
	const etfStore = readFileSync(new URL('../stores/etfStore.ts', import.meta.url), 'utf8');
	assert.match(etfStore, /mode=\$\{dataMode\}/);
});

test('stale A-share routines retain a rules-only mode without dynamic outputs', () => {
  assert.match(routinesPanel, /A 股日课 · 规则模式/);
  assert.match(routinesPanel, /不显示价格、今日动作、买卖点或提醒/);
  assert.match(routinesPanel, /profile\?\.rules/);
});
