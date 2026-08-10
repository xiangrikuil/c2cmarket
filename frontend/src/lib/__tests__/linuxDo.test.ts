import assert from 'node:assert/strict'
import { test } from 'vitest'
import { linuxDoProfileSummaryUrl } from '../linuxDo'

test('builds linux.do contact links with the profile summary route', () => {
  assert.equal(linuxDoProfileSummaryUrl('12345'), 'https://linux.do/u/12345/summary')
  assert.equal(linuxDoProfileSummaryUrl('@seller'), 'https://linux.do/u/seller/summary')
})
