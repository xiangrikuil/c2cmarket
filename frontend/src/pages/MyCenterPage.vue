<script setup lang="ts">
import { computed, reactive, watchEffect } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { toast } from 'vue-sonner'
import { Car, ContactRound, CreditCard, Eye, ImageUp, Link2, LockKeyhole, LogIn, Mail, MailCheck, MessageCircle, RefreshCw, Save, ShoppingBag, Trash2, UserRound, UsersRound, X } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import StatCard from '@/components/market/StatCard.vue'
import SoftTable from '@/components/market/SoftTable.vue'
import TablePagination from '@/components/market/TablePagination.vue'
import { usePagination } from '@/composables/usePagination'
import { getPricingDisplay, getRemainingSeats } from '@/lib/pricing'
import { getApiMerchantDisplayName, getApiMerchantVisibilityLabel, getApiServicePublicDetailUrl, type ApiService, type AvatarMode, type ContactMethodType, type ContactUsageScope, type SaveContactMethodRequest, type UserContactMethod, type UserPrivacySettings } from '@/lib/api'
import {
  apiPaymentMethods,
  apiPaymentMethodRequiresQrCode,
  apiPaymentSettingsMissingReason,
  apiPaymentSettingsSummary,
  createDefaultApiPaymentOptions,
  defaultApiPaymentWindowMinutes,
  isApiPaymentAccountSettingsComplete,
  isApiPaymentOptionComplete,
  type ApiPaymentAccountSettings,
  type ApiPaymentMethod,
  type ApiPaymentOption,
} from '@/lib/apiPaymentSettings'
import { containsSensitiveContent } from '@/lib/formValidation'
import {
  useApiPaymentAccountSettingsQuery,
  useApiServices,
  useConfirmEmailVerificationMutation,
  useCreateContactMethodMutation,
  useDeleteContactMethodMutation,
  useDeleteCustomAvatarMutation,
  useMyContactMethodsQuery,
  useMyApiServices,
  useMyCarpools,
  usePauseApiServiceMutation,
  useMyProfileQuery,
  usePublishApiServiceMutation,
  useResumeApiServiceMutation,
  useSetBackupPasswordMutation,
  useSetDefaultContactMethodMutation,
  useStartEmailVerificationMutation,
  useUpdateApiPaymentAccountSettingsMutation,
  useUpdateContactMethodMutation,
  useUpdateMyProfileMutation,
  useUseLinuxDoAvatarMutation,
  useVerifyContactMethodMutation,
} from '@/queries/useMarketQueries'

const route = useRoute()
const router = useRouter()
const profileQuery = useMyProfileQuery()
const profile = profileQuery.data
const { data: contacts } = useMyContactMethodsQuery()
const { data: apiPaymentSettings } = useApiPaymentAccountSettingsQuery()
const { data: carpools } = useMyCarpools()
const { data: apiServices } = useMyApiServices()

const updateProfileMutation = useUpdateMyProfileMutation()
const deleteAvatarMutation = useDeleteCustomAvatarMutation()
const useLinuxDoAvatarMutation = useUseLinuxDoAvatarMutation()
const setPasswordMutation = useSetBackupPasswordMutation()
const startEmailVerificationMutation = useStartEmailVerificationMutation()
const confirmEmailVerificationMutation = useConfirmEmailVerificationMutation()
const createContactMutation = useCreateContactMethodMutation()
const updateContactMutation = useUpdateContactMethodMutation()
const deleteContactMutation = useDeleteContactMethodMutation()
const setDefaultContactMutation = useSetDefaultContactMethodMutation()
const verifyContactMutation = useVerifyContactMethodMutation()
const updateApiPaymentSettingsMutation = useUpdateApiPaymentAccountSettingsMutation()
const publishApiServiceMutation = usePublishApiServiceMutation()
const pauseApiServiceMutation = usePauseApiServiceMutation()
const resumeApiServiceMutation = useResumeApiServiceMutation()
const apiPaymentQrMaxBytes = 512 * 1024

const sectionLinks = [
  { label: '概览', to: '/my', key: 'overview' },
  { label: '个人资料', to: '/my/profile', key: 'profile' },
  { label: '联系方式', to: '/my/contacts', key: 'contacts' },
  { label: '账号与认证', to: '/my/account', key: 'account' },
  { label: '隐私设置', to: '/my/privacy', key: 'privacy' },
  { label: '我的收藏', to: '/my/favorites', key: 'favorites' },
  { label: '通知设置', to: '/my/notifications', key: 'notifications' },
] as const

const usageScopeOptions: { value: ContactUsageScope, label: string }[] = [
  { value: 'carpool_owner', label: '拼车车主' },
  { value: 'api_merchant', label: 'API 商户' },
  { value: 'buyer', label: '买家' },
  { value: 'dispute', label: '纠纷联系' },
]

const activeSection = computed(() => {
  if (route.path === '/my/profile') return 'profile'
  if (route.path === '/my/contacts') return 'contacts'
  if (route.path === '/my/account') return 'account'
  if (route.path === '/my/privacy') return 'privacy'
  if (route.path === '/my/favorites') return 'favorites'
  if (route.path === '/my/notifications') return 'notifications'
  return 'overview'
})

const profileForm = reactive({
  displayName: '',
  username: '',
  bio: '',
  regionCode: '',
  timezone: 'Asia/Shanghai',
  avatarMode: 'linuxdo' as AvatarMode,
  avatarUrl: '',
})

const passwordForm = reactive({
  currentPassword: '',
  newPassword: '',
  confirmPassword: '',
})

const emailForm = reactive({
  email: '',
  code: '',
})

const privacyForm = reactive<UserPrivacySettings>({
  showCreatedAt: true,
  showLastActiveAt: true,
  showCompletionStats: true,
  showResponseMedian: true,
  showResolvedDisputeSummary: true,
  allowPublicProfileReport: true,
})

