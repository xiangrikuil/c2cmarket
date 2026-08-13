import type {
  AdminStudentInstitutionDomain,
  AdminStudentInstitutionDomainCreateRequest,
  AdminStudentInstitutionDomainList,
  AdminStudentInstitutionDomainUpdateRequest,
  AdminStudentRegistrationSetting,
  AdminStudentRegistrationUpdateRequest,
} from '@/api/generated/openapi'
import { backendMutation, backendRequest, ensureBackendSession, shouldUseRealBackend } from '@/lib/backendClient'
import {
  createMockInstitutionDomain,
  getMockRegistrationAdminState,
  updateMockInstitutionDomain,
  updateMockRegistrationSetting,
} from '@/lib/mockAuth'

export type { AdminStudentInstitutionDomain, AdminStudentRegistrationSetting } from '@/api/generated/openapi'

async function requireAdmin() {
  await ensureBackendSession('admin', true)
}

export async function getAdminStudentRegistrationSetting(): Promise<AdminStudentRegistrationSetting> {
  await requireAdmin()
  if (!shouldUseRealBackend()) return getMockRegistrationAdminState().setting
  return backendRequest<AdminStudentRegistrationSetting>('/api/v1/admin/student-registration')
}

export async function updateAdminStudentRegistrationSetting(input: AdminStudentRegistrationUpdateRequest): Promise<AdminStudentRegistrationSetting> {
  await requireAdmin()
  if (!shouldUseRealBackend()) {
    return updateMockRegistrationSetting(input)
  }
  const updated = await backendMutation<AdminStudentRegistrationSetting | undefined>('/api/v1/admin/student-registration', input, {
    method: 'PATCH',
    idempotencyPrefix: 'admin-student-registration',
    ifMatch: input.expectedVersion,
  })
  return updated ?? getAdminStudentRegistrationSetting()
}

export async function getAdminStudentInstitutionDomains(): Promise<AdminStudentInstitutionDomain[]> {
  await requireAdmin()
  if (!shouldUseRealBackend()) return getMockRegistrationAdminState().domains
  const response = await backendRequest<AdminStudentInstitutionDomainList>('/api/v1/admin/student-institution-domains')
  return response.items
}

export async function createAdminStudentInstitutionDomain(input: AdminStudentInstitutionDomainCreateRequest): Promise<AdminStudentInstitutionDomain> {
  await requireAdmin()
  if (!shouldUseRealBackend()) {
    return createMockInstitutionDomain(input)
  }
  return backendMutation<AdminStudentInstitutionDomain>('/api/v1/admin/student-institution-domains', input, {
    idempotencyPrefix: 'admin-student-institution-domain',
    ifMatch: 0,
  })
}

export async function updateAdminStudentInstitutionDomain(
  input: AdminStudentInstitutionDomainUpdateRequest & { id: string },
): Promise<AdminStudentInstitutionDomain> {
  await requireAdmin()
  if (!shouldUseRealBackend()) {
    return updateMockInstitutionDomain(input)
  }
  return backendMutation<AdminStudentInstitutionDomain>(`/api/v1/admin/student-institution-domains/${encodeURIComponent(input.id)}`, {
    institutionName: input.institutionName,
    enabled: input.enabled,
    expectedVersion: input.expectedVersion,
    reason: input.reason,
  }, {
    method: 'PATCH',
    idempotencyPrefix: 'admin-student-institution-domain-update',
    ifMatch: input.expectedVersion,
  })
}
