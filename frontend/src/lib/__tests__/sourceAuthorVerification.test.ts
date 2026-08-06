import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'vitest'
import type { Carpool } from '@/data/mock'
import { isCurrentTradable } from '../pricing'
import {
  isSourceAuthorVerified,
  sourceAuthorVerificationLabel,
  sourceAuthorVerificationRank,
} from '../sourceAuthorVerification'

const apiPurchasePanelSource = readFileSync(new URL('../../components/api-service-detail/ApiPurchasePanel.vue', import.meta.url), 'utf8')
const carpoolListSource = readFileSync(new URL('../../pages/CarpoolsPage.vue', import.meta.url), 'utf8')
const carpoolDetailSource = readFileSync(new URL('../../pages/CarpoolDetailPage.vue', import.meta.url), 'utf8')
const carpoolApplicationDetailSource = readFileSync(new URL('../../pages/CarpoolApplicationDetailPage.vue', import.meta.url), 'utf8')
const myCarpoolsSource = readFileSync(new URL('../../pages/MyCarpoolsPage.vue', import.meta.url), 'utf8')
const carpoolPublishSource = readFileSync(new URL('../../pages/CarpoolPublishPage.vue', import.meta.url), 'utf8')

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

test('shared compatibility ranking preserves verification-state ordering', () => {
  assert.ok(sourceAuthorVerificationRank({ status: 'verified' }) > sourceAuthorVerificationRank({ status: 'pending' }))
  assert.ok(sourceAuthorVerificationRank({ status: 'pending' }) > sourceAuthorVerificationRank({ status: 'mismatch' }))
})

test('API detail keeps actionable verification states while carpool surfaces no longer show post verification', () => {
  assert.match(apiPurchasePanelSource, /status === 'verified' \|\| status === 'mismatch'/)
  assert.match(apiPurchasePanelSource, /v-if="showSourceAuthorVerification"[\s\S]*?<SourceAuthorVerificationBadge/)
  for (const source of [carpoolListSource, carpoolDetailSource, carpoolApplicationDetailSource, myCarpoolsSource, carpoolPublishSource]) {
    assert.doesNotMatch(source, /SourceAuthorVerificationBadge|原帖作者|LinuxDoTopicImport|linuxDoTopicUrl|parsedTopicId/)
  }
  assert.match(carpoolDetailSource, /:show-source-author-verification="false"/)
  assert.match(carpoolApplicationDetailSource, /:show-source-author-verification="false"/)
})

test('carpool trade availability does not require a source post', () => {
  const carpool = {
    maxMembers: 4,
    currentConfirmedMembers: 1,
    confirmedWithin48h: true,
    hasInfoConflict: false,
    hasUnresolvedDispute: false,
    status: '可上车',
    sourceAuthorVerification: { status: 'not_submitted' },
  } as Carpool

  assert.equal(isCurrentTradable(carpool), true)
})