const wechatForm = reactive({
  displayValue: '',
})

const loadedContactDraftKeys = reactive({
  wechat: '',
  email: '',
  apiPayment: '',
})

const apiPaymentForm = reactive<Omit<ApiPaymentAccountSettings, 'updatedAt'>>({
  paymentWindowMinutes: defaultApiPaymentWindowMinutes,
  paymentOptions: createDefaultApiPaymentOptions(),
})

const defaultContactUsageScopes: ContactUsageScope[] = ['carpool_owner', 'api_merchant', 'buyer', 'dispute']

const wechatContact = computed(() => (contacts.value ?? []).find(item => item.type === 'wechat') ?? null)
const emailContact = computed(() => (contacts.value ?? []).find(item => item.type === 'email') ?? null)
const enabledContactCount = computed(() => [wechatContact.value, emailContact.value].filter(item => item?.enabled && (item.type !== 'email' || item.verified)).length)
const wechatBound = computed(() => Boolean(wechatContact.value?.enabled && wechatContact.value.displayValue))
const emailBound = computed(() => Boolean(emailContact.value?.enabled && emailContact.value.verified))
const contactSaving = computed(() => createContactMutation.isPending.value || updateContactMutation.isPending.value)
const emailBindingPending = computed(() => contactSaving.value || startEmailVerificationMutation.isPending.value || confirmEmailVerificationMutation.isPending.value || verifyContactMutation.isPending.value)
const apiPaymentComplete = computed(() => isApiPaymentAccountSettingsComplete(apiPaymentForm))
const apiPaymentMissingReasonText = computed(() => apiPaymentSettingsMissingReason(apiPaymentForm))
const apiPaymentSummaryText = computed(() => apiPaymentSettingsSummary(apiPaymentForm))

watchEffect(() => {
  if (!profile.value) return
  profileForm.displayName = profile.value.displayName
  profileForm.username = profile.value.username
  profileForm.bio = profile.value.bio ?? ''
  profileForm.regionCode = profile.value.regionCode ?? ''
  profileForm.timezone = profile.value.timezone ?? 'Asia/Shanghai'
  profileForm.avatarMode = profile.value.avatarMode
  profileForm.avatarUrl = profile.value.customAvatarUrl ?? ''
  if (!emailForm.email) emailForm.email = emailContact.value?.displayValue || profile.value.email || ''
  Object.assign(privacyForm, profile.value.privacy)

  const wechat = wechatContact.value
  const wechatDraftKey = `${wechat?.id ?? 'empty'}:${wechat?.updatedAt ?? ''}`
  if (loadedContactDraftKeys.wechat !== wechatDraftKey) {
    wechatForm.displayValue = wechat?.displayValue ?? ''
    loadedContactDraftKeys.wechat = wechatDraftKey
  }

  const email = emailContact.value
  const emailDraftKey = `${email?.id ?? profile.value.email ?? 'empty'}:${email?.updatedAt ?? profile.value.emailVerifiedAt ?? ''}`
  if (loadedContactDraftKeys.email !== emailDraftKey) {
    emailForm.email = email?.displayValue || profile.value.email || ''
    loadedContactDraftKeys.email = emailDraftKey
  }

  const payment = apiPaymentSettings.value
  const paymentDraftKey = payment?.updatedAt || 'empty'
  if (payment && loadedContactDraftKeys.apiPayment !== paymentDraftKey) {
    apiPaymentForm.paymentWindowMinutes = payment.paymentWindowMinutes
    apiPaymentForm.paymentOptions = payment.paymentOptions.map(option => ({ ...option }))
    loadedContactDraftKeys.apiPayment = paymentDraftKey
  }
})

const carpoolRows = computed(() => carpools.value ?? [])
const apiServiceRows = computed(() => apiServices.value ?? [])
const carpoolPagination = usePagination(carpoolRows)
const apiServicePagination = usePagination(apiServiceRows)
const avatarText = computed(() => (profile.value?.displayName || profile.value?.username || '我').slice(0, 1).toUpperCase())
const profileErrorMessage = computed(() => {
  const error = profileQuery.error.value
  return error instanceof Error ? error.message : '无法读取个人资料，请登录后重试。'
})

function isSectionActive(to: string) {
  return route.path === to
}

function scopeLabels(scopes: ContactUsageScope[]) {
  return scopes.map(scope => usageScopeOptions.find(item => item.value === scope)?.label ?? scope).join('、')
}

function apiPaymentMethodLabel(method: ApiPaymentMethod) {
  return apiPaymentMethods.find(item => item.value === method)?.label ?? method
}

function apiPaymentInstructionsPlaceholder(method: ApiPaymentMethod) {
  if (apiPaymentMethodRequiresQrCode(method)) return '可选：填写收款码备注、核对口径或站外确认节奏。'
  return '填写 USDT 网络、地址确认方式和站外核对说明。'
}

function handleApiPaymentQrUpload(event: Event, option: ApiPaymentOption) {
  const input = event.target
  if (!(input instanceof HTMLInputElement)) return
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  if (!['image/png', 'image/jpeg', 'image/webp'].includes(file.type)) {
    toast.warning('请上传 PNG、JPG 或 WebP 格式的收款码图片。')
    return
  }
  if (file.size > apiPaymentQrMaxBytes) {
    toast.warning('收款码图片不能超过 512KB。')
    return
  }
  const reader = new FileReader()
  reader.onload = () => {
    if (typeof reader.result !== 'string') {
      toast.error('收款码读取失败，请重新选择图片。')
      return
    }
    option.paymentQrCodeDataUrl = reader.result
  }
  reader.onerror = () => toast.error('收款码读取失败，请重新选择图片。')
  reader.readAsDataURL(file)
}

function removeApiPaymentQr(option: ApiPaymentOption) {
  option.paymentQrCodeDataUrl = null
}

function apiPaymentOptionReady(option: ApiPaymentOption) {
  return isApiPaymentOptionComplete(option)
}

