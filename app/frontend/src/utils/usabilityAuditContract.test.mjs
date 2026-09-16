import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

const utilsDir = path.dirname(fileURLToPath(import.meta.url));
const frontendRoot = path.resolve(utilsDir, '../..');
const audit = fs.readFileSync(path.join(frontendRoot, 'scripts', 'usability-audit.mjs'), 'utf8');
const paperAudit = fs.readFileSync(path.join(frontendRoot, 'scripts', 'paper-lifecycle-audit.mjs'), 'utf8');
const paperRunner = fs.readFileSync(path.resolve(frontendRoot, '..', '..', 'scripts', 'verify-paper-browser-runtime.sh'), 'utf8');

test('usability summary cannot count an unexecuted write lifecycle as passed', () => {
	assert.match(audit, /status: 'not-run'/);
	assert.match(audit, /conclusion = .*'partial'/s);
	assert.match(audit, /complete: failed === 0 && partial === 0/);
	assert.doesNotMatch(audit, /!allowPaperWrites \|\| viewport\.name !== 'desktop' \|\| paperLifecycle/);
});

test('Paper runtime acceptance builds current UI and fails on browser errors', () => {
	assert.match(paperRunner, /npm run build/);
	assert.match(paperAudit, /page\.on\('pageerror'/);
	assert.match(paperAudit, /message\.type\(\) === 'error'/);
	assert.match(paperAudit, /failures\.length > 0/);
});

test('usability artifact names dedicated Paper and GPU runtime evidence as authoritative', () => {
	assert.match(audit, /verify-paper-browser-runtime\.sh/);
	assert.match(audit, /verify-gpu-browser-runtime\.sh/);
	assert.match(audit, /writeLifecycleEvidence/);
});
