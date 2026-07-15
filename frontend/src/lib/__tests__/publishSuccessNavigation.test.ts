import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const carpoolPublishSource = readFileSync(new URL('../../pages/CarpoolPublishPage.vue', import.meta.url), 'utf8')
const apiServicePublishSource = readFileSync(new URL('../../pages/ApiServicePublishPage.vue', import.meta.url), 'utf8')

describe('发布完成后的导航', () => {
  it('将车源发布者带到自己的车源列表', () => {
    expect(carpoolPublishSource).toContain("router.replace('/my/carpools')")
  })

  it('将 API 服务发布者带到自己的服务列表', () => {
    expect(apiServicePublishSource).toContain("router.replace('/my/api-services')")
  })
})
