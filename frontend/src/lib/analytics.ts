export const ANALYTICS_EVENTS = [
  'search_submit',
  'carpool_detail_view',
  'carpool_publish_success',
  'carpool_application_submit_success',
  'contact_window_reveal',
  'api_service_detail_view',
  'api_service_publish_success',
  'api_purchase_intent_create_success',
  'favorite_toggle',
  'report_submit',
  'detail_visible_time',
] as const

export type AnalyticsEventName = typeof ANALYTICS_EVENTS[number]
export type AnalyticsValue = string | number | boolean
export type AnalyticsProperties = Record<string, AnalyticsValue>

type RawProperties = Record<string, unknown>
type UmamiWindow = Window & {
  umami?: {
    track?: (eventName: string, data?: AnalyticsProperties) => unknown
  }
}

const accessModes = [
  'personal_account_cost_share',
  'provider_member_invitation',
  'owner_managed_access',
  'other_off_platform',
  'not_allowed',
] as const

const providerCategories = ['gpt', 'claude', 'cursor', 'gemini', 'perplexity', 'other', 'unknown'] as const
const entityTypes = ['carpool', 'api_service', 'contact_snapshot', 'public_user', 'carpool_application', 'carpool_membership', 'api_purchase_intent', 'api_order', 'unknown'] as const
const favoriteActions = ['add', 'remove'] as const
const reportReasonCodes = ['unreachable', 'contact_invalid', 'impersonation', 'description_mismatch', 'seat_rule_dispute', 'api_quota_dispute', 'order_delivery_dispute', 'other'] as const
const deliveryModes = ['api_key_endpoint', 'sub2api_panel_account', 'unknown'] as const
const billingModes = ['metered_credit', 'manual_credit', 'fixed_package', 'unknown'] as const
const riskNotices = ['openai_subscription_carpool', 'none', 'unknown'] as const

const hasOwn = (value: RawProperties, key: string) => Object.prototype.hasOwnProperty.call(value, key)

const asNumber = (value: unknown) => {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim()) {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return null
}

const asBoolean = (value: unknown) => typeof value === 'boolean' ? value : null

const asString = (value: unknown) => typeof value === 'string' ? value.trim() : ''

const normalizeEnum = <T extends readonly string[]>(value: unknown, allowed: T, fallback: T[number] | 'unknown' = 'unknown') => {
  const normalized = asString(value).replaceAll('-', '_')
  return allowed.includes(normalized) ? normalized as T[number] : fallback
}

