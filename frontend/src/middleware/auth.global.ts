import { defineNuxtRouteMiddleware, navigateTo } from '#app'
import { authAccessFromMeta, loginRoute } from '@/lib/authNavigation'
import { BackendProblemError, ensureBackendSession } from '@/lib/backendClient'

const loginRequiredCodes = new Set(['SESSION_EXPIRED', 'SESSION_REVOKED'])

export default defineNuxtRouteMiddleware(async (to) => {
  if (import.meta.server) return

  const access = authAccessFromMeta(to.meta)
  if (!access) return

  try {
    await ensureBackendSession('orbit', access === 'admin', {
      notifySessionInvalidation: false,
    })
  } catch (error) {
    if (
      error instanceof BackendProblemError
      && loginRequiredCodes.has(error.code)
    ) {
      return navigateTo(loginRoute(to.fullPath), { replace: true })
    }
    if (
      error instanceof BackendProblemError
      && error.status === 403
      && error.code === 'PERMISSION_DENIED'
    ) {
      return navigateTo('/', { replace: true })
    }
    throw error
  }
})