function saveProfile() {
  updateProfileMutation.mutate({
    displayName: profileForm.displayName,
    username: profileForm.username,
    bio: profileForm.bio || null,
    regionCode: profileForm.regionCode || null,
    timezone: profileForm.timezone || null,
    avatarMode: profileForm.avatarMode,
    avatarUrl: profileForm.avatarMode === 'custom_url' ? profileForm.avatarUrl.trim() : null,
    privacy: privacyForm,
  }, {
    onSuccess: () => toast.success('个人资料已保存。'),
    onError: error => toast.error(error instanceof Error ? error.message : '保存失败'),
  })
}

function savePassword() {
  if (passwordForm.newPassword !== passwordForm.confirmPassword) {
    toast.warning('两次输入的新密码不一致。')
    return
  }
  setPasswordMutation.mutate({
    currentPassword: passwordForm.currentPassword || undefined,
    newPassword: passwordForm.newPassword,
  }, {
    onSuccess: () => {
      passwordForm.currentPassword = ''
      passwordForm.newPassword = ''
      passwordForm.confirmPassword = ''
      toast.success('备用密码已更新。')
    },
    onError: error => toast.error(error instanceof Error ? error.message : '备用密码更新失败。'),
  })
}

function startEmailVerification() {
  startEmailVerificationMutation.mutate(emailForm.email, {
    onSuccess: challenge => {
      emailForm.email = challenge.email
      emailForm.code = ''
      toast.success('验证码已发送，请查看邮箱。')
    },
    onError: error => toast.error(error instanceof Error ? error.message : '验证码发送失败。'),
  })
}

function confirmEmailVerification() {
  confirmEmailVerificationMutation.mutate({
    email: emailForm.email,
    code: emailForm.code,
  }, {
    onSuccess: () => {
      emailForm.code = ''
      toast.success('邮箱已绑定。')
    },
    onError: error => toast.error(error instanceof Error ? error.message : '邮箱绑定失败。'),
  })
}

function savePrivacy() {
  if (!profile.value) return
  saveProfile()
}

function buildContactPayload(type: ContactMethodType, label: string, displayValue: string, current: UserContactMethod | null): SaveContactMethodRequest {
  return {
    type,
    label,
    displayValue: displayValue.trim(),
    usageScopes: current?.usageScopes.length ? [...current.usageScopes] : [...defaultContactUsageScopes],
    isDefault: current?.isDefault ?? false,
    enabled: true,
  }
}

function saveWechatContact() {
  const displayValue = wechatForm.displayValue.trim()
  if (!displayValue) {
    toast.warning('请先填写微信号。')
    return
  }
  const current = wechatContact.value
  const payload = buildContactPayload('wechat', '微信', displayValue, current)
  const mutationOptions = {
    onSuccess: () => {
      toast.success(current ? '微信联系方式已更新。' : '微信联系方式已绑定。')
    },
    onError: (error: Error) => toast.error(error.message),
  }
  if (current) {
    updateContactMutation.mutate({ contactId: current.id, payload }, mutationOptions)
    return
  }
  createContactMutation.mutate(payload, mutationOptions)
}

function markEmailContactVerified(contact: UserContactMethod) {
  if (contact.verified) {
    toast.success('邮箱联系方式已绑定。')
    return
  }
  verifyContactMutation.mutate(contact.id, {
    onSuccess: () => toast.success('邮箱联系方式已绑定。'),
    onError: error => toast.error(error instanceof Error ? error.message : '邮箱联系方式验证失败。'),
  })
}

function saveVerifiedEmailContact() {
  const displayValue = emailForm.email.trim().toLowerCase()
  if (!displayValue) {
    toast.warning('请先填写邮箱。')
    return
  }
  const current = emailContact.value
  const payload = buildContactPayload('email', '邮箱', displayValue, current)
  const mutationOptions = {
    onSuccess: markEmailContactVerified,
    onError: (error: Error) => toast.error(error.message),
  }
  if (current) {
    updateContactMutation.mutate({ contactId: current.id, payload }, mutationOptions)
    return
  }
  createContactMutation.mutate(payload, mutationOptions)
}

function confirmContactEmailVerification() {
  confirmEmailVerificationMutation.mutate({
    email: emailForm.email,
    code: emailForm.code,
  }, {
    onSuccess: () => {
      emailForm.code = ''
      saveVerifiedEmailContact()
    },
    onError: error => toast.error(error instanceof Error ? error.message : '邮箱绑定失败。'),
  })
}

function removeContact(contact: UserContactMethod) {
  deleteContactMutation.mutate(contact.id, {
    onSuccess: () => {
      if (contact.type === 'wechat') wechatForm.displayValue = ''
      if (contact.type === 'email') {
        emailForm.email = profile.value?.email ?? ''
        emailForm.code = ''
      }
      toast.success('联系方式已解除绑定。')
    },
    onError: error => toast.error(error instanceof Error ? error.message : '删除失败'),
  })
}

function saveApiPaymentSettings() {
  if (!apiPaymentComplete.value) {
    toast.warning(apiPaymentMissingReasonText.value || '请先补全 API 收款设置。')
    return
  }
  if (containsSensitiveContent(apiPaymentForm.paymentOptions.map(option => option.paymentInstructions))) {
    toast.warning('收款说明不能包含 API Key、token、密码、Session、Cookie、付款码或面板凭据。')
    return
  }
  updateApiPaymentSettingsMutation.mutate({
    paymentWindowMinutes: defaultApiPaymentWindowMinutes,
    paymentOptions: apiPaymentForm.paymentOptions.map(option => ({
      ...option,
      paymentInstructions: option.paymentInstructions.trim(),
    })),
  }, {
    onSuccess: () => toast.success('API 收款设置已保存。'),
    onError: error => toast.error(error instanceof Error ? error.message : 'API 收款设置保存失败。'),
  })
}

