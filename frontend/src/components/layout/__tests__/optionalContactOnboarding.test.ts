import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const appShell = readFileSync(new URL('../AppShell.vue', import.meta.url), 'utf8')

describe('optional transaction contact onboarding', () => {
  it('does not query contacts or interrupt navigation from the app shell', () => {
    expect(appShell).not.toContain('useMyContactMethodsQuery')
    expect(appShell).not.toContain('wechatOnboarding')
    expect(appShell).not.toContain('配置微信联系方式')
    expect(appShell).not.toContain('sessionStorage')
  })
})
