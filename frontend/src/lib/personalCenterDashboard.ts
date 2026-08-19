import {
  getApiOrderNextAction,
  getApiOrderStatusLabel,
  getCarpoolApplicationNextAction,
  getCarpoolApplicationStatusLabel,
  isApiOrderBuyerActionRequired,
  isApiOrderMerchantActionRequired,
  isCarpoolBuyerActionRequired,
  isCarpoolOwnerActionRequired,
  type ApiOrder,
  type ApiService,
  type Carpool,
  type CarpoolApplication,
  type UserProfile,
} from '@/lib/api'

export type PersonalCenterTask = {
  key: string
  id: string
  kind: 'carpool-application' | 'api-order'
  role: 'buyer' | 'owner' | 'merchant'
  typeLabel: string
  title: string
  status: string
  nextAction: string
  updatedAt: string
  to: string
  priority: number
}

export type PersonalCenterMetric = {
  id: 'pending' | 'published' | 'api-orders' | 'completeness'
  label: string
  value: number | string
  hint: string
  loading: boolean
  available: boolean
}

export type PublishedContentKind = 'all' | 'carpool' | 'api-service'

export type PublishedContentItem = {
  key: string
  id: string
  kind: Exclude<PublishedContentKind, 'all'>
  kindLabel: string
  title: string
  summary: string
  status: string
  updatedAt: string
  manageTo: string
  active: boolean
}

export type AccountCompletenessTask = {
  id: 'display-name' | 'bio' | 'linuxdo' | 'email' | 'password' | 'contact' | 'api-payment'
  label: string
  description: string
  completed: boolean
  to: string
}

export type AccountCompleteness = {
  percentage: number
  completedCount: number
  missingCount: number
  tasks: AccountCompletenessTask[]
  nextTo: string
}

export type AccountAlert = {
  id: AccountCompletenessTask['id']
  title: string
  description: string
  actionLabel: string
  to: string
}

export type BuildPendingTasksInput = {
  buyerCarpoolApplications: CarpoolApplication[]
  ownerCarpoolApplications: CarpoolApplication[]
  buyerApiOrders: ApiOrder[]
  merchantApiOrders: ApiOrder[]
}

export type BuildPublishedContentInput = {
  carpools: Carpool[]
  apiServices: ApiService[]
}

export type BuildAccountCompletenessInput = {
  profile: UserProfile
  wechatBound: boolean
  hasApiServices: boolean
  apiPaymentComplete: boolean
}

export type FirstTransactionQuerySnapshot = {
  data: readonly unknown[] | undefined
  isSuccess: boolean
  isFetchedAfterMount: boolean
  isFetching: boolean
}

export type FirstTransactionQueries = {
  ownedCarpools: FirstTransactionQuerySnapshot
  ownedApiServices: FirstTransactionQuerySnapshot
  buyerCarpoolApplications: FirstTransactionQuerySnapshot
  ownerCarpoolApplications: FirstTransactionQuerySnapshot
  buyerApiOrders: FirstTransactionQuerySnapshot
  merchantApiOrders: FirstTransactionQuerySnapshot
}

const firstTransactionQueryNames: Array<keyof FirstTransactionQueries> = [
  'ownedCarpools',
  'ownedApiServices',
  'buyerCarpoolApplications',
  'ownerCarpoolApplications',
  'buyerApiOrders',
  'merchantApiOrders',
]

export function shouldShowFirstTransactionGuide(queries: FirstTransactionQueries) {
  return firstTransactionQueryNames.every((name) => {
    const query = queries[name]
    return query.isSuccess
      && query.isFetchedAfterMount
      && !query.isFetching
      && query.data?.length === 0
  })
}

export function dashboardTimestamp(value: string) {
  const normalized = value.includes('T') ? value : value.replace(' ', 'T')
  const parsed = new Date(normalized).getTime()
  return Number.isFinite(parsed) ? parsed : 0
}

