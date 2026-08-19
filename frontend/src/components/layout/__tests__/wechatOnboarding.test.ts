import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const appShell = readFileSync(new URL('../AppShell.vue', import.meta.url), 'utf8')
const appShellQueries = readFileSync(new URL('../../../queries/useAppShellQueries.ts', import.meta.url), 'utf8')

describe('missing WeChat onboarding', () => {
  it('loads contacts only after authentication and keeps the app-shell query focused', () => {
    expect(appShell).toContain('useMyContactMethodsQuery(isAuthenticated, authenticatedUserId)')
    expect(appShell).not.toContain("from '@/queries/useMarketQueries'")
    expect(appShellQueries).toContain("const { backendMyContactMethods } = await import('@/lib/profileBackend')")
    expect(appShellQueries).toContain("queryKey: computed(() => [...myContactMethodsQueryKey(), valueOf(userId)])")
    expect(appShellQueries).toContain('enabled: computed(() => valueOf(enabled))')
  })

  it('prompts for a missing enabled WeChat and closes after contact state refreshes', () => {
    expect(appShell).toContain("method.enabled && method.type === 'wechat'")
    expect(appShell).toContain('wechatOnboardingOpen.value = !hasWechat')
    expect(appShell).toContain('<DialogTitle>配置微信联系方式</DialogTitle>')
    expect(appShell).toContain('不代表平台已验证该微信号')
    expect(appShell).toContain("await router.push('/my/contacts')")
  })

  it('allows one dismissal per authenticated user and browser session', () => {
    expect(appShell).toContain('c2cmarket.wechat-onboarding-dismissed.v1:${userId}')
    expect(appShell).toContain('window.sessionStorage.getItem(wechatOnboardingStorageKey(userId))')
    expect(appShell).toContain("window.sessionStorage.setItem(wechatOnboardingStorageKey(userId), '1')")
    expect(appShell).toContain('@click="dismissWechatOnboarding">稍后填写</Button>')
  })
})
