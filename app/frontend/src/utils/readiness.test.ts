import assert from 'node:assert/strict'
import test from 'node:test'

import { normalizeHistoricalReadiness } from './readiness'

test('rejects readiness error payloads from an older backend', () => {
  assert.equal(
    normalizeHistoricalReadiness({ error: 'Not Found' }, 404),
    null,
  )
})

test('accepts readiness only when checks is an array', () => {
  assert.equal(
    normalizeHistoricalReadiness({ ready: false, checks: undefined }, 200),
    null,
  )

  const readiness = normalizeHistoricalReadiness(
    {
      profile: 'historical-research',
      status: 'blocked',
      ready: false,
      checks: [{ id: 'history', status: 'fail' }],
      disclaimer: 'Historical research only.',
    },
    503,
  )

  assert.equal(readiness?.checks.length, 1)
  assert.equal(readiness?.httpStatus, 503)
})
