import assert from 'node:assert/strict';
import fs from 'node:fs';
import test from 'node:test';

const journal = fs.readFileSync(new URL('../pages/Journal.tsx', import.meta.url), 'utf8');
const i18n = fs.readFileSync(new URL('../i18n.ts', import.meta.url), 'utf8');
const copilot = fs.readFileSync(new URL('../pages/TradingCopilot.tsx', import.meta.url), 'utf8');
const portfolio = fs.readFileSync(new URL('../pages/Portfolio.tsx', import.meta.url), 'utf8');
const backendPrompt = fs.readFileSync(new URL('../../../backend/internal/api/chat_handlers.go', import.meta.url), 'utf8');
const paperClient = fs.readFileSync(new URL('./paperOrders.ts', import.meta.url), 'utf8');
const commandStore = fs.readFileSync(new URL('../stores/commandStore.ts', import.meta.url), 'utf8');
const commandHandler = fs.readFileSync(new URL('../../../backend/internal/api/command_center_handlers.go', import.meta.url), 'utf8');
const router = fs.readFileSync(new URL('../../../backend/internal/api/router.go', import.meta.url), 'utf8');

test('all user-facing transaction workflows are explicitly Paper-only', () => {
  assert.ok(journal.includes('PAPER ONLY'));
  assert.ok(i18n.includes('Paper 模拟交易复盘'));
  assert.doesNotMatch(i18n, /每笔真实或模拟交易|Log live or paper trades/);
  assert.ok(copilot.includes('所有条件单与交易计划仅用于 Paper 模拟'));
  assert.ok(portfolio.includes('本地仅记录 Paper 情景仓位'));
});

test('AI execution guidance and order client cannot escape Paper simulation', () => {
  assert.ok(backendPrompt.includes('只能用于本系统 Paper 模拟'));
  assert.doesNotMatch(backendPrompt, /在券商 App（富途\/\u8001虎/);
  assert.match(paperClient, /environment: 'PAPER'/);
  assert.doesNotMatch(paperClient, /broker|alpaca|interactive.?brokers/i);
  assert.match(commandStore, /apiPost\('\/api\/trades', \{ \.\.\.tradeData, environment: 'PAPER' \}\)/);
  assert.match(commandHandler, /Environment: paperEnvironment/);
  assert.match(router, /adminWriteGuard\(cfg, "paper-journal\.create"\)/);
});
