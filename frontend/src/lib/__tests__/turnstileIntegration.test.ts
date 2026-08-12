import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = (relativePath: string) => readFileSync(new URL(relativePath, import.meta.url), 'utf8')

describe('Turnstile authentication integration', () => {
  it('renders the official widget explicitly and clears one-time tokens on every terminal state', () => {
    const widget = source('../../components/auth/TurnstileWidget.vue')

    expect(widget).toContain('https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit')
    expect(widget).toContain('api.render(container.value')
    expect(widget).toContain("'response-field': false")
    expect(widget).toContain("'error-callback': () => clearToken")
    expect(widget).toContain("'expired-callback': () => clearToken")
    expect(widget).toContain("'timeout-callback': () => clearToken")
    expect(widget).toContain('api.reset(widgetId)')
    expect(widget).toContain('api.remove(widgetId)')
  })

  it('requires a password-login token and resets the widget after every submission attempt', () => {
    const login = source('../../pages/LoginPage.vue')
    const client = source('../backendClient.ts')

    expect(client).toMatch(/PasswordLoginRequest[\s\S]*turnstileToken: string/)
    expect(login).toContain("action=\"password_login\"")
    expect(login).toContain("@update:token=\"turnstileToken = $event\"")
    expect(login).toContain("if (!turnstileToken.value)")
    expect(login).toContain('turnstileToken: turnstileToken.value')
    expect(login).toContain(':disabled="passwordLoading || !turnstileToken"')
    expect(login).toMatch(/finally \{[\s\S]*turnstileWidget\.value\?\.reset\(\)/)
  })

  it('keeps the site key public and requires it for production builds', () => {
    const config = source('../../../nuxt.config.ts')

    expect(config).toContain('process.env.NUXT_PUBLIC_TURNSTILE_SITE_KEY')
    expect(config).toContain('public: {')
    expect(config).toContain('turnstileSiteKey')
    expect(config).toContain('Production frontend builds must set NUXT_PUBLIC_TURNSTILE_SITE_KEY.')
    expect(config).not.toContain('TURNSTILE_SECRET')
  })
})
