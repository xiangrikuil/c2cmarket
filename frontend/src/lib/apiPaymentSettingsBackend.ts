import type {
  ApiAccountPaymentSettings as BackendApiAccountPaymentSettings,
  UpdateApiAccountPaymentSettingsRequest,
} from '@/api/generated/openapi'
import { backendMutation, backendRequest, requireBackendSession } from '@/lib/backendClient'
import {
  defaultApiPaymentWindowMinutes,
  normalizeApiPaymentAccountSettings,
  type ApiPaymentAccountSettings,
} from '@/lib/apiPaymentSettings'

function mapAccountPaymentSettings(settings: BackendApiAccountPaymentSettings): ApiPaymentAccountSettings {
  return normalizeApiPaymentAccountSettings(settings)
}

export async function backendGetApiPaymentAccountSettings() {
  await requireBackendSession()
  const settings = await backendRequest<BackendApiAccountPaymentSettings>('/api/v1/me/api-payment-settings')
  return mapAccountPaymentSettings(settings)
}

export async function backendUpdateApiPaymentAccountSettings(
  settings: Omit<ApiPaymentAccountSettings, 'updatedAt'>,
) {
  await requireBackendSession()
  const payload: UpdateApiAccountPaymentSettingsRequest = {
    paymentWindowMinutes: defaultApiPaymentWindowMinutes,
    paymentOptions: settings.paymentOptions.map(option => ({
      paymentMethod: option.paymentMethod,
      enabled: option.enabled,
      paymentInstructions: option.paymentInstructions,
      ...(option.paymentQrCodeDataUrl ? { paymentQrCodeDataUrl: option.paymentQrCodeDataUrl } : {}),
    })),
  }
  const updated = await backendMutation<BackendApiAccountPaymentSettings>(
    '/api/v1/me/api-payment-settings',
    payload,
    { method: 'PUT' },
  )
  return mapAccountPaymentSettings(updated)
}
