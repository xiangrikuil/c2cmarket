export type ApiQuotaLimitMode = 'limited' | 'unlimited' | 'unspecified'
export type ApiWritableQuotaLimitMode = Exclude<ApiQuotaLimitMode, 'unspecified'>

export type ApiQuotaUsageLimit = {
  mode: ApiQuotaLimitMode
  amountUsd: string | null
}

export type ApiQuotaUsagePolicy = {
  fiveHour: ApiQuotaUsageLimit
  daily: ApiQuotaUsageLimit
  scope: 'per_buyer_credential'
  dailyReset: 'utc_plus_8_calendar_day'
}

export type ApiQuotaUsageLimitInput = {
  mode: ApiWritableQuotaLimitMode
  amountUsd?: string
}

export type ApiQuotaUsagePolicyInput = {
  fiveHour: ApiQuotaUsageLimitInput
  daily: ApiQuotaUsageLimitInput
}
