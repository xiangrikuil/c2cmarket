import {
  BackendProblemError,
  backendMutation,
  backendRequest,
  ensureBackendSession,
  requireBackendSession,
} from '@/lib/backendClient'
import type {
  PublicDisputeRecord,
  PublicProfileAPIService,
  PublicProfileCarpool,
  PublicProfileCompletion,
  PublicReviewRecord,
} from '@/data/mock'
import type {
  ContactMethodType,
  PublicMerchantProfile,
  PublicUserProfile,
  SaveContactMethodRequest,
  UpdateMyProfileRequest,
  UserContactMethod,
  UserProfile,
} from '@/lib/api'
import type { ReputationSnapshot } from '@/types/reputation'
import { mapBackendReputationSnapshot } from '@/lib/reputationBackend'
import { normalizeCapabilities } from '@/lib/capabilities'

type ListResponse<T> = {
  items: T[]
  nextCursor?: string | null
}

type BackendPrivacy = {
  showCreatedAt: boolean
  showLastActiveAt: boolean
  showCompletedCarpoolCount: boolean
  showCompletedApiIntentCount: boolean
  showResponseMedian: boolean
  showResolvedDisputeSummary: boolean
  allowPublicProfileReport: boolean
}

type BackendProfile = {
  id: string
  username: string
  displayName: string
  bio: string | null
  avatarUrl: string | null
  customAvatarUrl: string | null
  email: string | null
  emailVerified: boolean
  emailVerifiedAt: string | null
  passwordConfigured: boolean
  regionCode: string | null
  timezone: string | null
  avatarMode: 'linuxdo' | 'custom_url'
  accountStatus: string
  permissions: Array<'admin'>
  capabilities: string[]
  linuxDoBinding: {
    bound: boolean
    linuxDoUserId: string | null
    linuxDoUsername: string | null
    linuxDoAvatarUrl: string | null
    trustLevel: number | null
    lastSyncedAt: string | null
  }
  badges: string[] | null
  restrictions: string[] | null
  usernameChangePolicy: {
    canChange: boolean
    nextAvailableAt: string | null
  }
  privacy: BackendPrivacy
  createdAt: string
  updatedAt: string
  lastActiveAt: string | null
}

type BackendEmailVerificationChallenge = {
  email: string
  expiresAt: string
  devCode?: string
}

export type BackendContactEmailVerificationChallenge = {
  contactMethodId: string
  contactMethodVersionId: string
  email: string
  expiresAt: string
  devCode?: string
}

type BackendContact = {
  id: string
  userId: string
  type: ContactMethodType
  label: string
  maskedValue: string
  displayValue?: string
  usageScopes: UserContactMethod['usageScopes']
  isDefault: boolean
  enabled: boolean
  verified: boolean
  createdAt: string
  updatedAt: string
}

export type BackendMerchantProfile = {
  id: string
  ownerUserId?: string
  slug: string
  displayName: string
  avatarUrl: string | null
  status: string
  createdAt: string
  updatedAt: string
  version: number
}

type BackendPublicUserProfile = Omit<PublicUserProfile, 'badges' | 'privacy'> & {
  badges: string[]
  privacy: BackendPrivacy
}

type BackendPublicUserProfileBundle = {
  profile: BackendPublicUserProfile
  reputations?: ReputationSnapshot[] | null
  carpools: PublicProfileCarpool[]
  services: PublicProfileAPIService[]
  completions: PublicProfileCompletion[]
  reviews: PublicReviewRecord[]
  disputes: PublicDisputeRecord[]
}

export async function backendMyProfile(): Promise<UserProfile> {
  await requireBackendSession()
  return mapProfile(await backendRequest<BackendProfile>('/api/v1/me/profile'))
}

export async function backendUpdateMyProfile(payload: UpdateMyProfileRequest): Promise<UserProfile> {
  const response = await backendMutation<BackendProfile>('/api/v1/me/profile', {
    displayName: payload.displayName,
    username: payload.username,
    bio: payload.bio ?? '',
    regionCode: payload.regionCode ?? '',
    timezone: payload.timezone ?? '',
    avatarMode: payload.avatarMode,
    avatarUrl: payload.avatarUrl ?? '',
    privacy: payload.privacy ? toBackendPrivacy(payload.privacy) : undefined,
  }, { method: 'PATCH' })
  return mapProfile(response)
}

