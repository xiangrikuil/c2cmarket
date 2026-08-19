import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { describe, test } from 'vitest'
import { applyApiServicePackageDraft, cloneApiServicePackageDraft, createDefaultApiServicePackage } from '../packages'

describe('fixed package dialog editor', () => {
  test('keeps dialog edits isolated until the draft is applied', () => {
    const item = createDefaultApiServicePackage(['gpt-5.6'])
    const draft = cloneApiServicePackageDraft(item)

    draft.name = '7 天高额度包'
    draft.modelCatalogIds.push('gpt-5.5')
    draft.quotaUsagePolicy.daily = { mode: 'limited', amountUsd: '20' }

    assert.equal(item.name, '3 天短期流量包')
    assert.deepEqual(item.modelCatalogIds, ['gpt-5.6'])
    assert.deepEqual(item.quotaUsagePolicy.daily, { mode: 'unlimited' })

    applyApiServicePackageDraft(item, draft)

    assert.equal(item.name, '7 天高额度包')
    assert.deepEqual(item.modelCatalogIds, ['gpt-5.6'])
    assert.deepEqual(item.quotaUsagePolicy.daily, { mode: 'limited', amountUsd: '20' })

    draft.quotaUsagePolicy.daily.amountUsd = '30'
    assert.equal(item.quotaUsagePolicy.daily.amountUsd, '20')
  })

  test('uses one responsive dialog instead of an inline editor', () => {
    const source = readFileSync(new URL('../FixedPackageSection.vue', import.meta.url), 'utf8')

    assert.match(source, /<Dialog :open="packageEditorOpen" @update:open="setPackageEditorOpen">/)
    assert.match(source, /<DialogContent class="[^"]*max-h-\[calc\(100dvh-1rem\)\][^"]*overflow-hidden[^"]*sm:max-w-2xl"/)
    assert.match(source, /@click="openPackageEditor\(item\.id\)"/)
    assert.match(source, /@click="setPackageEditorOpen\(false\)"[^>]*>取消<\/Button>/)
    assert.match(source, /@click="savePackage"[^>]*>保存套餐<\/Button>/)
    assert.equal(source.match(/<DialogContent/g)?.length, 1)
    assert.doesNotMatch(source, /v-if="selectedPackage"/)
  })
})
