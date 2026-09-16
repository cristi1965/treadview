import assert from 'node:assert/strict';
import fs from 'node:fs';
import test from 'node:test';
import ts from 'typescript';

const source = fs.readFileSync(new URL('./dataTrust.ts', import.meta.url), 'utf8');
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
}).outputText;
const module = { exports: {} };
new Function('module', 'exports', compiled)(module, module.exports);
const { assessDataTrust } = module.exports;

const liveMeta = {
  source: 'verified-source',
  dataTime: '2026-09-07T01:00:00Z',
  stale: false,
  staleReason: '',
  partialErrors: [],
};

test('accepts only complete payload with source and parseable data time', () => {
  assert.deepEqual(assessDataTrust(liveMeta, true), { state: 'live', reason: '' });
  assert.equal(assessDataTrust({ ...liveMeta, source: '' }, true).state, 'unavailable');
  assert.equal(assessDataTrust({ ...liveMeta, dataTime: 'unknown' }, true).state, 'unavailable');
  assert.equal(assessDataTrust({ ...liveMeta, dataTime: '2099-01-01T00:00:00Z' }, true).state, 'unavailable');
  assert.equal(assessDataTrust(liveMeta, false).state, 'unavailable');
});

test('fails closed on stale and partial-error responses', () => {
  assert.deepEqual(
    assessDataTrust({ ...liveMeta, stale: true, staleReason: 'snapshot expired' }, true),
    { state: 'stale', reason: 'snapshot expired' },
  );
  const partial = assessDataTrust({ ...liveMeta, partialErrors: ['missing NAV'] }, true);
  assert.equal(partial.state, 'unavailable');
  assert.match(partial.reason, /missing NAV/);
});
