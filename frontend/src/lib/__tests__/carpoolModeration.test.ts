import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'vitest'
import { createCarpoolModerationRow, isCarpoolExceptionStatus } from '../carpoolModeration.ts'

test('keeps normal public listings out of the admin exception queue', () => {
  assert.equal(isCarpoolExceptionStatus('可上车'), false)
  assert.equal(isCarpoolExceptionStatus('已满'), false)
  assert.equal(isCarpoolExceptionStatus('已恢复'), false)
  assert.equal(isCarpoolExceptionStatus('暂停'), true)
  assert.equal(isCarpoolExceptionStatus('待复核'), true)
  assert.equal(isCarpoolExceptionStatus('待处理'), true)
})

test('maps a public carpool to the existing admin moderation contract', () => {
  assert.deepEqual(createCarpoolModerationRow({
    id: 'carpool-1',
    product: 'ChatGPT Business',
    region: '日本区',
    monthly: 88,
    status: '可上车',
    owner: 'beifeng',
    trustLevel: 3,
    linuxdoBound: true,
  }), {
    id: 'carpool-1',
    primary: 'ChatGPT Business',
    secondary: '日本区 · ¥88/月 · 可上车',
    owner: 'beifeng · 信任等级3',
    status: '可上车',
    risk: '原帖已绑定',
    targetType: 'carpool',
    targetTo: '/carpools/carpool-1',
  })
})

test('wires public patrol, detail moderation, and admin exception handling', () => {
  const listSource = readFileSync(new URL('../../pages/CarpoolsPage.vue', import.meta.url), 'utf8')
  const detailSource = readFileSync(new URL('../../pages/CarpoolDetailPage.vue', import.meta.url), 'utf8')
  const adminSectionSource = readFileSync(new URL('../../pages/AdminSectionPage.vue', import.meta.url), 'utf8')
  const adminOverviewSource = readFileSync(new URL('../../pages/AdminPage.vue', import.meta.url), 'utf8')

  assert.match(listSource, /useMyProfileQuery/)
  assert.match(listSource, /管理员巡查模式/)
  assert.match(listSource, /@keydown\.enter="openCarpool/)
  assert.match(listSource, /router\.push\(`\/carpools\/\$\{id\}`\)/)

  assert.match(detailSource, /runAdminModerationAction/)
  assert.match(detailSource, /adminConfirmStep\.value !== 'confirm'/)
  assert.match(detailSource, /继续确认/)
  assert.match(detailSource, /queryKey: \['admin-section'\]/)

  assert.match(adminSectionSource, /isCarpoolExceptionStatus/)
  assert.match(adminSectionSource, /车源异常处理/)
  assert.match(adminOverviewSource, /useAdminSectionRows\('carpools'\)/)
  assert.match(adminOverviewSource, /sectionLabel: '车源异常'/)
  assert.doesNotMatch(adminOverviewSource, /carpoolPagination/)
})
