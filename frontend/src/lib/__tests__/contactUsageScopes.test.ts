import { describe, expect, it } from 'vitest'
import type { UserContactMethod } from '@/lib/api'
import {
  buildContactMethodPayload,
  contactUsageScopeOptionsForCapabilities,
  initialContactUsageScopes,
  normalizeContactUsageScopes,
  sameContactUsageScopes,
} from '@/lib/contactUsageScopes'

const currentWechat: UserContactMethod = {
  id: 'contact-wechat',
  userId: 'user-1',
  type: 'wechat',
  label: '微信',
  maskedValue: 'wx***23',
  displayValue: 'wx_user_123',
  usageScopes: ['buyer', 'dispute'],
  isDefault: false,
  enabled: true,
  verified: true,
  createdAt: '2026-08-01T00:00:00Z',
  updatedAt: '2026-08-01T00:00:00Z',
}

describe('联系方式适用场景', () => {
  it('只向具备对应发布能力的账号提供商户用途', () => {
    expect(contactUsageScopeOptionsForCapabilities({
      canPublishCarpool: false,
      canPublishApiService: true,
    }).map(option => option.value)).toEqual(['api_merchant', 'buyer', 'dispute'])

    expect(contactUsageScopeOptionsForCapabilities({
      canPublishCarpool: false,
      canPublishApiService: false,
    }).map(option => option.value)).toEqual(['buyer', 'dispute'])
  })

  it('加载已有微信时保留原用途，不自动扩大到 API 商户', () => {
    expect(initialContactUsageScopes(currentWechat, ['api_merchant', 'buyer', 'dispute']))
      .toEqual(['buyer', 'dispute'])
  })

  it('用途比较忽略勾选顺序但能识别新增 API 商户用途', () => {
    expect(sameContactUsageScopes(['buyer', 'dispute'], ['dispute', 'buyer'])).toBe(true)
    expect(sameContactUsageScopes(['api_merchant', 'buyer', 'dispute'], ['buyer', 'dispute'])).toBe(false)
  })

  it('保存载荷使用显式草稿用途并按契约顺序去重', () => {
    expect(buildContactMethodPayload({
      type: 'wechat',
      label: '微信',
      displayValue: ' wx_user_123 ',
      usageScopes: ['buyer', 'api_merchant', 'api_merchant', 'dispute'],
      current: currentWechat,
    })).toMatchObject({
      displayValue: 'wx_user_123',
      usageScopes: ['api_merchant', 'buyer', 'dispute'],
      enabled: true,
    })
    expect(normalizeContactUsageScopes(['dispute', 'api_merchant', 'buyer']))
      .toEqual(['api_merchant', 'buyer', 'dispute'])
  })
})
