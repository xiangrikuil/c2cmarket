import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'vitest'
import {
  isApiServiceAdminActionStatus,
  isApiServiceExceptionStatus,
  isApiServicePublicStatus,
} from '../apiServiceModeration.ts'

test('separates public API services from exception history and actionable exceptions', () => {
  assert.equal(isApiServicePublicStatus('在线'), true)
  assert.equal(isApiServicePublicStatus('暂停'), false)
  assert.equal(isApiServicePublicStatus('已通过'), false)

  for (const status of ['待处理', '待复核', '已下架', '已拒绝', '已移除']) {
    assert.equal(isApiServiceExceptionStatus(status), true)
  }
  for (const status of ['在线', '暂停', '已通过', '草稿']) {
    assert.equal(isApiServiceExceptionStatus(status), false)
  }

  assert.equal(isApiServiceAdminActionStatus('待处理'), true)
  assert.equal(isApiServiceAdminActionStatus('已下架'), true)
  assert.equal(isApiServiceAdminActionStatus('待复核'), false)
  assert.equal(isApiServiceAdminActionStatus('在线'), false)
})

test('wires API service management views and exception tasks', () => {
  const sectionSource = readFileSync(new URL('../../pages/AdminSectionPage.vue', import.meta.url), 'utf8')
  const overviewSource = readFileSync(new URL('../../pages/AdminPage.vue', import.meta.url), 'utf8')

  assert.match(sectionSource, /API 服务管理/)
  assert.match(sectionSource, /TabsTrigger value="public">公开服务/)
  assert.match(sectionSource, /TabsTrigger value="exceptions">异常服务/)
  assert.match(sectionSource, /isApiServicePublicStatus/)
  assert.match(sectionSource, /isApiServiceExceptionStatus/)
  assert.match(overviewSource, /isApiServiceAdminActionStatus/)
  assert.match(overviewSource, /sectionLabel: '服务异常'/)
  assert.match(overviewSource, /\/admin\/api-services\?view=exceptions/)
  assert.doesNotMatch(overviewSource, /待审核服务/)
})
