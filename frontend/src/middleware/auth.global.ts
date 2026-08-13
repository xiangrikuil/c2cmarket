import { defineNuxtRouteMiddleware, navigateTo } from '#app'
import { authAccessFromMeta, loginRoute } from '@/lib/authNavigation'
import { BackendProblemError, ensureBackendSession } from '@/lib/backendClient'
import { capabilityFromRouteMeta, hasCapability } from '@/lib/capabilities'

const loginRequiredCodes = new Set(['SESSION_EXPIRED', 'SESSION_REVOKED'])

export default defineNuxtRouteMiddleware(async (to) => {
  if (import.meta.server) return

  const access = authAccessFromMeta(to.meta)
  if (!access) return

  try {
    const session = await ensureBackendSession('orbit', false, {
      notifySessionInvalidation: false,
    })
    const requiredCapability = capabilityFromRouteMeta(to.meta)
    if (requiredCapability && !hasCapability(session.user, requiredCapability)) {
      return navigateTo({
        path: '/forbidden',
        query: { required: requiredCapability, returnTo: to.fullPath },
      }, { replace: true })
    }
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
      && (error.code === 'PERMISSION_DENIED' || error.code === 'CAPABILITY_REQUIRED')
    ) {
      return navigateTo({ path: '/forbidden', query: { returnTo: to.fullPath } }, { replace: true })
    }
    throw error
  }
})