export async function backendUseLinuxDoAvatar(): Promise<UserProfile> {
  const current = await backendMyProfile()
  return backendUpdateMyProfile({
    displayName: current.displayName,
    username: current.username,
    bio: current.bio,
    regionCode: current.regionCode,
    timezone: current.timezone,
    avatarMode: 'linuxdo',
    avatarUrl: null,
    privacy: current.privacy,
  })
}

export async function backendSetPassword(payload: { currentPassword?: string, newPassword: string }): Promise<void> {
  await backendMutation<void>('/api/v1/auth/password', {
    currentPassword: payload.currentPassword ?? '',
    newPassword: payload.newPassword,
  })
}

export async function backendStartEmailVerification(email: string): Promise<BackendEmailVerificationChallenge> {
  return backendMutation<BackendEmailVerificationChallenge>('/api/v1/me/email-verification/start', { email })
}

export async function backendConfirmEmailVerification(payload: { email: string, code: string }): Promise<UserProfile> {
  return mapProfile(await backendMutation<BackendProfile>('/api/v1/me/email-verification/confirm', payload))
}

export async function backendMyContactMethods(): Promise<UserContactMethod[]> {
  await requireBackendSession()
  const response = await backendRequest<ListResponse<BackendContact>>('/api/v1/me/contact-methods')
  return response.items.map(item => mapContact(item))
}

export async function backendCreateContact(payload: SaveContactMethodRequest): Promise<UserContactMethod> {
  const response = await backendMutation<BackendContact>('/api/v1/contact-methods', toContactPayload(payload), {
    idempotencyPrefix: 'profile-contact',
  })
  return mapContact(response, payload.displayValue)
}

export async function backendUpdateContact(contactId: string, payload: SaveContactMethodRequest): Promise<UserContactMethod> {
  const response = await backendMutation<BackendContact>(`/api/v1/contact-methods/${contactId}`, toContactPayload(payload), {
    method: 'PATCH',
    idempotencyPrefix: 'profile-contact-update',
  })
  return mapContact(response, payload.displayValue)
}

export async function backendDeleteContact(contactId: string): Promise<UserContactMethod> {
  return mapContact(await backendMutation<BackendContact>(`/api/v1/contact-methods/${contactId}`, {}, {
    method: 'DELETE',
    idempotencyPrefix: 'profile-contact-delete',
  }))
}

export async function backendSetDefaultContact(contactId: string): Promise<UserContactMethod> {
  return mapContact(await backendMutation<BackendContact>(`/api/v1/contact-methods/${contactId}/set-default`, {}, {
    idempotencyPrefix: 'profile-contact-default',
  }))
}

export async function backendStartContactEmailVerification(contactId: string): Promise<BackendContactEmailVerificationChallenge> {
  return backendMutation<BackendContactEmailVerificationChallenge>(`/api/v1/contact-methods/${contactId}/email-verification/start`, {})
}

export async function backendConfirmContactEmailVerification(contactId: string, code: string): Promise<UserContactMethod> {
  return mapContact(await backendMutation<BackendContact>(`/api/v1/contact-methods/${contactId}/email-verification/confirm`, { code }, {
    idempotencyPrefix: 'profile-contact-email-confirm',
  }))
}

export async function backendMyMerchantProfile(): Promise<BackendMerchantProfile | null> {
  await ensureBackendSession('merchant', false)
  try {
    return await backendRequest<BackendMerchantProfile>('/api/v1/me/merchant-profile')
  } catch (error) {
    if (!(error instanceof BackendProblemError) || error.status !== 404) throw error
    return null
  }
}

export async function backendUpsertMerchantProfile(payload: { slug: string, displayName: string, avatarUrl?: string }): Promise<BackendMerchantProfile> {
  await ensureBackendSession('merchant', false)
  return backendMutation<BackendMerchantProfile>('/api/v1/me/merchant-profile', {
    slug: payload.slug,
    displayName: payload.displayName,
    avatarUrl: payload.avatarUrl ?? '',
  })
}

export async function backendPublicUserProfile(username: string) {
  const encodedUsername = encodeURIComponent(username)
  const response = await backendRequest<BackendPublicUserProfileBundle>(`/api/v1/users/${encodedUsername}/public-profile`)
  return {
    profile: mapPublicProfile(response.profile),
    reputations: (response.reputations ?? []).map(mapBackendReputationSnapshot),
    carpools: response.carpools,
    services: response.services,
    completions: response.completions,
    reviews: response.reviews,
    disputes: response.disputes,
  }
}