function taskPriority(status: CarpoolApplication['status'] | ApiOrder['status']) {
  if (status === 'disputed') return 0
  if (status === 'payment_issue') return 1
  if (status === 'payment_submitted' || status === 'paid_confirmed') return 2
  if (status === 'pending_owner') return 3
  if (status === 'delivery_submitted') return 5
  if (status === 'pending_payment') return 6
  return 7
}

function carpoolTask(item: CarpoolApplication, role: 'buyer' | 'owner'): PersonalCenterTask {
  return {
    key: `carpool-application:${role}:${item.id}`,
    id: item.id,
    kind: 'carpool-application',
    role,
    typeLabel: role === 'buyer' ? '我的上车' : '上车申请',
    title: item.snapshot.productName,
    status: getCarpoolApplicationStatusLabel(item.status),
    nextAction: getCarpoolApplicationNextAction(item, role),
    updatedAt: item.updatedAt,
    to: role === 'buyer' ? `/my/rides/${item.id}` : `/merchant/carpool-applications/${item.id}`,
    priority: taskPriority(item.status),
  }
}

function apiOrderTask(item: ApiOrder, role: 'buyer' | 'merchant'): PersonalCenterTask {
  return {
    key: `api-order:${role}:${item.id}`,
    id: item.id,
    kind: 'api-order',
    role,
    typeLabel: role === 'buyer' ? 'API 购买订单' : 'API 销售订单',
    title: item.serviceTitle,
    status: getApiOrderStatusLabel(item.status),
    nextAction: getApiOrderNextAction(item, role),
    updatedAt: item.updatedAt,
    to: role === 'buyer' ? `/my/api-orders/${item.id}` : `/merchant/api-orders/${item.id}`,
    priority: taskPriority(item.status),
  }
}

export function buildPendingTasks(input: BuildPendingTasksInput) {
  const tasks = [
    ...input.buyerCarpoolApplications.filter(isCarpoolBuyerActionRequired).map(item => carpoolTask(item, 'buyer')),
    ...input.ownerCarpoolApplications.filter(isCarpoolOwnerActionRequired).map(item => carpoolTask(item, 'owner')),
    ...input.buyerApiOrders.filter(isApiOrderBuyerActionRequired).map(item => apiOrderTask(item, 'buyer')),
    ...input.merchantApiOrders.filter(isApiOrderMerchantActionRequired).map(item => apiOrderTask(item, 'merchant')),
  ]

  return tasks.sort((left, right) => left.priority - right.priority
    || dashboardTimestamp(right.updatedAt) - dashboardTimestamp(left.updatedAt)
    || left.key.localeCompare(right.key))
}

function apiServiceStatus(item: ApiService) {
  if (item.online) return '发布中'
  if (item.state === 'reviewing') return '审核中'
  if (item.state === 'paused') return '已暂停'
  return '已下线'
}

export function buildPublishedContent(input: BuildPublishedContentInput) {
  const items: PublishedContentItem[] = [
    ...input.carpools.map(item => ({
      key: `carpool:${item.id}`,
      id: item.id,
      kind: 'carpool' as const,
      kindLabel: '拼车车源',
      title: item.product,
      summary: `${item.region} · ${item.seats} · ¥${item.monthly}/月`,
      status: item.status,
      updatedAt: item.confirmedAt,
      manageTo: '/my/carpools',
      active: item.status === '可上车' || item.status === '候补',
    })),
    ...input.apiServices.map(item => ({
      key: `api-service:${item.id}`,
      id: item.id,
      kind: 'api-service' as const,
      kindLabel: 'API 服务',
      title: item.title,
      summary: `¥${item.cnyPerUsdAllowance ?? item.rate} / $1 · 可售 $${item.availableUsdAllowance ?? item.balance} · ${item.delivery}`,
      status: apiServiceStatus(item),
      updatedAt: item.lastOnlineConfirmedAt,
      manageTo: `/my/api-services/${item.id}`,
      active: item.online,
    })),
  ]

  return items.sort((left, right) => dashboardTimestamp(right.updatedAt) - dashboardTimestamp(left.updatedAt)
    || left.key.localeCompare(right.key))
}

