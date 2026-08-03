import { readFileSync } from 'node:fs'
import { describe, expect, expectTypeOf, it } from 'vitest'
import { routes } from '@/router'
import type { Appeal, SelfDispute, SelfReport } from '@/api/generated/openapi'

const source = (path: string) => readFileSync(new URL(path, import.meta.url), 'utf8')
const pageSource = source('../../pages/AccountAppealPage.vue')
const loginSource = source('../../pages/LoginPage.vue')
const clientSource = source('../backendClient.ts')
const seoSource = source('../../seo/routeSeo.ts')

describe('restricted-account appeal frontend', () => {
  it('registers a public standalone route outside the authenticated workspace', () => {
    const route = routes.find(item => item.path === '/account-appeal')

    expect(route?.name).toBe('account-appeal')
    expect(route?.meta).toEqual({ standalone: true })
    expect(route?.meta?.auth).toBeUndefined()
    expect(loginSource).toContain('to="/account-appeal"')
    expect(seoSource).toContain("'/account-appeal'")
  })

  it('uses the dedicated session as truth and clears the callback hint', () => {
    expect(pageSource).toContain('getAccountAppealSession()')
    expect(pageSource).toContain("accountAppealOutcome")
    expect(pageSource).toContain('delete query.accountAppealOutcome')
    expect(pageSource).toContain("outcome === 'ineligible'")
    expect(pageSource).toContain("pageState.value = 'verified'")
    expect(pageSource).not.toContain('getCurrentBackendSession')
    expect(pageSource).not.toContain("'/my")
    expect(pageSource).not.toContain('localStorage')
  })

  it('renders verification, verified form, submitted and error states', () => {
    expect(pageSource).toContain("type PageState = 'loading' | 'verification' | 'verified' | 'submitted' | 'error'")
    expect(pageSource).toContain('使用 linux.do 验证')
    expect(pageSource).toContain('提交复核说明')
    expect(pageSource).toContain('申诉已提交')
    expect(pageSource).toContain('无法继续账号申诉')
    expect(pageSource).toContain('审核通过不会自动恢复账号状态')
  })

  it('keeps restricted-session credentials separate from ordinary auth state', () => {
    expect(clientSource).toContain("'X-Account-Appeal-CSRF': accountAppealCSRFToken")
    expect(clientSource).toContain("idempotencyKey('account-appeal-create')")
    expect(clientSource).toContain('affectsSessionCache: false')
    expect(clientSource).not.toContain('cacheBackendSession(session)\n    accountAppealCSRFToken')
  })

  it('extends only appeal targets with account governance', () => {
    type LegacyTarget = 'contact_snapshot' | 'public_user' | 'carpool_application' | 'carpool_membership' | 'api_purchase_intent' | 'api_order'

    expectTypeOf<SelfReport['targetType']>().toEqualTypeOf<LegacyTarget>()
    expectTypeOf<SelfDispute['targetType']>().toEqualTypeOf<LegacyTarget>()
    expectTypeOf<Appeal['targetType']>().toEqualTypeOf<LegacyTarget | 'account_governance'>()
  })
})
