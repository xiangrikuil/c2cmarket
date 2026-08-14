import { describe, expect, it } from 'vitest'
import {
  parseSentryEnabled,
  parseSentrySampleRate,
  sanitizeSentryBreadcrumb,
  sanitizeSentryEvent,
  sanitizeSentryURL,
} from '../sentryPrivacy'

describe('Sentry privacy helpers', () => {
  it('removes query strings and fragments from URLs', () => {
    expect(sanitizeSentryURL('https://c2cmarket.shop/password-reset?token=secret#confirm'))
      .toBe('https://c2cmarket.shop/password-reset')
    expect(sanitizeSentryURL('/api/v1/search?q=private')).toBe('/api/v1/search')
  })

  it('removes request and user data from events', () => {
    const event = sanitizeSentryEvent({
      user: { email: 'student@example.test' },
      request: {
        url: 'https://api.c2cmarket.shop/api/v1/search?q=private',
        query_string: 'q=private',
        data: { password: 'secret' },
        cookies: 'session=secret',
        headers: { Authorization: 'Bearer secret' },
        env: { REMOTE_ADDR: '127.0.0.1' },
      },
    })

    expect(event.user).toBeUndefined()
    expect(event.request).toEqual({
      url: 'https://api.c2cmarket.shop/api/v1/search',
      query_string: undefined,
      data: undefined,
      cookies: undefined,
      headers: undefined,
      env: undefined,
    })
  })

  it('redacts sensitive breadcrumb fields', () => {
    const breadcrumb = sanitizeSentryBreadcrumb({
      data: {
        url: 'https://api.c2cmarket.shop/api/v1/search?q=private',
        method: 'GET',
        authorization: 'Bearer secret',
        requestBody: '{"password":"secret"}',
      },
    })

    expect(breadcrumb.data).toEqual({
      url: 'https://api.c2cmarket.shop/api/v1/search',
      method: 'GET',
    })
  })

  it('parses runtime toggles and bounded sample rates', () => {
    expect(parseSentryEnabled('true')).toBe(true)
    expect(parseSentryEnabled('false')).toBe(false)
    expect(parseSentrySampleRate('0.25', 0.05)).toBe(0.25)
    expect(parseSentrySampleRate('2', 0.05)).toBe(0.05)
  })
})
