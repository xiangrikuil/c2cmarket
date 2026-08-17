import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'vitest'
import { formatDailyWeeklyQuota } from '../quota'

const listSource = readFileSync(new URL('../../pages/CarpoolsPage.vue', import.meta.url), 'utf8')
const detailSource = readFileSync(new URL('../../pages/CarpoolDetailPage.vue', import.meta.url), 'utf8')
const publishRulesSource = readFileSync(new URL('../../components/carpool-publish/CarpoolRulesEditor.vue', import.meta.url), 'utf8')

test('formats required daily and weekly quota as one compact line', () => {
  assert.equal(formatDailyWeeklyQuota({ dailyQuotaAmount: 50, weeklyQuotaAmount: 200, quotaUnit: 'USD' }), '每日最大花费 $50 · 每周最大花费 $200')
  assert.equal(formatDailyWeeklyQuota({ weeklyQuotaAmount: 200, quotaUnit: 'USD' }), '每日最大花费 未声明 · 每周最大花费 $200')
  assert.equal(formatDailyWeeklyQuota({}), '每日最大花费 未声明 · 每周最大花费 未声明')
})

test('carpool market uses the approved shadcn quota and access column', () => {
  assert.match(listSource, /\['车源', '价格', '车位', '额度 \/ 接入', '车主', '状态'\]/)
  assert.match(listSource, /<Popover>/)
  assert.match(listSource, /<PopoverTrigger as-child>/)
  assert.match(listSource, /<PopoverContent/)
  assert.match(listSource, /formatDailyWeeklyQuota\(row\)/)
  assert.match(listSource, /跟随官方重置/)
  assert.match(listSource, /adminAccountLabel\(row\.providesAdminAccount\)/)
  assert.match(listSource, /具体权限与使用细节请站外确认，平台不保存管理员凭据。/)
  assert.doesNotMatch(listSource, /开通信息/)
})

test('carpool detail exposes usage signals without a multiplier row', () => {
  for (const label of ['每日最大花费额度 / 每周最大花费额度', '额度重置', 'VPS 区域', '国内直连', '开通渠道', '付款方式', '管理员账号']) {
    assert.match(detailSource, new RegExp(label))
  }
  assert.doesNotMatch(detailSource, /<span class="text-muted-foreground">倍率<\/span>/)
})

test('carpool publish guidance describes daily and weekly quota without exposing a multiplier', () => {
  assert.match(publishRulesSource, /每日与每周最大花费额度/)
  assert.doesNotMatch(publishRulesSource, /倍率/)
})
