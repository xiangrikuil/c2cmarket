import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import {
  createAdminStudentInstitutionDomain,
  getAdminStudentInstitutionDomains,
  getAdminStudentRegistrationSetting,
  updateAdminStudentInstitutionDomain,
  updateAdminStudentRegistrationSetting,
} from '@/lib/studentRegistrationAdminBackend'

const settingKey = ['admin', 'student-registration'] as const
const domainsKey = ['admin', 'student-institution-domains'] as const

export function useAdminStudentRegistrationSetting() {
  return useQuery({ queryKey: settingKey, queryFn: getAdminStudentRegistrationSetting, retry: false })
}

export function useAdminStudentInstitutionDomains() {
  return useQuery({ queryKey: domainsKey, queryFn: getAdminStudentInstitutionDomains, retry: false })
}

export function useUpdateAdminStudentRegistrationSetting() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: updateAdminStudentRegistrationSetting,
    onSuccess(data) {
      queryClient.setQueryData(settingKey, data)
      queryClient.invalidateQueries({ queryKey: ['email-registration-config'] })
    },
  })
}

export function useCreateAdminStudentInstitutionDomain() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: createAdminStudentInstitutionDomain,
    onSuccess() {
      queryClient.invalidateQueries({ queryKey: domainsKey })
      queryClient.invalidateQueries({ queryKey: ['email-registration-config'] })
    },
  })
}

export function useUpdateAdminStudentInstitutionDomain() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: updateAdminStudentInstitutionDomain,
    onSuccess() {
      queryClient.invalidateQueries({ queryKey: domainsKey })
      queryClient.invalidateQueries({ queryKey: ['email-registration-config'] })
    },
  })
}