function setDefaultContact(contact: UserContactMethod) {
  setDefaultContactMutation.mutate(contact.id, {
    onSuccess: () => toast.success('默认联系方式已更新。'),
    onError: error => toast.error(error instanceof Error ? error.message : '设置失败'),
  })
}

function apiServiceStatusLabel(state: string, online: boolean) {
  if (online) return '在线'
  if (state === 'reviewing') return '审核中'
  if (state === 'paused') return '暂停'
  return '离线'
}

function apiServiceStatusVariant(state: string, online: boolean) {
  if (online) return 'default'
  if (state === 'reviewing') return 'secondary'
  if (state === 'paused') return 'secondary'
  return 'outline'
}

function apiServicePublicDetailUrl(item: ApiService) {
  return getApiServicePublicDetailUrl(item)
}

function publishService(id: string) {
  publishApiServiceMutation.mutate(id, {
    onSuccess: () => toast.success('API 服务已上线。'),
    onError: error => toast.error(error instanceof Error ? error.message : '上线失败。'),
  })
}

function pauseService(id: string) {
  pauseApiServiceMutation.mutate(id, {
    onSuccess: () => toast.success('API 服务已暂停。'),
    onError: error => toast.error(error instanceof Error ? error.message : '暂停失败。'),
  })
}

function resumeService(id: string) {
  resumeApiServiceMutation.mutate(id, {
    onSuccess: () => toast.success('API 服务已恢复上线。'),
    onError: error => toast.error(error instanceof Error ? error.message : '恢复失败。'),
  })
}

function goToLogin() {
  router.push({ path: '/login', query: { returnTo: route.fullPath } })
}
</script>

