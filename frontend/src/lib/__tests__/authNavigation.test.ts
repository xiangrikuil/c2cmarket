import { readFileSync } from 'node:fs'
import { describe, expect, it, vi } from 'vitest'
import type { RouteLocationRaw } from 'vue-router'
import { routes } from '@/router'
import {
  authAccessFromMeta,
  createLoginRedirectCoordinator,
  loginRoute,
  normalizeReturnTo,
  passwordResetRoute,
} from '@/lib/authNavigation'

describe('authentication navigation', () => {
  it('preserves internal path, query, and hash in returnTo', () => {
    const returnTo = '/api-market/quota/new?serviceId=service-1&copy=1#payment'

    expect(normalizeReturnTo(returnTo)).toBe(returnTo)
    expect(loginRoute(returnTo)).toEqual({
      path: '/login',
      query: { returnTo },
    })
    expect(passwordResetRoute(returnTo)).toEqual({
      path: '/password-reset',
      query: { returnTo },
    })
  })

  it.each([
    undefined,
    '',
    'https://example.com/account',
    '//example.com/account',
    '/\\example.com/account',
  ])('rejects unsafe returnTo value %s', (value) => {
    expect(normalizeReturnTo(value)).toBe('/')
  })

  it('reads only supported route access metadata', () => {
    expect(authAccessFromMeta({ auth: 'user' })).toBe('user')
    expect(authAccessFromMeta({ auth: 'admin' })).toBe('admin')
    expect(authAccessFromMeta({ auth: 'owner' })).toBeNull()
    expect(authAccessFromMeta({})).toBeNull()
  })

  it('coalesces concurrent session-invalidated redirects', async () => {
    let completeRedirect = () => {}
    const redirectPending = new Promise<void>((resolve) => {
      completeRedirect = resolve
    })
    const redirect = vi.fn((_location: RouteLocationRaw) => redirectPending)
    const redirectToLogin = createLoginRedirectCoordinator(redirect)

    const first = redirectToLogin('/my/api-orders?status=pending#latest')
    const second = redirectToLogin('/merchant/api-orders')

    expect(first).toBe(second)
    expect(redirect).toHaveBeenCalledTimes(1)
    expect(redirect).toHaveBeenCalledWith({
      path: '/login',
      query: { returnTo: '/my/api-orders?status=pending#latest' },
    })

    completeRedirect()
    await first
  })
})

describe('route authentication contract', () => {
  const routeByPath = new Map(routes.map(route => [route.path, route]))

  it('marks every account workspace and publish entry as user-only', () => {
    const protectedPaths = [
      '/carpools/new',
      '/api-market/new',
      '/api-market/quota/new',
      '/api-intents/:id',
    ]
    for (const path of protectedPaths) {
      expect(authAccessFromMeta(routeByPath.get(path)?.meta ?? {}), path).toBe('user')
    }

    for (const route of routes.filter(route =>
      route.path === '/my'
      || route.path.startsWith('/my/')
      || route.path.startsWith('/merchant/'),
    )) {
      expect(authAccessFromMeta(route.meta ?? {}), route.path).toBe('user')
    }
  })

  it('marks every admin route as admin-only', () => {
    for (const route of routes.filter(route => route.path === '/admin' || route.path.startsWith('/admin/'))) {
      expect(authAccessFromMeta(route.meta ?? {}), route.path).toBe('admin')
    }
  })

  it('keeps public market, announcement, and profile routes anonymous', () => {
    for (const path of [
      '/',
      '/search',
      '/official-prices',
      '/carpools',
      '/carpools/:id',
      '/api-market',
      '/api-market/:id',
      '/announcements/:slug',
      '/u/:username',
      '/password-reset',
    ]) {
      expect(authAccessFromMeta(routeByPath.get(path)?.meta ?? {}), path).toBeNull()
    }
  })

  it('runs the global guard before protected pages render', () => {
    const source = readFileSync(new URL('../../middleware/auth.global.ts', import.meta.url), 'utf8')

    expect(source).toContain('authAccessFromMeta(to.meta)')
    expect(source).toContain("ensureBackendSession('orbit', false")
    expect(source).toContain('capabilityFromRouteMeta(to.meta)')
    expect(source).toContain('!hasCapability(session.user, requiredCapability)')
    expect(source).toContain('notifySessionInvalidation: false')
    expect(source).toContain('navigateTo(loginRoute(to.fullPath), { replace: true })')
    expect(source).toContain("error.code === 'PERMISSION_DENIED' || error.code === 'CAPABILITY_REQUIRED'")
  })
})
