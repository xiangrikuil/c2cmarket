import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const carpoolPublishSource = readFileSync(new URL('../../pages/CarpoolPublishPage.vue', import.meta.url), 'utf8')
const apiServicePublishSource = readFileSync(new URL('../../pages/ApiServicePublishPage.vue', import.meta.url), 'utf8')

describe('发布完成后的导航', () => {
  it('将车源发布者带到自己的车源列表', () => {
    expect(carpoolPublishSource).toContain("router.replace('/my/carpools')")
  })

  it('普通发布返回服务列表，额度包流程续接固定场次向导', () => {
    expect(apiServicePublishSource).toContain("route.query.after === 'quota'")
    expect(apiServicePublishSource).toContain('`/api-market/quota/new?serviceId=${service.id}`')
    expect(apiServicePublishSource).toContain(": '/my/api-services'")
    expect(apiServicePublishSource).toContain('router.replace(destination)')
  })
})
