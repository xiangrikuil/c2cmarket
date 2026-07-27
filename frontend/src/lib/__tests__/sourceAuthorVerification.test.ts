import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'vitest'
import {
  isSourceAuthorVerified,
  sourceAuthorVerificationLabel,
  sourceAuthorVerificationRank,
} from '../sourceAuthorVerification'

const apiPurchasePanelSource = readFileSync(new URL('../../components/api-service-detail/ApiPurchasePanel.vue', import.meta.url), 'utf8')
const carpoolListSource = readFileSync(new URL('../../pages/CarpoolsPage.vue', import.meta.url), 'utf8')
const carpoolDetailSource = readFileSync(new URL('../../pages/CarpoolDetailPage.vue', import.meta.url), 'utf8')

test('source-author labels expose every effective backend state truthfully', () => {
  assert.equal(sourceAuthorVerificationLabel(undefined), '原帖作者未验证')
  assert.equal(sourceAuthorVerificationLabel({ status: 'pending' }), '原帖作者待核验')
  assert.equal(sourceAuthorVerificationLabel({ status: 'verified' }), '原帖作者已验证')
  assert.equal(sourceAuthorVerificationLabel({ status: 'mismatch' }), '原帖作者不一致')
  assert.equal(sourceAuthorVerificationLabel({ status: 'expired' }), '原帖作者验证已过期')
})

test('only verified status enables the verified trust signal', () => {
  for (const status of ['not_submitted', 'pending', 'mismatch', 'expired'] as const) {
    assert.equal(isSourceAuthorVerified({ status }), false)
  }
  assert.equal(isSourceAuthorVerified({ status: 'verified' }), true)
})

test('recommended sorting prioritizes verified status without hiding risk states', () => {
  assert.ok(sourceAuthorVerificationRank({ status: 'verified' }) > sourceAuthorVerificationRank({ status: 'pending' }))
  assert.ok(sourceAuthorVerificationRank({ status: 'pending' }) > sourceAuthorVerificationRank({ status: 'mismatch' }))
})

test('API detail only surfaces actionable verification states while carpool keeps the full badge', () => {
  assert.match(apiPurchasePanelSource, /status === 'verified' \|\| status === 'mismatch'/)
  assert.match(apiPurchasePanelSource, /v-if="showSourceAuthorVerification"[\s\S]*?<SourceAuthorVerificationBadge/)
  assert.match(carpoolListSource, /<SourceAuthorVerificationBadge :verification="row\.sourceAuthorVerification"/)
  assert.match(carpoolDetailSource, /<SourceAuthorVerificationBadge :verification="carpool\.sourceAuthorVerification"/)
})