const normalizeSourceRoute = (value: unknown) => {
  const route = asString(value)
  if (!route) return null
  if (route.startsWith('/')) return normalizeKnownSourcePath(route.split(/[?#]/, 1)[0] || '/')
  if (/^[a-z0-9_-]+$/i.test(route)) return route
  return 'unknown'
}

const normalizeKnownSourcePath = (path: string) => {
  const segments = path.split('/').filter(Boolean)
  const first = segments[0]
  const second = segments[1]
  const fourth = segments[3]

  if (segments.length === 0) return '/'
  if (first === 'u' && segments.length === 2) return '/u/:username'
  if (first === 'announcements' && segments.length === 2) return '/announcements/:slug'
  if (first === 'official-prices' && segments.length === 2 && !['submit', 'manage', 'detail'].includes(second)) return '/official-prices/:id'
  if (first === 'carpools' && segments.length === 2 && !['new', 'detail'].includes(second)) return '/carpools/:id'
  if (first === 'demands' && segments.length === 2) return '/demands/:id'
  if (first === 'api-market' && segments.length === 2 && !['new', 'detail'].includes(second)) return '/api-market/:id'
  if (first === 'api-intents' && segments.length === 2) return '/api-intents/:id'
  if (first === 'my' && second === 'rides' && segments.length === 3) return '/my/rides/:id'
  if (first === 'my' && second === 'api-orders' && segments.length === 3) return '/my/api-orders/:id'
  if (first === 'my' && second === 'feedback' && segments.length === 3) return '/my/feedback/:id'
  if (first === 'merchant' && second === 'carpool-applications' && segments.length === 3) return '/merchant/carpool-applications/:id'
  if (first === 'merchant' && second === 'api-orders' && segments.length === 3) return '/merchant/api-orders/:id'
  if (first === 'admin' && second === 'feedback' && segments.length === 3) return '/admin/feedback/:id'
  if (first === 'admin' && second === 'announcements' && fourth === 'edit') return '/admin/announcements/:id/edit'
  return path
}

const normalizeProductCategory = (value: unknown) => {
  const text = asString(value).toLowerCase()
  if (!text) return 'unknown'
  if (text.includes('gpt') || text.includes('openai') || text.includes('chatgpt')) return 'gpt'
  if (text.includes('claude') || text.includes('anthropic')) return 'claude'
  if (text.includes('cursor')) return 'cursor'
  if (text.includes('gemini')) return 'gemini'
  if (text.includes('perplexity')) return 'perplexity'
  return 'other'
}

const normalizeProviderCategory = (value: unknown) => {
  const direct = normalizeEnum(value, providerCategories)
  return direct === 'unknown' ? normalizeProductCategory(value) : direct
}

const normalizeEntityType = (value: unknown) => {
  const normalized = asString(value).replaceAll('-', '_')
  return entityTypes.includes(normalized as typeof entityTypes[number])
    ? normalized as typeof entityTypes[number]
    : 'unknown'
}

const pickFirst = (props: RawProperties, keys: string[]) => {
  for (const key of keys) {
    if (hasOwn(props, key)) return props[key]
  }
  return undefined
}

export const bucketPriceCny = (value: unknown) => {
  const amount = asNumber(value)
  if (amount === null) return 'unknown'
  if (amount <= 0) return 'free_or_zero'
  if (amount < 20) return 'lt_20'
  if (amount < 50) return '20_49'
  if (amount < 100) return '50_99'
  if (amount < 200) return '100_199'
  return '200_plus'
}

export const bucketSeats = (value: unknown) => {
  const seats = asNumber(value)
  if (seats === null || seats <= 0) return 'unknown'
  if (seats <= 1) return '1'
  if (seats <= 5) return '2_5'
  if (seats <= 10) return '6_10'
  if (seats <= 20) return '11_20'
  return '20_plus'
}

export const bucketVisibleSeconds = (value: unknown) => {
  const seconds = asNumber(value)
  if (seconds === null || seconds < 0) return 'unknown'
  if (seconds < 3) return 'lt_3'
  if (seconds < 10) return '3_9'
  if (seconds < 30) return '10_29'
  if (seconds < 60) return '30_59'
  if (seconds < 180) return '60_179'
  if (seconds < 600) return '180_599'
  return '600_plus'
}

const bucketResultCount = (value: unknown) => {
  const count = asNumber(value)
  if (count === null || count <= 0) return '0'
  if (count <= 5) return '1_5'
  if (count <= 20) return '6_20'
  if (count <= 50) return '21_50'
  return '51_plus'
}

const addSourceRoute = (target: AnalyticsProperties, props: RawProperties) => {
  const sourceRoute = normalizeSourceRoute(props.source_route)
  if (sourceRoute) target.source_route = sourceRoute
}

const sanitizeSearchSubmit = (props: RawProperties) => {
  const target: AnalyticsProperties = {}
  addSourceRoute(target, props)
  const hasQuery = asBoolean(props.has_query)
  if (hasQuery !== null) target.has_query = hasQuery
  if (hasOwn(props, 'result_count')) target.result_count_bucket = bucketResultCount(props.result_count)
  const filtersCount = asNumber(props.filters_count)
  if (filtersCount !== null) target.filters_count = Math.max(0, Math.min(10, Math.round(filtersCount)))
  return target
}

const addCarpoolFields = (target: AnalyticsProperties, props: RawProperties) => {
  const product = pickFirst(props, ['product_category', 'product'])
  target.product_category = normalizeProductCategory(product)
  const accessMode = normalizeEnum(pickFirst(props, ['access_mode', 'accessArrangementMode']), accessModes)
  target.access_mode = accessMode
  target.price_bucket = bucketPriceCny(pickFirst(props, ['price_cny', 'monthly_price_cny', 'monthly']))
  target.seats_bucket = bucketSeats(pickFirst(props, ['seats', 'maxMembers', 'totalSeats']))
  const riskAckRequired = asBoolean(pickFirst(props, ['risk_ack_required', 'riskAcknowledged']))
  if (riskAckRequired !== null) target.risk_ack_required = riskAckRequired
  const riskNotice = normalizeEnum(pickFirst(props, ['risk_notice', 'riskNoticeCode']), riskNotices)
  if (riskNotice !== 'unknown') target.risk_notice = riskNotice
}

const sanitizeCarpoolEvent = (props: RawProperties) => {
  const target: AnalyticsProperties = {}
  addSourceRoute(target, props)
  addCarpoolFields(target, props)
  return target
}

const addApiServiceFields = (target: AnalyticsProperties, props: RawProperties) => {
  target.provider_category = normalizeProviderCategory(pickFirst(props, ['provider_category', 'category', 'title', 'models_text']))
  target.billing_mode = normalizeEnum(pickFirst(props, ['billing_mode', 'billingMode']), billingModes)
  target.delivery_mode = normalizeEnum(pickFirst(props, ['delivery_mode', 'selectedDeliveryMode']), deliveryModes)
  target.price_bucket = bucketPriceCny(pickFirst(props, ['price_cny', 'minimum_purchase_cny', 'minimumPurchaseCny']))
}

const sanitizeApiServiceEvent = (props: RawProperties) => {
  const target: AnalyticsProperties = {}
  addSourceRoute(target, props)
  addApiServiceFields(target, props)
  return target
}

const sanitizeApiPurchaseIntent = (props: RawProperties) => {
  const target: AnalyticsProperties = {}
  addSourceRoute(target, props)
  target.provider_category = normalizeProviderCategory(pickFirst(props, ['provider_category', 'category', 'title', 'models_text']))
  target.delivery_mode = normalizeEnum(pickFirst(props, ['delivery_mode', 'selectedDeliveryMode']), deliveryModes)
  target.budget_bucket = bucketPriceCny(pickFirst(props, ['budget_cny', 'purchase_amount_cny', 'purchaseAmountCny', 'amount']))
  return target
}

const sanitizeFavoriteToggle = (props: RawProperties) => {
  const target: AnalyticsProperties = {}
  addSourceRoute(target, props)
  target.entity_type = normalizeEntityType(props.entity_type)
  target.action = normalizeEnum(props.action, favoriteActions)
  return target
}

const sanitizeReportSubmit = (props: RawProperties) => {
  const target: AnalyticsProperties = {}
  addSourceRoute(target, props)
  target.entity_type = normalizeEntityType(pickFirst(props, ['entity_type', 'target_type', 'targetType']))
  target.reason_code = normalizeEnum(pickFirst(props, ['reason_code', 'reasonCode']), reportReasonCodes)
  return target
}

const sanitizeContactWindowReveal = (props: RawProperties) => {
  const target: AnalyticsProperties = {}
  addSourceRoute(target, props)
  target.entity_type = normalizeEntityType(props.entity_type)
  return target
}

const sanitizeDetailVisibleTime = (props: RawProperties) => {
  const target: AnalyticsProperties = {}
  addSourceRoute(target, props)
  target.entity_type = normalizeEntityType(props.entity_type)
  target.visible_seconds_bucket = bucketVisibleSeconds(pickFirst(props, ['visible_seconds', 'seconds']))
  return target
}

export const sanitizeAnalyticsEvent = (eventName: AnalyticsEventName, props: RawProperties = {}) => {
  switch (eventName) {
    case 'search_submit':
      return sanitizeSearchSubmit(props)
    case 'carpool_detail_view':
    case 'carpool_publish_success':
    case 'carpool_application_submit_success':
      return sanitizeCarpoolEvent(props)
    case 'api_service_detail_view':
    case 'api_service_publish_success':
      return sanitizeApiServiceEvent(props)
    case 'api_purchase_intent_create_success':
      return sanitizeApiPurchaseIntent(props)
    case 'favorite_toggle':
      return sanitizeFavoriteToggle(props)
    case 'report_submit':
      return sanitizeReportSubmit(props)
    case 'contact_window_reveal':
      return sanitizeContactWindowReveal(props)
    case 'detail_visible_time':
      return sanitizeDetailVisibleTime(props)
  }
}

const analyticsEnabled = () => import.meta.env.VITE_UMAMI_ENABLED === 'true'

const debugEnabled = () => import.meta.env.VITE_UMAMI_DEBUG === 'true'

const getUmamiTracker = () => {
  if (typeof window === 'undefined') return null
  const tracker = (window as UmamiWindow).umami?.track
  return typeof tracker === 'function' ? tracker : null
}

export const trackAnalytics = (eventName: AnalyticsEventName, props: RawProperties = {}) => {
  if (!analyticsEnabled()) return
  const tracker = getUmamiTracker()
  if (!tracker) return
  const sanitized = sanitizeAnalyticsEvent(eventName, props)
  try {
    tracker(eventName, sanitized)
  } catch (error) {
    if (debugEnabled()) console.debug('[analytics] track failed', error)
  }
}
