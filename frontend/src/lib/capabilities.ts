import type { Capability as GeneratedCapability } from '@/api/generated/openapi'

export type Capability = GeneratedCapability

export const CAPABILITY = {
  adminAccess: 'admin.access',
  apiOrderCreate: 'api_order.create',
  apiProbeManage: 'api_probe.manage',
  apiQuotaPublish: 'api_quota.publish',
  apiServicePublish: 'api_service.publish',
  carpoolApply: 'carpool.apply',
  carpoolPublish: 'carpool.publish',
} as const satisfies Record<string, Capability>

export const capabilityValues = [
  CAPABILITY.adminAccess,
  CAPABILITY.apiOrderCreate,
  CAPABILITY.apiProbeManage,
  CAPABILITY.apiQuotaPublish,
  CAPABILITY.apiServicePublish,
  CAPABILITY.carpoolApply,
  CAPABILITY.carpoolPublish,
] as const satisfies readonly Capability[]

export type CapabilitySource = {
  capabilities?: readonly string[] | null
} | readonly string[] | null | undefined

export type CapabilityIdentityFacts = {
  linuxDoBound: boolean
  studentClaim: boolean
  admin: boolean
}

const capabilitySet = new Set<string>(capabilityValues)

export function isCapability(value: unknown): value is Capability {
  return typeof value === 'string' && capabilitySet.has(value)
}

export function normalizeCapabilities(value: unknown): Capability[] {
  if (!Array.isArray(value)) return []
  const selected = new Set(value.filter(isCapability))
  return capabilityValues.filter(capability => selected.has(capability))
}

export function projectCapabilities(facts: CapabilityIdentityFacts): Capability[] {
  const projected: Capability[] = []
  if (facts.admin) projected.push(CAPABILITY.adminAccess)
  if (facts.linuxDoBound || facts.studentClaim) projected.push(CAPABILITY.apiOrderCreate)
  if (facts.linuxDoBound) {
    projected.push(
      CAPABILITY.apiProbeManage,
      CAPABILITY.apiQuotaPublish,
      CAPABILITY.apiServicePublish,
      CAPABILITY.carpoolApply,
      CAPABILITY.carpoolPublish,
    )
  }
  return normalizeCapabilities(projected)
}

function capabilitiesFrom(source: CapabilitySource): readonly string[] {
  if (Array.isArray(source)) return source
  if (source && typeof source === 'object' && 'capabilities' in source && Array.isArray(source.capabilities)) {
    return source.capabilities
  }
  return []
}

export function hasCapability(source: CapabilitySource, capability: Capability): boolean {
  return capabilitiesFrom(source).includes(capability)
}

export function hasAnyCapability(source: CapabilitySource, capabilities: readonly Capability[]): boolean {
  return capabilities.some(capability => hasCapability(source, capability))
}

export function capabilityFromRouteMeta(meta: Record<string, unknown>): Capability | null {
  return isCapability(meta.capability) ? meta.capability : null
}
