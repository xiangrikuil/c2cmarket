import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const carpoolPublishSource = readFileSync(new URL('../../pages/CarpoolPublishPage.vue', import.meta.url), 'utf8')
const apiServicePublishSource = readFileSync(new URL('../../pages/ApiServicePublishPage.vue', import.meta.url), 'utf8')

describe('发布完成后的导航', () => {
  it('将车源发布者带到自己的车源列表', () => {
    expect(carpoolPublishSource).toContain("router.replace('/my/carpools')")
  })

  it('普通发布返回服务列表，限时额度入口直接进入单一向导', () => {
    expect(apiServicePublishSource).toContain('apiPublishModeFromQuery(route.query.mode, route.query.after)')
    expect(apiServicePublishSource).toContain("await router.push('/api-market/quota/new')")
    expect(apiServicePublishSource).toContain("await router.replace('/my/api-services')")
    expect(apiServicePublishSource).not.toContain('`/api-market/quota/new?serviceId=${service.id}`')
  })
})
