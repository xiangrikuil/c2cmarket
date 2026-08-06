import assert from 'node:assert/strict'
import { test } from 'vitest'
import { requireApiMode } from '../apiMode'

test('accepts only explicit real and mock API modes', () => {
  assert.equal(requireApiMode('real'), 'real')
  assert.equal(requireApiMode(' mock '), 'mock')

  for (const value of [undefined, null, '', 'development', 'REAL']) {
    assert.throws(
      () => requireApiMode(value),
      /NUXT_PUBLIC_API_MODE must be explicitly set to "real" or "mock"/,
    )
  }
})
