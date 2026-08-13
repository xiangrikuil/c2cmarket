import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { routes } from '@/router'

const source = (path: string) => readFileSync(new URL(path, import.meta.url), 'utf8')
const pageSource = source('../../pages/RestrictedBusinessPage.vue')
const clientSource = source('../backendClient.ts')
const loginSource = source('../../pages/LoginPage.vue')

describe('账号治理受限业务中心', () => {
  it('注册独立路由并从受限登录结果进入', () => {
    const route = routes.find(item => item.path === '/restricted-business')

    expect(route?.name).toBe('restricted-business')
    expect(route?.meta).toEqual({ standalone: true })
    expect(loginSource).toContain("await router.push('/restricted-business')")
  })

  it('以服务端聚合结果区分保留关系和处置记录', () => {
    expect(pageSource).toContain("getAccountGovernanceBusinessCenter('restricted_business')")
    expect(pageSource).toContain("item.result === 'preserved'")
    expect(pageSource).toContain("item.result !== 'preserved'")
    expect(pageSource).toContain('继续处理')
    expect(pageSource).toContain('处置记录')
    expect(pageSource).not.toContain('pending_payment')
  })

  it('只展示服务端确认的付款申报资格和期限', () => {
    expect(pageSource).toContain('item.paymentClaimEligible')
    expect(pageSource).toContain('item.paymentClaimDeadlineAt')
    expect(pageSource).toContain('付款申报截止')
    expect(pageSource).not.toContain('submitPaymentClaim')
    expect(pageSource).not.toContain('申报已付款')
  })

  it('使用隔离的受限会话、audience header 和退出动作', () => {
    expect(pageSource).toContain('getRestrictedBusinessSession({ forceRefresh: true })')
    expect(pageSource).toContain('logoutRestrictedBusinessSession()')
    expect(clientSource).toContain("headers: { 'X-Session-Audience': audience }")
    expect(clientSource).toContain("'X-Restricted-Business-CSRF': restrictedBusinessCSRFToken")
    expect(clientSource).toContain('cachedRestrictedBusinessSession = null')
    expect(clientSource).toContain('restrictedBusinessCSRFToken = null')
  })
})
