import {
  BackendProblemError,
  backendMutation,
  backendRequest,
  ensureBackendSession,
  requireBackendSession,
} from '@/lib/backendClient'
import type {
  PublicCompletionRecord,
  PublicDisputeRecord,
  PublicReviewRecord,
} from '@/data/mock'
import type {
  ContactMethodType,
  ApiService,
  Carpool,
  PublicMerchantProfile,
  PublicUserProfile,
  SaveContactMethodRequest,
  UpdateMyProfileRequest,
  UserContactMethod,
  UserProfile,
} from '@/lib/api'
import { backendPublicUserReviews } from '@/lib/reviewBackend'

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
  linuxDoBinding: {
    bound: boolean
    linuxDoUserId: string | null
    linuxDoUsername: string | null
    linuxDoAvatarUrl: string | null
    trustLevel: number | null
    lastSyncedAt: string | null
  }
  badges: string[]
  restrictions: string[]
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

type BackendPublicUserProfileBundle = {
  profile: PublicUserProfile
  carpools: Carpool[]
  services: ApiService[]
  completions: PublicCompletionRecord[]
  reviews: PublicReviewRecord[]
  disputes: PublicDisputeRecord[]
}

type BackendPublicMerchantProfileBundle = {
  profile: PublicMerchantProfile
  services: ApiService[]
  completions: PublicCompletionRecord[]
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
    privacy: payload.privacy,
  }, { method: 'PATCH' })
  return mapProfile(response)
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
  })
  return mapContact(response, payload.displayValue)
}

export async function backendDeleteContact(contactId: string): Promise<UserContactMethod> {
  return mapContact(await backendMutation<BackendContact>(`/api/v1/contact-methods/${contactId}`, {}, { method: 'DELETE' }))
}

export async function backendSetDefaultContact(contactId: string): Promise<UserContactMethod> {
  return mapContact(await backendMutation<BackendContact>(`/api/v1/contact-methods/${contactId}/set-default`, {}))
}

export async function backendVerifyContact(contactId: string): Promise<UserContactMethod> {
  return mapContact(await backendMutation<BackendContact>(`/api/v1/contact-methods/${contactId}/verify`, {}))
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
  const [response, reviews] = await Promise.all([
    backendRequest<BackendPublicUserProfileBundle>(`/api/v1/users/${encodedUsername}/public-profile`),
    backendPublicUserReviews(username),
  ])
  return {
    profile: response.profile,
    carpools: response.carpools,
    services: response.services,
    completions: response.completions,
    reviews,
    disputes: response.disputes,
  }
}

export async function backendPublicMerchantProfile(slug: string) {
  const response = await backendRequest<BackendPublicMerchantProfileBundle>(`/api/v1/merchant-profiles/${encodeURIComponent(slug)}`)
  return {
    profile: response.profile,
    services: response.services,
    completions: response.completions,
    reviews: response.reviews,
    disputes: response.disputes,
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
    badges: value.badges.map(code => ({ id: `backend-${code}`, code, label: code, type: code === 'admin' ? 'system' : 'identity' })),
    accountStatus: value.accountStatus as UserProfile['accountStatus'],
    permissions: value.permissions,
    restrictions: value.restrictions,
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
