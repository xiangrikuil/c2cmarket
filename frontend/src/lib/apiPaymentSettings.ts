export type ApiPaymentMethod = 'wechat' | 'alipay'

export type ApiPaymentOption = {
  paymentMethod: ApiPaymentMethod
  enabled: boolean
  paymentInstructions: string
  paymentQrCodeDataUrl: string | null
}

export type ApiPaymentAccountSettings = {
  paymentWindowMinutes: number
  paymentOptions: ApiPaymentOption[]
  updatedAt: string
}

type RawApiPaymentOption = Partial<Omit<ApiPaymentOption, 'paymentMethod'>> & {
  paymentMethod?: string
}

type RawApiPaymentAccountSettings = Partial<Omit<ApiPaymentAccountSettings, 'paymentOptions'>> & {
  paymentOptions?: RawApiPaymentOption[]
}

export const defaultApiPaymentWindowMinutes = 10

export const apiPaymentMethodLabels: Record<ApiPaymentMethod, string> = {
  wechat: '微信',
  alipay: '支付宝',
}

export const apiPaymentMethods = [
  { value: 'wechat', label: apiPaymentMethodLabels.wechat, hint: '创建订单后买家可查看微信收款码并站外确认。' },
  { value: 'alipay', label: apiPaymentMethodLabels.alipay, hint: '创建订单后买家可查看支付宝收款码并站外确认。' },
] satisfies Array<{ value: ApiPaymentMethod, label: string, hint: string }>

export const apiPaymentQrCodeMethods: ApiPaymentMethod[] = ['wechat', 'alipay']

export function createDefaultApiPaymentOptions(): ApiPaymentOption[] {
  return apiPaymentMethods.map(item => ({
    paymentMethod: item.value,
    enabled: false,
    paymentInstructions: '',
    paymentQrCodeDataUrl: null,
  }))
}

export function createEmptyApiPaymentAccountSettings(updatedAt = ''): ApiPaymentAccountSettings {
  return {
    paymentWindowMinutes: defaultApiPaymentWindowMinutes,
    paymentOptions: createDefaultApiPaymentOptions(),
    updatedAt,
  }
}

export function normalizeApiPaymentAccountSettings(value: RawApiPaymentAccountSettings | null | undefined, updatedAt = ''): ApiPaymentAccountSettings {
  const sourceOptions = Array.isArray(value?.paymentOptions) ? value.paymentOptions : []
  const byMethod = new Map(sourceOptions.map(option => [option.paymentMethod, option]))
  return {
    paymentWindowMinutes: defaultApiPaymentWindowMinutes,
    paymentOptions: apiPaymentMethods.map(method => {
      const option = byMethod.get(method.value)
      return {
        paymentMethod: method.value,
        enabled: Boolean(option?.enabled),
        paymentInstructions: String(option?.paymentInstructions ?? ''),
        paymentQrCodeDataUrl: normalizeQrCodeDataUrl(option?.paymentQrCodeDataUrl),
      }
    }),
    updatedAt: value?.updatedAt || updatedAt,
  }
}

export function cloneApiPaymentAccountSettings(settings: ApiPaymentAccountSettings): ApiPaymentAccountSettings {
  return {
    paymentWindowMinutes: settings.paymentWindowMinutes,
    paymentOptions: settings.paymentOptions.map(option => ({ ...option })),
    updatedAt: settings.updatedAt,
  }
}

export function apiPaymentMethodRequiresQrCode(method: ApiPaymentMethod) {
  return apiPaymentQrCodeMethods.includes(method)
}

export function isApiPaymentMethod(value: string): value is ApiPaymentMethod {
  return value in apiPaymentMethodLabels
}

export function normalizeQrCodeDataUrl(value: unknown) {
  if (typeof value !== 'string') return null
  const trimmed = value.trim()
  return /^data:image\/(?:png|jpe?g|webp);base64,/i.test(trimmed) ? trimmed : null
}

export function isApiPaymentOptionComplete(option: Pick<ApiPaymentOption, 'paymentMethod' | 'paymentInstructions' | 'paymentQrCodeDataUrl'>) {
  if (apiPaymentMethodRequiresQrCode(option.paymentMethod)) return Boolean(option.paymentQrCodeDataUrl)
  return Boolean(option.paymentInstructions.trim())
}

export function enabledApiPaymentOptions(source: Pick<ApiPaymentAccountSettings, 'paymentOptions'>) {
  return source.paymentOptions.filter(option => option.enabled)
}

export function isApiPaymentWindowValid(value: number) {
  return value === defaultApiPaymentWindowMinutes
}

export function isApiPaymentAccountSettingsComplete(settings: Pick<ApiPaymentAccountSettings, 'paymentWindowMinutes' | 'paymentOptions'>) {
  const enabled = enabledApiPaymentOptions(settings)
  return isApiPaymentWindowValid(settings.paymentWindowMinutes)
    && enabled.length > 0
    && enabled.every(isApiPaymentOptionComplete)
}

export function apiPaymentSettingsSummary(settings: Pick<ApiPaymentAccountSettings, 'paymentWindowMinutes' | 'paymentOptions'>) {
  const labels = enabledApiPaymentOptions(settings).map(option => apiPaymentMethodLabels[option.paymentMethod])
  return labels.length ? `${labels.join(' / ')} · 固定 ${defaultApiPaymentWindowMinutes} 分钟确认` : '未配置 API 收款设置'
}

export function apiPaymentSettingsMissingReason(settings: Pick<ApiPaymentAccountSettings, 'paymentWindowMinutes' | 'paymentOptions'>) {
  if (!isApiPaymentWindowValid(settings.paymentWindowMinutes)) return `买家确认付款窗口固定为 ${defaultApiPaymentWindowMinutes} 分钟。`
  const enabled = enabledApiPaymentOptions(settings)
  if (!enabled.length) return '请先在个人中心启用至少一种 API 收款方式。'
  const missing = enabled.find(option => !isApiPaymentOptionComplete(option))
  if (missing) {
    return apiPaymentMethodRequiresQrCode(missing.paymentMethod)
      ? `请先在个人中心上传${apiPaymentMethodLabels[missing.paymentMethod]}收款码。`
      : `请先在个人中心填写${apiPaymentMethodLabels[missing.paymentMethod]}收款说明。`
  }
  return ''
}
