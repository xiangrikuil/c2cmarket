import type { RouteLocationRaw, RouteMeta } from 'vue-router'

export type AuthAccess = 'user' | 'admin'

const internalOrigin = 'https://c2cmarket.local'
export const WECHAT_ONBOARDING_PATH = '/my/account'

export function authAccessFromMeta(meta: RouteMeta | Record<string, unknown>): AuthAccess | null {
  return meta.auth === 'user' || meta.auth === 'admin' ? meta.auth : null
}

export function normalizeReturnTo(value: unknown, fallback = '/') {
  if (typeof value !== 'string') return fallback
  const candidate = value.trim()
  if (!candidate.startsWith('/') || candidate.startsWith('//')) return fallback

  try {
    const url = new URL(candidate, internalOrigin)
    if (url.origin !== internalOrigin) return fallback
    return `${url.pathname}${url.search}${url.hash}`
  } catch {
    return fallback
  }
}

export function loginRoute(returnTo: unknown): RouteLocationRaw {
  return {
    path: '/login',
    query: { returnTo: normalizeReturnTo(returnTo) },
  }
}

export function passwordResetRoute(returnTo: unknown): RouteLocationRaw {
  return {
    path: '/password-reset',
    query: { returnTo: normalizeReturnTo(returnTo) },
  }
}

export function wechatOnboardingReturnTo(value: unknown) {
  const returnTo = normalizeReturnTo(value, '/my')
  const target = new URL(returnTo, internalOrigin)
  if (target.pathname === WECHAT_ONBOARDING_PATH && (!target.search || target.searchParams.get('onboarding') === 'wechat')) {
    return '/my'
  }
  return returnTo
}

export function wechatOnboardingRoute(returnTo: unknown): RouteLocationRaw {
  return {
    path: WECHAT_ONBOARDING_PATH,
    query: {
      onboarding: 'wechat',
      returnTo: wechatOnboardingReturnTo(returnTo),
    },
  }
}

export function createLoginRedirectCoordinator(
  redirect: (location: RouteLocationRaw) => void | Promise<void>,
) {
  let pending: Promise<void> | null = null

  return (returnTo: unknown) => {
    if (pending) return pending
    pending = Promise.resolve(redirect(loginRoute(returnTo)))
      .finally(() => {
        pending = null
      })
    return pending
  }
}