export async function backendPublicMerchantProfile(slug: string) {
  return backendRequest<PublicMerchantProfile>(`/api/v1/merchant-profiles/${encodeURIComponent(slug)}`)
}

function mapPublicProfile(value: BackendPublicUserProfile): PublicUserProfile {
  return {
    ...value,
    accountStatus: value.accountStatus as PublicUserProfile['accountStatus'],
    badges: value.badges.map(code => ({ id: `backend-${code}`, code, label: code, type: code === 'admin' ? 'system' : 'identity' })),
    privacy: {
      showCreatedAt: value.privacy.showCreatedAt,
      showLastActiveAt: value.privacy.showLastActiveAt,
      showCompletionStats: value.privacy.showCompletedCarpoolCount || value.privacy.showCompletedApiIntentCount,
      showResponseMedian: value.privacy.showResponseMedian,
      showResolvedDisputeSummary: value.privacy.showResolvedDisputeSummary,
      allowPublicProfileReport: value.privacy.allowPublicProfileReport,
    },
  }
}

function mapProfile(value: BackendProfile): UserProfile {
  return {
    id: value.id,
    username: value.username,
    displayName: value.displayName,
    bio: value.bio,
    avatarUrl: value.avatarUrl,
    customAvatarUrl: value.customAvatarUrl,
    email: value.email,
    emailVerified: value.emailVerified,
    emailVerifiedAt: value.emailVerifiedAt,
    passwordConfigured: value.passwordConfigured,
    avatarMode: value.avatarMode,
    regionCode: value.regionCode,
    timezone: value.timezone,
    linuxDoBinding: {
      bound: value.linuxDoBinding.bound,
      linuxDoUserId: value.linuxDoBinding.linuxDoUserId,
      linuxDoUsername: value.linuxDoBinding.linuxDoUsername,
      linuxDoAvatarUrl: value.linuxDoBinding.linuxDoAvatarUrl,
      trustLevel: value.linuxDoBinding.trustLevel,
      lastSyncedAt: value.linuxDoBinding.lastSyncedAt,
    },
    badges: (value.badges ?? []).map(code => ({ id: `backend-${code}`, code, label: code, type: code === 'admin' ? 'system' : 'identity' })),
    accountStatus: value.accountStatus as UserProfile['accountStatus'],
    permissions: value.permissions,
    capabilities: normalizeCapabilities(value.capabilities),
    restrictions: value.restrictions ?? [],
    usernameChangePolicy: value.usernameChangePolicy,
    privacy: {
      showCreatedAt: value.privacy.showCreatedAt,
      showLastActiveAt: value.privacy.showLastActiveAt,
      showCompletionStats: value.privacy.showCompletedCarpoolCount || value.privacy.showCompletedApiIntentCount,
      showResponseMedian: value.privacy.showResponseMedian,
      showResolvedDisputeSummary: value.privacy.showResolvedDisputeSummary,
      allowPublicProfileReport: value.privacy.allowPublicProfileReport,
    },
    createdAt: value.createdAt,
    lastActiveAt: value.lastActiveAt ?? '',
  }
}

function toBackendPrivacy(value: UserProfile['privacy']): BackendPrivacy {
  return {
    showCreatedAt: value.showCreatedAt,
    showLastActiveAt: value.showLastActiveAt,
    showCompletedCarpoolCount: value.showCompletionStats,
    showCompletedApiIntentCount: value.showCompletionStats,
    showResponseMedian: value.showResponseMedian,
    showResolvedDisputeSummary: value.showResolvedDisputeSummary,
    allowPublicProfileReport: value.allowPublicProfileReport,
  }
}

function mapContact(value: BackendContact, displayValue = ''): UserContactMethod {
  return {
    id: value.id,
    userId: value.userId,
    type: value.type,
    label: value.label,
    maskedValue: value.maskedValue,
    displayValue: value.displayValue ?? displayValue,
    usageScopes: value.usageScopes,
    isDefault: value.isDefault,
    enabled: value.enabled,
    verified: value.verified,
    createdAt: value.createdAt,
    updatedAt: value.updatedAt,
  }
}

function toContactPayload(payload: SaveContactMethodRequest) {
  return {
    type: payload.type,
    label: payload.label,
    displayValue: payload.displayValue,
    usageScopes: payload.usageScopes,
    isDefault: payload.isDefault,
    enabled: payload.enabled,
  }
}