<template>
  <div v-if="profileQuery.isPending.value" class="rounded-xl border border-border bg-card p-8 text-sm text-muted-foreground">正在加载个人资料...</div>
  <Card v-else-if="profileQuery.isError.value || !profile" class="mx-auto max-w-2xl p-6">
    <div class="flex flex-col gap-4 sm:flex-row sm:items-start">
      <div class="grid h-11 w-11 shrink-0 place-items-center rounded-xl bg-primary/10 text-primary">
        <LogIn class="h-5 w-5" />
      </div>
      <div class="min-w-0 flex-1">
        <h1 class="text-lg font-semibold tracking-tight">请先登录后查看我的中心</h1>
        <p class="mt-2 text-sm leading-6 text-muted-foreground">
          {{ profileErrorMessage }}
        </p>
        <div class="mt-5 flex flex-wrap gap-2">
          <Button @click="goToLogin"><LogIn class="h-4 w-4" />去登录</Button>
          <Button variant="outline" :disabled="profileQuery.isFetching.value" @click="profileQuery.refetch()">
            <RefreshCw class="h-4 w-4" :class="profileQuery.isFetching.value ? 'animate-spin' : ''" />
            重新读取
          </Button>
        </div>
      </div>
    </div>
  </Card>
  <div v-else class="space-y-5">
    <Card class="p-5">
      <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div class="flex min-w-0 gap-4">
          <div class="grid h-16 w-16 shrink-0 place-items-center overflow-hidden rounded-full bg-primary text-xl font-semibold text-primary-foreground">
            <img v-if="profile.avatarUrl" :src="profile.avatarUrl" alt="当前头像" class="h-full w-full object-cover" />
            <span v-else>{{ avatarText }}</span>
          </div>
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <h1 class="text-2xl font-semibold tracking-tight">{{ profile.displayName }}</h1>
              <Badge v-for="badge in profile.badges" :key="badge.id" :variant="badge.type === 'system' ? 'default' : 'secondary'">{{ badge.label }}</Badge>
            </div>
            <p class="mt-1 text-sm text-muted-foreground">
              @{{ profile.username }} · linux.do @{{ profile.linuxDoBinding.linuxDoUsername }} · 信任等级{{ profile.linuxDoBinding.trustLevel }}
            </p>
            <p class="mt-2 max-w-3xl text-sm text-muted-foreground">{{ profile.bio }}</p>
          </div>
        </div>
        <div class="flex flex-wrap gap-2">
          <Button variant="outline" @click="router.push(`/u/${profile.username}`)"><Eye class="h-4 w-4" />查看公开主页</Button>
          <Button @click="router.push('/my/profile')"><UserRound class="h-4 w-4" />编辑个人资料</Button>
        </div>
      </div>
    </Card>

    <nav class="flex gap-2 overflow-x-auto pb-1" aria-label="我的中心二级导航">
      <RouterLink
        v-for="item in sectionLinks"
        :key="item.to"
        :to="item.to"
        class="shrink-0 rounded-md border px-3 py-2 text-sm transition"
        :class="isSectionActive(item.to) ? 'border-primary bg-primary text-primary-foreground' : 'border-border bg-card text-muted-foreground hover:bg-accent hover:text-foreground'"
      >
        {{ item.label }}
      </RouterLink>
    </nav>

    <section v-if="activeSection === 'overview'" class="space-y-5">
      <div class="grid gap-3 md:grid-cols-4">
        <RouterLink to="/my/carpools"><StatCard label="我的开车" :value="String(carpoolRows.length)" hint="仅展示当前用户车源" :icon="Car" accent /></RouterLink>
        <RouterLink to="/my/rides"><StatCard label="我的上车" value="5" hint="1 个待完成" :icon="UsersRound" /></RouterLink>
        <RouterLink to="/my/api-orders"><StatCard label="API 意向" value="2" hint="1 个站外确认中" :icon="ShoppingBag" /></RouterLink>
        <RouterLink to="/my/contacts"><StatCard label="启用联系方式" :value="String(enabledContactCount)" hint="只在私有页和联系窗口内可见" :icon="ContactRound" /></RouterLink>
      </div>

      <div class="grid gap-5 lg:grid-cols-2">
        <section class="space-y-3">
          <h2 class="font-semibold">我的开车</h2>
          <SoftTable :columns="['车源', '价格', '车位', '状态']">
            <tr v-for="item in carpoolPagination.paginatedRows.value" :key="item.id">
              <td>{{ item.product }}</td>
              <td>{{ getPricingDisplay(item).primaryLabel }} ¥{{ getPricingDisplay(item).primaryPrice }}</td>
              <td>剩余 {{ getRemainingSeats(item) }} 位</td>
              <td><Badge>{{ item.status }}</Badge></td>
            </tr>
            <tr v-if="carpoolRows.length === 0"><td colspan="4" class="py-8 text-center text-sm text-muted-foreground">暂无当前用户车源。</td></tr>
            <template #footer>
              <TablePagination v-model:page="carpoolPagination.page.value" :page-count="carpoolPagination.pageCount.value" :total="carpoolPagination.total.value" :start-item="carpoolPagination.startItem.value" :end-item="carpoolPagination.endItem.value" />
            </template>
          </SoftTable>
        </section>

        <section class="space-y-3">
          <h2 class="font-semibold">API 服务</h2>
          <SoftTable :columns="['服务', '对外商家名', '额度', '状态', '操作']">
            <tr v-for="item in apiServicePagination.paginatedRows.value" :key="item.id">
              <td>{{ item.title }}</td>
              <td>
                <div>{{ getApiMerchantDisplayName(item) }}</div>
                <div class="text-xs text-muted-foreground">{{ getApiMerchantVisibilityLabel(item) }}</div>
              </td>
              <td>¥{{ item.balance }}</td>
              <td><Badge :variant="apiServiceStatusVariant(item.state, item.online)">{{ apiServiceStatusLabel(item.state, item.online) }}</Badge></td>
              <td>
                <div class="flex flex-wrap gap-2">
                  <Button v-if="item.state === 'offline'" size="sm" @click="publishService(item.id)">上线</Button>
                  <Button v-if="item.online" size="sm" variant="outline" @click="pauseService(item.id)">暂停</Button>
                  <Button v-if="item.state === 'paused'" size="sm" variant="outline" @click="resumeService(item.id)">恢复</Button>
                  <RouterLink v-if="apiServicePublicDetailUrl(item)" :to="apiServicePublicDetailUrl(item)!"><Button size="sm" variant="outline">查看</Button></RouterLink>
                  <Button v-else size="sm" variant="outline" disabled>待配置接单</Button>
                </div>
              </td>
            </tr>
            <tr v-if="apiServiceRows.length === 0"><td colspan="5" class="py-8 text-center text-sm text-muted-foreground">暂无当前用户 API 服务。</td></tr>
            <template #footer>
              <TablePagination v-model:page="apiServicePagination.page.value" :page-count="apiServicePagination.pageCount.value" :total="apiServicePagination.total.value" :start-item="apiServicePagination.startItem.value" :end-item="apiServicePagination.endItem.value" />
            </template>
          </SoftTable>
        </section>
      </div>
    </section>

    <section v-else-if="activeSection === 'profile'" class="grid gap-5 xl:grid-cols-[1fr_360px]">
      <Card class="p-5">
        <h2 class="font-semibold">个人资料设置</h2>
        <div class="mt-5 grid gap-4 md:grid-cols-2">
          <label class="space-y-2"><span class="text-sm font-medium">显示名称</span><Input v-model="profileForm.displayName" /></label>
          <label class="space-y-2"><span class="text-sm font-medium">站内用户名</span><Input v-model="profileForm.username" /></label>
          <label class="space-y-2 md:col-span-2"><span class="text-sm font-medium">个人简介</span><Textarea v-model="profileForm.bio" class="min-h-28" maxlength="300" /></label>
          <label class="space-y-2"><span class="text-sm font-medium">常用地区</span><Input v-model="profileForm.regionCode" placeholder="cn-east / hk / jp" /></label>
          <label class="space-y-2">
            <span class="text-sm font-medium">时区</span>
            <Select v-model="profileForm.timezone">
              <SelectTrigger class="w-full bg-background"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="Asia/Shanghai">Asia/Shanghai</SelectItem>
                <SelectItem value="Asia/Hong_Kong">Asia/Hong_Kong</SelectItem>
                <SelectItem value="Asia/Tokyo">Asia/Tokyo</SelectItem>
                <SelectItem value="America/Los_Angeles">America/Los_Angeles</SelectItem>
              </SelectContent>
            </Select>
          </label>
        </div>
        <Button class="mt-5" :disabled="updateProfileMutation.isPending.value" @click="saveProfile"><Save class="h-4 w-4" />保存个人资料</Button>
      </Card>

      <Card class="p-5">
        <h2 class="font-semibold">头像</h2>
        <div class="mt-4 space-y-3">
          <label class="flex items-center gap-2 text-sm"><input v-model="profileForm.avatarMode" type="radio" value="linuxdo" />跟随 linux.do 头像</label>
          <label class="flex items-center gap-2 text-sm"><input v-model="profileForm.avatarMode" type="radio" value="custom_url" />使用 HTTPS 图片 URL</label>
          <label class="space-y-2">
            <span class="text-sm font-medium">自定义头像 URL</span>
            <Input v-model="profileForm.avatarUrl" :disabled="profileForm.avatarMode !== 'custom_url'" placeholder="https://example.com/avatar.webp" />
            <span class="text-xs text-muted-foreground">仅支持 HTTPS 图片链接。</span>
          </label>
          <div class="flex flex-wrap gap-2">
            <Button size="sm" variant="outline" @click="useLinuxDoAvatarMutation.mutate()"><Link2 class="h-4 w-4" />恢复 linux.do</Button>
            <Button size="sm" variant="outline" @click="deleteAvatarMutation.mutate()"><Trash2 class="h-4 w-4" />删除自定义头像</Button>
            <Button size="sm" @click="saveProfile"><Save class="h-4 w-4" />保存头像来源</Button>
          </div>
        </div>
      </Card>
    </section>

    <section v-else-if="activeSection === 'contacts'" class="space-y-4">
      <Card class="p-5">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div class="flex min-w-0 gap-4">
            <div class="grid h-12 w-12 shrink-0 place-items-center rounded-xl bg-primary/10 text-primary">
              <MessageCircle class="h-5 w-5" />
            </div>
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <h2 class="text-lg font-semibold tracking-tight">微信</h2>
                <Badge :variant="wechatBound ? 'verified' : 'secondary'">{{ wechatBound ? '已绑定' : '未绑定' }}</Badge>
                <Badge v-if="wechatContact?.isDefault" variant="secondary">默认联系方式</Badge>
              </div>
              <p class="mt-1 text-sm leading-6 text-muted-foreground">填写微信号后即可作为联系窗口方式，不做验证码验证。</p>
              <p v-if="wechatContact" class="mt-2 text-sm text-muted-foreground">
                当前：{{ wechatContact.maskedValue }} · 适用：{{ scopeLabels(wechatContact.usageScopes) }}
              </p>
            </div>
          </div>
          <div v-if="wechatContact" class="flex flex-wrap gap-2">
            <Button v-if="!wechatContact.isDefault" size="sm" variant="outline" :disabled="setDefaultContactMutation.isPending.value" @click="setDefaultContact(wechatContact)">设为默认</Button>
            <Button size="sm" variant="outline" :disabled="deleteContactMutation.isPending.value" @click="removeContact(wechatContact)"><Trash2 class="h-4 w-4" />解除绑定</Button>
          </div>
        </div>
        <div class="mt-5 grid gap-3 lg:grid-cols-[minmax(0,1fr)_160px]">
          <label class="space-y-2">
            <span class="text-sm font-medium">微信号</span>
            <Input v-model="wechatForm.displayValue" autocomplete="off" placeholder="例如 c2c_orbit" />
          </label>
          <Button class="lg:self-end" :disabled="contactSaving || !wechatForm.displayValue.trim()" @click="saveWechatContact"><Save class="h-4 w-4" />保存微信</Button>
        </div>
      </Card>

      <Card class="p-5">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div class="flex min-w-0 gap-4">
            <div class="grid h-12 w-12 shrink-0 place-items-center rounded-xl bg-sky-500/10 text-sky-700">
              <Mail class="h-5 w-5" />
            </div>
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <h2 class="text-lg font-semibold tracking-tight">邮箱</h2>
                <Badge :variant="emailBound ? 'verified' : 'secondary'">{{ emailBound ? '已绑定' : '未绑定' }}</Badge>
                <Badge v-if="emailContact && !emailContact.verified" variant="secondary">待验证</Badge>
                <Badge v-if="emailContact?.isDefault" variant="secondary">默认联系方式</Badge>
              </div>
              <p class="mt-1 text-sm leading-6 text-muted-foreground">邮箱必须通过验证码后才会启用为联系方式。</p>
              <p v-if="emailContact" class="mt-2 text-sm text-muted-foreground">
                当前：{{ emailContact.maskedValue }} · 适用：{{ scopeLabels(emailContact.usageScopes) }}
              </p>
            </div>
          </div>
          <div v-if="emailContact" class="flex flex-wrap gap-2">
            <Button v-if="emailBound && !emailContact.isDefault" size="sm" variant="outline" :disabled="setDefaultContactMutation.isPending.value" @click="setDefaultContact(emailContact)">设为默认</Button>
            <Button size="sm" variant="outline" :disabled="deleteContactMutation.isPending.value" @click="removeContact(emailContact)"><Trash2 class="h-4 w-4" />解除绑定</Button>
          </div>
        </div>
        <div class="mt-5 grid gap-3 lg:grid-cols-[minmax(0,1fr)_160px]">
          <label class="space-y-2">
            <span class="text-sm font-medium">邮箱地址</span>
            <Input v-model="emailForm.email" type="email" autocomplete="email" placeholder="name@example.com" />
          </label>
          <Button class="lg:self-end" variant="outline" :disabled="emailBindingPending || !emailForm.email.trim()" @click="startEmailVerification"><MailCheck class="h-4 w-4" />发送验证码</Button>
          <label class="space-y-2">
            <span class="text-sm font-medium">验证码</span>
            <Input v-model="emailForm.code" inputmode="numeric" maxlength="6" placeholder="6 位验证码" />
          </label>
          <Button class="lg:self-end" :disabled="emailBindingPending || !emailForm.code.trim()" @click="confirmContactEmailVerification">验证并绑定邮箱</Button>
        </div>
      </Card>

      <Card class="border-emerald-200 bg-emerald-50/40 p-4">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div class="flex min-w-0 gap-4">
            <div class="grid h-10 w-10 shrink-0 place-items-center rounded-lg bg-emerald-500/10 text-emerald-700">
              <CreditCard class="h-5 w-5" />
            </div>
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <h2 class="text-lg font-semibold tracking-tight">API 收款设置</h2>
                <Badge :variant="apiPaymentComplete ? 'verified' : 'secondary'">{{ apiPaymentComplete ? '已配置' : '待配置' }}</Badge>
              </div>
              <p class="mt-1 text-sm leading-6 text-muted-foreground">
                发布 API 额度时默认使用这里的收款资料，并在发布时复制为该服务的接单快照。
              </p>
              <p class="mt-2 text-sm text-muted-foreground">{{ apiPaymentSummaryText }}</p>
            </div>
          </div>
          <Button :disabled="updateApiPaymentSettingsMutation.isPending.value" @click="saveApiPaymentSettings">
            <Save class="h-4 w-4" />保存 API 收款设置
          </Button>
        </div>

        <div class="mt-4 space-y-3">
          <div class="flex flex-col gap-2 rounded-md border border-emerald-200 bg-white/70 px-3 py-2 text-sm sm:flex-row sm:items-center sm:justify-between">
            <span class="font-medium">买家确认付款窗口</span>
            <span class="text-muted-foreground">固定 {{ defaultApiPaymentWindowMinutes }} 分钟</span>
          </div>

          <div class="space-y-2">
            <div
              v-for="option in apiPaymentForm.paymentOptions"
              :key="option.paymentMethod"
              class="grid gap-3 rounded-md border bg-white/80 p-3 lg:grid-cols-[minmax(170px,220px)_minmax(0,1fr)_auto]"
              :class="option.enabled ? 'border-primary/35' : 'border-border'"
            >
              <label class="flex cursor-pointer items-start gap-3 lg:items-center">
                <input v-model="option.enabled" type="checkbox" class="mt-1 h-4 w-4 accent-primary" />
                <span class="min-w-0">
                  <strong class="block text-sm">{{ apiPaymentMethodLabel(option.paymentMethod) }}</strong>
                  <span class="mt-1 block text-xs leading-5 text-muted-foreground">
                    {{ apiPaymentMethods.find(item => item.value === option.paymentMethod)?.hint }}
                  </span>
                </span>
              </label>

              <div v-if="option.enabled" class="min-w-0">
                <div v-if="apiPaymentMethodRequiresQrCode(option.paymentMethod)" class="flex flex-col gap-3 sm:flex-row sm:items-center">
                  <div class="grid h-20 w-20 shrink-0 place-items-center overflow-hidden rounded-md border border-border bg-muted/40">
                    <img v-if="option.paymentQrCodeDataUrl" :src="option.paymentQrCodeDataUrl" :alt="`${apiPaymentMethodLabel(option.paymentMethod)}收款码`" class="h-full w-full object-cover" />
                    <ImageUp v-else class="h-5 w-5 text-muted-foreground" />
                  </div>
                  <div class="min-w-0 flex-1 space-y-2">
                    <div class="flex flex-wrap gap-2">
                      <input
                        :id="`api-payment-qr-${option.paymentMethod}`"
                        class="sr-only"
                        type="file"
                        accept="image/png,image/jpeg,image/webp"
                        @change="handleApiPaymentQrUpload($event, option)"
                      />
                      <label
                        :for="`api-payment-qr-${option.paymentMethod}`"
                        class="inline-flex h-9 cursor-pointer items-center justify-center gap-2 rounded-md border border-input bg-background px-3 text-sm font-medium shadow-xs hover:bg-accent hover:text-accent-foreground"
                      >
                        <ImageUp class="h-4 w-4" />{{ option.paymentQrCodeDataUrl ? '替换收款码' : '上传收款码' }}
                      </label>
                      <Button v-if="option.paymentQrCodeDataUrl" type="button" size="sm" variant="outline" @click="removeApiPaymentQr(option)">
                        <X class="h-4 w-4" />移除
                      </Button>
                    </div>
                    <p class="text-xs leading-5 text-muted-foreground">支持 PNG、JPG、WebP，最多 512KB。</p>
                  </div>
                </div>

                <Textarea
                  v-model="option.paymentInstructions"
                  class="mt-3 min-h-16 text-sm"
                  maxlength="160"
                  :placeholder="apiPaymentInstructionsPlaceholder(option.paymentMethod)"
                />
              </div>
              <div v-else class="text-sm text-muted-foreground lg:self-center">未启用</div>

              <Badge class="lg:self-center" :variant="option.enabled && apiPaymentOptionReady(option) ? 'verified' : 'secondary'">
                {{ option.enabled && apiPaymentOptionReady(option) ? '已就绪' : option.enabled ? '待补全' : '未启用' }}
              </Badge>
            </div>
          </div>
        </div>

        <p
          class="mt-4 rounded-md border px-3 py-2 text-xs leading-5"
          :class="apiPaymentComplete ? 'border-success/20 bg-success/5 text-success' : 'border-warning/25 bg-warning/10 text-warning'"
        >
          {{ apiPaymentComplete ? 'API 发布页将直接读取这组设置，不需要每次重新填写。' : apiPaymentMissingReasonText }}
        </p>
        <p class="mt-2 rounded-md border border-border bg-accent/50 p-3 text-xs leading-5 text-muted-foreground">
          收款资料只在买家提交购买意向后用于站外确认；不要填写付款码、银行卡号、API Key、token、账号密码、Cookie、Session 或面板凭据。
        </p>
      </Card>

      <p class="rounded-md border border-border bg-accent/50 p-3 text-xs leading-5 text-muted-foreground">
        当前只开放微信和邮箱两种联系方式。联系方式只用于参与方之间的站外联系；公开主页、首页、车源列表和 API 市集不会展示完整联系方式。
      </p>
    </section>

    <section v-else-if="activeSection === 'account'" class="grid gap-4 lg:grid-cols-2">
      <Card class="p-5">
        <h2 class="font-semibold">linux.do 身份绑定</h2>
        <div class="mt-4 space-y-3 text-sm">
          <div class="flex justify-between gap-4"><span class="text-muted-foreground">linux.do 绑定状态</span><span>{{ profile.linuxDoBinding.bound ? '已绑定 linux.do' : '未绑定' }}</span></div>
          <div class="flex justify-between gap-4"><span class="text-muted-foreground">linux.do 用户名</span><span>@{{ profile.linuxDoBinding.linuxDoUsername }}</span></div>
          <div class="flex justify-between gap-4"><span class="text-muted-foreground">linux.do 用户 ID</span><span>{{ profile.linuxDoBinding.linuxDoUserId }}</span></div>
          <div class="flex justify-between gap-4"><span class="text-muted-foreground">信任等级</span><span>{{ profile.linuxDoBinding.trustLevel }}</span></div>
          <div class="flex justify-between gap-4"><span class="text-muted-foreground">头像同步</span><span>{{ profile.avatarMode === 'linuxdo' ? '跟随 linux.do' : '自定义头像' }}</span></div>
          <div class="flex justify-between gap-4"><span class="text-muted-foreground">最近同步</span><span>{{ profile.linuxDoBinding.lastSyncedAt }}</span></div>
          <div class="flex justify-between gap-4"><span class="text-muted-foreground">账号状态</span><span>{{ profile.accountStatus }}</span></div>
          <div class="flex justify-between gap-4"><span class="text-muted-foreground">绑定邮箱</span><span>{{ profile.emailVerified ? profile.email : '未绑定' }}</span></div>
          <div class="flex justify-between gap-4"><span class="text-muted-foreground">备用密码</span><span>{{ profile.passwordConfigured ? '已设置' : '未设置' }}</span></div>
        </div>
        <p class="mt-4 rounded-md border border-border bg-accent/50 p-3 text-xs leading-5 text-muted-foreground">
          linux.do 绑定不可自助解绑或换绑；异常情况请联系管理员人工处理。
        </p>
      </Card>
      <Card class="p-5">
        <h2 class="font-semibold">备用密码</h2>
        <div class="mt-4 space-y-3">
          <label v-if="profile.passwordConfigured" class="space-y-2"><span class="text-sm font-medium">当前密码</span><Input v-model="passwordForm.currentPassword" type="password" autocomplete="current-password" /></label>
          <label class="space-y-2"><span class="text-sm font-medium">新密码</span><Input v-model="passwordForm.newPassword" type="password" autocomplete="new-password" /></label>
          <label class="space-y-2"><span class="text-sm font-medium">确认新密码</span><Input v-model="passwordForm.confirmPassword" type="password" autocomplete="new-password" /></label>
          <Button :disabled="setPasswordMutation.isPending.value" @click="savePassword"><LockKeyhole class="h-4 w-4" />{{ profile.passwordConfigured ? '修改备用密码' : '设置备用密码' }}</Button>
        </div>
      </Card>
      <Card class="p-5">
        <h2 class="font-semibold">邮箱绑定</h2>
        <div class="mt-4 space-y-3">
          <label class="space-y-2"><span class="text-sm font-medium">邮箱</span><Input v-model="emailForm.email" type="email" autocomplete="email" /></label>
          <div class="flex flex-wrap gap-2">
            <Button variant="outline" :disabled="startEmailVerificationMutation.isPending.value" @click="startEmailVerification"><MailCheck class="h-4 w-4" />发送验证码</Button>
            <Badge :variant="profile.emailVerified ? 'verified' : 'secondary'">{{ profile.emailVerified ? '邮箱已绑定' : '邮箱未绑定' }}</Badge>
          </div>
          <label class="space-y-2"><span class="text-sm font-medium">验证码</span><Input v-model="emailForm.code" inputmode="numeric" maxlength="6" placeholder="6 位验证码" /></label>
          <Button :disabled="confirmEmailVerificationMutation.isPending.value" @click="confirmEmailVerification">确认绑定邮箱</Button>
        </div>
      </Card>
      <Card class="p-5">
        <h2 class="font-semibold">系统铭牌与限制</h2>
        <div class="mt-4 flex flex-wrap gap-2"><Badge v-for="badge in profile.badges" :key="badge.id" variant="secondary">{{ badge.label }}</Badge></div>
        <div class="mt-4 rounded-md border border-border bg-accent/50 p-3 text-sm text-muted-foreground">
          当前功能限制：{{ profile.restrictions.length ? profile.restrictions.join('、') : '无' }}。
        </div>
        <div class="mt-4 flex flex-wrap gap-2">
          <Button variant="outline" @click="toast('linux.do 信息同步请求已记录。')">同步 linux.do 信息</Button>
          <Button variant="outline" @click="router.push('/my/profile')">切换头像跟随模式</Button>
          <Button variant="outline" @click="toast('申诉请求已记录。')"><LockKeyhole class="h-4 w-4" />提交申诉</Button>
        </div>
      </Card>
    </section>

    <section v-else-if="activeSection === 'privacy'" class="grid gap-4 lg:grid-cols-2">
      <Card class="p-5">
        <h2 class="font-semibold">公开主页隐私设置</h2>
        <div class="mt-4 space-y-3">
          <label class="flex items-center justify-between gap-4 text-sm"><span>展示最近活跃时间</span><input v-model="privacyForm.showLastActiveAt" type="checkbox" /></label>
          <label class="flex items-center justify-between gap-4 text-sm"><span>展示加入时间</span><input v-model="privacyForm.showCreatedAt" type="checkbox" /></label>
          <label class="flex items-center justify-between gap-4 text-sm"><span>展示近 30 天完成数量</span><input v-model="privacyForm.showCompletionStats" type="checkbox" /></label>
          <label class="flex items-center justify-between gap-4 text-sm"><span>展示响应中位时间</span><input v-model="privacyForm.showResponseMedian" type="checkbox" /></label>
          <label class="flex items-center justify-between gap-4 text-sm"><span>展示已处理纠纷摘要</span><input v-model="privacyForm.showResolvedDisputeSummary" type="checkbox" /></label>
          <label class="flex items-center justify-between gap-4 text-sm"><span>允许他人从公开主页举报</span><input v-model="privacyForm.allowPublicProfileReport" type="checkbox" /></label>
        </div>
        <Button class="mt-5" @click="savePrivacy"><Save class="h-4 w-4" />保存隐私设置</Button>
      </Card>
      <Card class="p-5">
        <h2 class="font-semibold">不能关闭的公开信号</h2>
        <div class="mt-4 space-y-3 text-sm text-muted-foreground">
          <p>账号处罚状态、严重未解决纠纷提示、系统认证铭牌和已绑定 linux.do 状态始终会在必要位置展示。</p>
          <p>隐私设置不影响有效意向参与者查看必要联系方式。</p>
          <p>公开主页不会展示微信、邮箱、登录邮箱、手机号、IP、设备信息或意向敏感详情。</p>
        </div>
      </Card>
    </section>
  </div>
</template>
