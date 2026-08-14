import type { QueryClient } from '@tanstack/vue-query'
import { BackendProblemError, logoutBackendSession } from '@/lib/backendClient'

const alreadyLoggedOutCodes = new Set(['SESSION_EXPIRED', 'SESSION_REVOKED'])

export async function logoutCurrentSession(queryClient: QueryClient) {
  try {
    await logoutBackendSession()
  } catch (error) {
    if (!(error instanceof BackendProblemError) || !alreadyLoggedOutCodes.has(error.code)) throw error
  }

  await queryClient.cancelQueries()
  queryClient.getMutationCache().clear()
  queryClient.clear()
}