export function countActivePublishedContent(items: PublishedContentItem[]) {
  return items.filter(item => item.active).length
}

export function uniqueRelatedApiOrderCount(buyerOrders: ApiOrder[], merchantOrders: ApiOrder[]) {
  return new Set([...buyerOrders, ...merchantOrders].map(item => item.id)).size
}

export function buildAccountCompleteness(input: BuildAccountCompletenessInput): AccountCompleteness {
  const tasks: AccountCompletenessTask[] = [
    {
      id: 'linuxdo',
      label: '绑定 Linux.do',
      description: '使用社区身份和信任等级。',
      completed: input.profile.linuxDoBinding.bound,
      to: '/my/account',
    },
    {
      id: 'email',
      label: '绑定验证邮箱',
      description: '用于站内通知和账号恢复。',
      completed: input.profile.emailVerified,
      to: '/my/account',
    },
  ]

  if (input.profile.linuxDoBinding.bound) {
    tasks.push({
      id: 'password',
      label: '设置备用密码',
      description: '认证入口不可用时仍可登录。',
      completed: input.profile.passwordConfigured,
      to: '/my/account',
    })
  }

  tasks.push(
    {
      id: 'contact',
      label: '绑定微信',
      description: '微信是平台必填联系方式，用于有效交易联系。',
      completed: input.wechatBound,
      to: '/my/contacts',
    },
    {
      id: 'display-name',
      label: '设置展示名',
      description: '让交易参与方清楚识别你。',
      completed: Boolean(input.profile.displayName.trim()),
      to: '/my/profile',
    },
    {
      id: 'bio',
      label: '填写个人简介',
      description: '说明你的交易或经营方向。',
      completed: Boolean(input.profile.bio?.trim()),
      to: '/my/profile',
    },
  )

  if (input.hasApiServices) {
    tasks.push({
      id: 'api-payment',
      label: '配置 API 收款方式',
      description: '已发布 API 服务需要可用的站外收款说明。',
      completed: input.apiPaymentComplete,
      to: '/my/contacts',
    })
  }

  const completedCount = tasks.filter(item => item.completed).length
  const missingCount = tasks.length - completedCount

  return {
    percentage: Math.round((completedCount / tasks.length) * 100),
    completedCount,
    missingCount,
    tasks,
    nextTo: tasks.find(item => !item.completed)?.to ?? '/my/profile',
  }
}

export function getPrimaryAccountAlert(completeness: AccountCompleteness): AccountAlert | null {
  const missingTasks = new Map(completeness.tasks.filter(item => !item.completed).map(item => [item.id, item]))
  const alertDefinitions: Array<Omit<AccountAlert, 'to'> & { id: AccountCompletenessTask['id'] }> = [
    {
      id: 'contact',
      title: '请先绑定微信',
      description: '微信是平台必填联系方式，绑定后会自动用于全部交易场景。',
      actionLabel: '绑定微信',
    },
    {
      id: 'linuxdo',
      title: 'Linux.do 绑定状态需要检查',
      description: '社区身份未绑定，公开身份和信任等级无法正常展示。',
      actionLabel: '检查账号认证',
    },
    {
      id: 'email',
      title: '尚未绑定验证邮箱',
      description: '认证入口不可用时，验证邮箱是重要的账号恢复方式。',
      actionLabel: '立即绑定',
    },
    {
      id: 'password',
      title: '尚未设置备用密码',
      description: '设置后可以在社区认证不可用时使用站内账号登录。',
      actionLabel: '设置密码',
    },
    {
      id: 'api-payment',
      title: 'API 收款方式尚未完成',
      description: '你已发布 API 服务，请补全站外收款说明后再接单。',
      actionLabel: '配置收款方式',
    },
  ]

  for (const definition of alertDefinitions) {
    const task = missingTasks.get(definition.id)
    if (task) return { ...definition, to: task.to }
  }
  return null
}
