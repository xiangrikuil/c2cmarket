import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const loginPage = readFileSync(new URL('../../pages/LoginPage.vue', import.meta.url), 'utf8')
const loginPanel = readFileSync(new URL('../../components/auth/LoginPanel.vue', import.meta.url), 'utf8')

describe('password login transition', () => {
  it('keeps the submitted password masked until the login panel unmounts', () => {
    expect(loginPanel).not.toContain("password.value = ''")
    expect(loginPanel).toMatch(/const session = await loginWithPassword[\s\S]*emit\('authenticated', session\)/)
  })

  it('replaces the form with a stable redirect state after authentication', () => {
    expect(loginPage).toContain('const loginRedirecting = ref(false)')
    expect(loginPage).toMatch(/loginRedirecting\.value = true[\s\S]*await router\.push\(returnTo\.value\)/)
    expect(loginPage).toContain('v-else-if="loginRedirecting"')
    expect(loginPage).toContain('aria-busy="true"')
    expect(loginPage).toContain('登录成功')
    expect(loginPage).toContain('正在进入 C2CMarket...')
  })

  it('restores the form with a visible error when navigation fails', () => {
    expect(loginPage).toMatch(/catch \{[\s\S]*loginRedirecting\.value = false[\s\S]*登录已完成，但页面跳转失败，请重试。/)
  })
})
