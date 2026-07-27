import assert from 'node:assert/strict'
import { afterEach, test, vi } from 'vitest'

type ProfileBackendModule = typeof import('../profileBackend')

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': 'application/json' },
  })
}

async function loadProfileBackend(config: { apiMode?: string, apiBaseUrl?: string } = {}): Promise<ProfileBackendModule> {
  vi.resetModules()
  const client = await import('../backendClient')
  client.setBackendRuntimeConfig(config)
  return import('../profileBackend')
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  vi.resetModules()
})

test('backend profile mapper normalizes nullable array fields', async () => {
  const fetchMock = vi.fn()
  vi.stubGlobal('fetch', fetchMock)
  fetchMock
    .mockResolvedValueOnce(jsonResponse({
      csrfToken: 'csrf-profile',
      expiresAt: '2999-01-01T00:00:00Z',
      user: {
        id: 'user-1',
        username: 'orbit',
        displayName: 'Orbit',
        isAdmin: false,
        permissions: [],
        linuxDoBinding: { bound: true, linuxDoUsername: 'orbit', trustLevel: 3 },
      },
    }))
    .mockResolvedValueOnce(jsonResponse({
      id: 'user-1',
      username: 'orbit',
      displayName: 'Orbit',
      bio: null,
      avatarUrl: null,
      customAvatarUrl: null,
      email: null,
      emailVerified: false,
      emailVerifiedAt: null,
      passwordConfigured: false,
      regionCode: null,
      timezone: null,
      avatarMode: 'linuxdo',
      accountStatus: 'normal',
      permissions: [],
      linuxDoBinding: {
        bound: true,
        linuxDoUserId: '1024',
        linuxDoUsername: 'orbit',
        linuxDoAvatarUrl: null,
        trustLevel: 3,
        lastSyncedAt: null,
      },
      badges: null,
      restrictions: null,
      usernameChangePolicy: {
        canChange: true,
        nextAvailableAt: null,
      },
      privacy: {
        showCreatedAt: true,
        showLastActiveAt: true,
        showCompletedCarpoolCount: true,
        showCompletedApiIntentCount: true,
        showResponseMedian: true,
        showResolvedDisputeSummary: true,
        allowPublicProfileReport: true,
      },
      createdAt: '2026-07-07T00:00:00Z',
      updatedAt: '2026-07-07T00:00:00Z',
      lastActiveAt: null,
    }))

  const { backendMyProfile } = await loadProfileBackend({ apiMode: 'real' })
  const profile = await backendMyProfile()

  assert.deepEqual(profile.badges, [])
  assert.deepEqual(profile.restrictions, [])
  assert.equal(fetchMock.mock.calls[0]?.[0], '/api/v1/auth/session')
  assert.equal(fetchMock.mock.calls[1]?.[0], '/api/v1/me/profile')
})

test('public profile adapter preserves unavailable reputation facts as null', async () => {
  const fetchMock = vi.fn((input: RequestInfo | URL) => {
    const url = String(input)
    if (url.endsWith('/reviews')) {
      return Promise.resolve(jsonResponse({ items: [] }))
    }
    return Promise.resolve(jsonResponse({
      profile: {
        id: '11111111-1111-4111-8111-111111111111',
        username: 'truthful',
        displayName: 'Truthful',
        bio: null,
        avatarUrl: null,
        avatarText: 'T',
        linuxDoBound: false,
        linuxDoUsername: null,
        trustLevel: null,
        badges: [],
        accountStatus: 'normal',
        createdAt: null,
        lastActiveAt: null,
        stats: {
          completedCarpools: null,
          completedApiOrders: null,
          completedCarpoolsLast90Days: null,
          completedApiOrdersLast90Days: null,
          responseMedianMinutes: null,
          buyerResponsibilityCancellationCount: null,
          sellerResponsibilityCancellationCount: null,
          unknownResponsibilityCancellationCount: null,
          unresolvedDisputeCount: null,
          resolvedDisputeCountLast90Days: null,
        },
        privacy: {
          showCreatedAt: true,
          showLastActiveAt: true,
          showCompletionStats: true,
          showResponseMedian: true,
          showResolvedDisputeSummary: true,
          allowPublicProfileReport: true,
        },
      },
      carpools: [],
      services: [],
      completions: [],
      reviews: [],
      disputes: [],
    }))
  })
  vi.stubGlobal('fetch', fetchMock)

  const { backendPublicUserProfile } = await loadProfileBackend({ apiMode: 'real' })
  const result = await backendPublicUserProfile('truthful')

  assert.deepEqual(result.reputations, [])
  assert.equal(result.profile.trustLevel, null)
  assert.equal(result.profile.stats.completedCarpoolsLast90Days, null)
  assert.equal(result.profile.stats.buyerResponsibilityCancellationCount, null)
  assert.equal(result.profile.stats.unresolvedDisputeCount, null)
})
