<script setup lang="ts">
import { computed, onUnmounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { toast } from 'vue-sonner'
import { Bell, CheckCircle2, Eye, KeyRound, Link2, LockKeyhole, LogIn, Mail, MailCheck, MessageCircle, RefreshCw, Save, ShieldAlert, ShieldCheck, Star, Trash2, TriangleAlert } from 'lucide-vue-next'
import AccountSettingsShell from '@/components/account-settings/AccountSettingsShell.vue'
import ApiPaymentSettingsEditor from '@/components/contact-payment/ApiPaymentSettingsEditor.vue'
import BuyerPreviewDrawer from '@/components/contact-payment/BuyerPreviewDrawer.vue'
import ConfigurationProgressCard from '@/components/contact-payment/ConfigurationProgressCard.vue'
import ContactMethodCard from '@/components/contact-payment/ContactMethodCard.vue'
import ContactUsageScopeSelector from '@/components/contact-payment/ContactUsageScopeSelector.vue'
import PasswordVisibilityInput from '@/components/auth/PasswordInput.vue'
import PersonalCenterDashboard from '@/components/personal-center/PersonalCenterDashboard.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { useUnsavedChangesGuard } from '@/composables/useUnsavedChangesGuard'
import { type AvatarMode, type ContactUsageScope, type UserContactMethod, type UserPrivacySettings, type UserProfile } from '@/lib/api'
import { accountRecoveryRequirements, isAccountRecoveryComplete, sanitizeAccountRecoveryReturnTo } from '@/lib/accountRecovery'
import { getBackupPasswordStrength, getBackupPasswordValidationMessage, getPasswordChecks } from '@/lib/passwordPolicy'
import {
  createEmptyApiPaymentAccountSettings,
  isApiPaymentAccountSettingsComplete,
  isApiPaymentOptionComplete,
} from '@/lib/apiPaymentSettings'
import {
  buildAccountCompleteness,
  buildPendingTasks,
  buildPublishedContent,
  countActivePublishedContent,
  getPrimaryAccountAlert,
  shouldShowFirstTransactionGuide,
  uniqueRelatedApiOrderCount,
  type PersonalCenterMetric,
} from '@/lib/personalCenterDashboard'
import {
  useApiPaymentAccountSettingsQuery,
  useConfirmContactEmailVerificationMutation,
  useConfirmEmailVerificationMutation,
  useCreateContactMethodMutation,
  useDeleteContactMethodMutation,
  useDeleteCustomAvatarMutation,
  useMerchantApiOrders,
  useMerchantCarpoolApplications,
  useMyContactMethodsQuery,
  useMyApiServices,
  useMyApiOrders,
  useMyCarpoolApplications,
  useMyCarpools,
  useMyProfileQuery,
  useSetBackupPasswordMutation,
  useSetDefaultContactMethodMutation,
  useStartContactEmailVerificationMutation,
  useStartEmailVerificationMutation,
  useUpdateContactMethodMutation,
  useUpdateMyProfileMutation,
  useUseLinuxDoAvatarMutation,
} from '@/queries/useMarketQueries'
import { CAPABILITY, hasCapability } from '@/lib/capabilities'
import { backendErrorMessage, reauthenticatePassword, startLinuxDoLink } from '@/lib/backendClient'
import { LIMITED_API_QUOTA_OFFERS_ENABLED } from '@/lib/featureFlags'
import {
  buildContactMethodPayload,
  ALL_CONTACT_USAGE_SCOPES,
  CONTACT_USAGE_SCOPE_OPTIONS,
  contactUsageScopeOptionsForCapabilities,
  initialContactUsageScopes,
  sameContactUsageScopes,
} from '@/lib/contactUsageScopes'

const route = useRoute()
const router = useRouter()
const profileQuery = useMyProfileQuery()
const profile = profileQuery.data
const canPublishCarpool = computed(() => hasCapability(profile.value, CAPABILITY.carpoolPublish))
const canPublishApiService = computed(() => hasCapability(profile.value, CAPABILITY.apiServicePublish))
const canPublishAnything = computed(() => canPublishCarpool.value || canPublishApiService.value)
const contactsQuery = useMyContactMethodsQuery()
const contacts = contactsQuery.data
const apiPaymentSettingsQuery = useApiPaymentAccountSettingsQuery(canPublishApiService)
const apiPaymentSettings = apiPaymentSettingsQuery.data
const carpoolsQuery = useMyCarpools(canPublishCarpool)
const carpools = carpoolsQuery.data
const apiServicesQuery = useMyApiServices('all', canPublishApiService)
const apiServices = apiServicesQuery.data
const buyerRideApplicationsQuery = useMyCarpoolApplications({ sort: 'default_buyer' })
const rideApplications = buyerRideApplicationsQuery.data
const ownerRideApplicationsQuery = useMerchantCarpoolApplications({ sort: 'default_owner' }, canPublishCarpool)
const ownerRideApplications = ownerRideApplicationsQuery.data
const buyerApiOrdersQuery = useMyApiOrders({ sort: 'default_buyer' })
const apiOrders = buyerApiOrdersQuery.data
const merchantApiOrdersQuery = useMerchantApiOrders({ sort: 'default_merchant' }, canPublishApiService)
const merchantApiOrders = merchantApiOrdersQuery.data

const updateProfileMutation = useUpdateMyProfileMutation()
const deleteAvatarMutation = useDeleteCustomAvatarMutation()
const useLinuxDoAvatarMutation = useUseLinuxDoAvatarMutation()
const setPasswordMutation = useSetBackupPasswordMutation()
const startEmailVerificationMutation = useStartEmailVerificationMutation()
const confirmEmailVerificationMutation = useConfirmEmailVerificationMutation()
const startContactEmailVerificationMutation = useStartContactEmailVerificationMutation()
const confirmContactEmailVerificationMutation = useConfirmContactEmailVerificationMutation()
const createContactMutation = useCreateContactMethodMutation()
const updateContactMutation = useUpdateContactMethodMutation()
const deleteContactMutation = useDeleteContactMethodMutation()
const setDefaultContactMutation = useSetDefaultContactMethodMutation()

type AccountSetupDialogMode = 'required' | 'password' | 'email'
type AccountSetupStep = 'email' | 'password' | 'complete'
type AccountSetupStepItem = {
  id: AccountSetupStep
  step: number
  label: string
  completed: boolean
  active: boolean
}

const activeSection = computed(() => {
  if (route.path === '/my/profile') return 'profile'
  if (route.path === '/my/contacts') return 'contacts'
  if (route.path === '/my/account') return 'account'
  if (route.path === '/my/privacy') return 'privacy'
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
const confirmPasswordTouched = ref(false)

const accountEmailForm = reactive({
  email: '',
  code: '',
})

const contactEmailForm = reactive({
  email: '',
  code: '',
  usageScopes: [] as ContactUsageScope[],
})

const privacyForm = reactive<UserPrivacySettings>({
  showCreatedAt: true,
  showLastActiveAt: true,
  showCompletionStats: true,
  showResponseMedian: true,
  showResolvedDisputeSummary: true,
  allowPublicProfileReport: true,
})

const profileSnapshot = ref('')
const privacySnapshot = ref('')

function profileFormSignature() {
  return JSON.stringify({
    displayName: profileForm.displayName,
    username: profileForm.username,
    bio: profileForm.bio,
    regionCode: profileForm.regionCode,
    timezone: profileForm.timezone,
    avatarMode: profileForm.avatarMode,
    avatarUrl: profileForm.avatarUrl,
  })
}

function privacyFormSignature() {
  return JSON.stringify({
    showCreatedAt: privacyForm.showCreatedAt,
    showLastActiveAt: privacyForm.showLastActiveAt,
    showCompletionStats: privacyForm.showCompletionStats,
    showResponseMedian: privacyForm.showResponseMedian,
    showResolvedDisputeSummary: privacyForm.showResolvedDisputeSummary,
    allowPublicProfileReport: privacyForm.allowPublicProfileReport,
  })
}

const profileSettingsDirty = computed(() => Boolean(profileSnapshot.value) && profileFormSignature() !== profileSnapshot.value)
const privacySettingsDirty = computed(() => Boolean(privacySnapshot.value) && privacyFormSignature() !== privacySnapshot.value)

const wechatForm = reactive({
  displayValue: '',
})

const wechatContactSnapshot = ref('')
const emailContactSnapshot = ref('')

function wechatFormSignature() {
  return wechatForm.displayValue.trim()
}

function emailContactFormSignature() {
  return JSON.stringify({
    email: contactEmailForm.email.trim().toLowerCase(),
    usageScopes: [...contactEmailForm.usageScopes].sort(),
  })
}

const availableContactUsageScopeOptions = computed(() => contactUsageScopeOptionsForCapabilities({
  canPublishCarpool: canPublishCarpool.value,
  canPublishApiService: canPublishApiService.value,
}))
const defaultContactUsageScopes = computed<ContactUsageScope[]>(() => (
  availableContactUsageScopeOptions.value.map(option => option.value)
))

const wechatContact = computed(() => (contacts.value ?? []).find(item => item.type === 'wechat') ?? null)
const emailContact = computed(() => (contacts.value ?? []).find(item => item.type === 'email') ?? null)
const enabledContactCount = computed(() => [wechatContact.value, emailContact.value].filter(item => item?.enabled && (item.type !== 'email' || item.verified)).length)
const dashboardPendingTasks = computed(() => buildPendingTasks({
  buyerCarpoolApplications: rideApplications.value ?? [],
  ownerCarpoolApplications: ownerRideApplications.value ?? [],
  buyerApiOrders: apiOrders.value ?? [],
  merchantApiOrders: merchantApiOrders.value ?? [],
}))
const dashboardTasksLoading = computed(() => (
  buyerRideApplicationsQuery.isPending.value
  || (canPublishCarpool.value && ownerRideApplicationsQuery.isPending.value)
  || buyerApiOrdersQuery.isPending.value
  || (canPublishApiService.value && merchantApiOrdersQuery.isPending.value)
))
const dashboardTasksError = computed(() => (
  buyerRideApplicationsQuery.isError.value
  || (canPublishCarpool.value && ownerRideApplicationsQuery.isError.value)
  || buyerApiOrdersQuery.isError.value
  || (canPublishApiService.value && merchantApiOrdersQuery.isError.value)
))
const dashboardTasksUnavailable = computed(() => (
  buyerRideApplicationsQuery.isError.value && rideApplications.value === undefined
  && ownerRideApplicationsQuery.isError.value && ownerRideApplications.value === undefined
  && buyerApiOrdersQuery.isError.value && apiOrders.value === undefined
  && merchantApiOrdersQuery.isError.value && merchantApiOrders.value === undefined
))
const dashboardPublishedItems = computed(() => buildPublishedContent({
  carpools: carpools.value ?? [],
  apiServices: apiServices.value ?? [],
}))
const dashboardPublishedLoading = computed(() => (
  (canPublishCarpool.value && carpoolsQuery.isPending.value)
  || (canPublishApiService.value && apiServicesQuery.isPending.value)
))
const dashboardPublishedError = computed(() => (
  (canPublishCarpool.value && carpoolsQuery.isError.value)
  || (canPublishApiService.value && apiServicesQuery.isError.value)
))
const dashboardPublishedUnavailable = computed(() => (
  carpoolsQuery.isError.value && carpools.value === undefined
  && apiServicesQuery.isError.value && apiServices.value === undefined
))
const showFirstTransactionGuide = computed(() => shouldShowFirstTransactionGuide({
  ownedCarpools: {
    data: canPublishCarpool.value ? carpools.value : [],
    isSuccess: !canPublishCarpool.value || carpoolsQuery.isSuccess.value,
    isFetchedAfterMount: !canPublishCarpool.value || carpoolsQuery.isFetchedAfterMount.value,
    isFetching: canPublishCarpool.value && carpoolsQuery.isFetching.value,
  },
  ownedApiServices: {
    data: canPublishApiService.value ? apiServices.value : [],
    isSuccess: !canPublishApiService.value || apiServicesQuery.isSuccess.value,
    isFetchedAfterMount: !canPublishApiService.value || apiServicesQuery.isFetchedAfterMount.value,
    isFetching: canPublishApiService.value && apiServicesQuery.isFetching.value,
  },
  buyerCarpoolApplications: {
    data: rideApplications.value,
    isSuccess: buyerRideApplicationsQuery.isSuccess.value,
    isFetchedAfterMount: buyerRideApplicationsQuery.isFetchedAfterMount.value,
    isFetching: buyerRideApplicationsQuery.isFetching.value,
  },
  ownerCarpoolApplications: {
    data: canPublishCarpool.value ? ownerRideApplications.value : [],
    isSuccess: !canPublishCarpool.value || ownerRideApplicationsQuery.isSuccess.value,
    isFetchedAfterMount: !canPublishCarpool.value || ownerRideApplicationsQuery.isFetchedAfterMount.value,
    isFetching: canPublishCarpool.value && ownerRideApplicationsQuery.isFetching.value,
  },
  buyerApiOrders: {
    data: apiOrders.value,
    isSuccess: buyerApiOrdersQuery.isSuccess.value,
    isFetchedAfterMount: buyerApiOrdersQuery.isFetchedAfterMount.value,
    isFetching: buyerApiOrdersQuery.isFetching.value,
  },
  merchantApiOrders: {
    data: canPublishApiService.value ? merchantApiOrders.value : [],
    isSuccess: !canPublishApiService.value || merchantApiOrdersQuery.isSuccess.value,
    isFetchedAfterMount: !canPublishApiService.value || merchantApiOrdersQuery.isFetchedAfterMount.value,
    isFetching: canPublishApiService.value && merchantApiOrdersQuery.isFetching.value,
  },
}))
const hasApiServices = computed(() => (apiServices.value?.length ?? 0) > 0)
const savedApiPaymentComplete = computed(() => (
  Boolean(apiPaymentSettings.value && isApiPaymentAccountSettingsComplete(apiPaymentSettings.value))
))
const apiPaymentSettingsValue = computed(() => (
  apiPaymentSettings.value ?? createEmptyApiPaymentAccountSettings()
))
const dashboardCompletenessLoading = computed(() => (
  contactsQuery.isPending.value
  || (canPublishApiService.value && apiServicesQuery.isPending.value)
  || (canPublishApiService.value && hasApiServices.value && apiPaymentSettingsQuery.isPending.value)
))
const dashboardCompletenessError = computed(() => (
  contactsQuery.isError.value
  || (canPublishApiService.value && apiServicesQuery.isError.value)
  || (canPublishApiService.value && hasApiServices.value && apiPaymentSettingsQuery.isError.value)
))
const dashboardCompleteness = computed(() => {
  if (!profile.value || dashboardCompletenessLoading.value || dashboardCompletenessError.value) return null
  return buildAccountCompleteness({
    profile: profile.value,
    enabledContactCount: enabledContactCount.value,
    hasApiServices: hasApiServices.value,
    apiPaymentComplete: savedApiPaymentComplete.value,
  })
})
const dashboardAccountAlert = computed(() => dashboardCompleteness.value
  ? getPrimaryAccountAlert(dashboardCompleteness.value)
  : null)
const dashboardApiOrdersLoading = computed(() => buyerApiOrdersQuery.isPending.value || (canPublishApiService.value && merchantApiOrdersQuery.isPending.value))
const dashboardApiOrdersError = computed(() => buyerApiOrdersQuery.isError.value || (canPublishApiService.value && merchantApiOrdersQuery.isError.value))
const dashboardMetrics = computed<PersonalCenterMetric[]>(() => {
  const metrics: PersonalCenterMetric[] = [{
    id: 'pending',
    label: '待处理',
    value: dashboardPendingTasks.value.length,
    hint: '买家与商户队列',
    loading: dashboardTasksLoading.value,
    available: !dashboardTasksLoading.value && !dashboardTasksError.value,
  },
  {
    id: 'published',
    label: '发布中',
    value: countActivePublishedContent(dashboardPublishedItems.value),
    hint: '车源与服务',
    loading: dashboardPublishedLoading.value,
    available: !dashboardPublishedLoading.value && !dashboardPublishedError.value,
  },
  {
    id: 'api-orders',
    label: 'API 订单',
    value: uniqueRelatedApiOrderCount(apiOrders.value ?? [], merchantApiOrders.value ?? []),
    hint: '买家与商户相关订单',
    loading: dashboardApiOrdersLoading.value,
    available: !dashboardApiOrdersLoading.value && !dashboardApiOrdersError.value,
  },
  {
    id: 'completeness',
    label: '资料待完善',
    value: dashboardCompleteness.value?.missingCount ?? 0,
    hint: dashboardCompleteness.value?.missingCount ? '项账户任务' : '资料已完成',
    loading: dashboardCompletenessLoading.value,
    available: Boolean(dashboardCompleteness.value),
  }]
  return metrics.filter(metric => canPublishAnything.value || metric.id !== 'published')
})
const profileCompleteness = computed(() => dashboardCompleteness.value?.percentage ?? null)
const wechatBound = computed(() => Boolean(wechatContact.value?.enabled && wechatContact.value.displayValue))
const emailBound = computed(() => Boolean(emailContact.value?.enabled && emailContact.value.verified))
const contactSaving = computed(() => createContactMutation.isPending.value || updateContactMutation.isPending.value)
const emailBindingPending = computed(() => contactSaving.value || startContactEmailVerificationMutation.isPending.value || confirmContactEmailVerificationMutation.isPending.value)
const apiPaymentEditorDirty = ref(false)
const wechatContactDirty = computed(() => Boolean(wechatContactSnapshot.value) && wechatFormSignature() !== wechatContactSnapshot.value)
const emailValueDirty = computed(() => (
  contactEmailForm.email.trim().toLowerCase() !== (emailContact.value?.displayValue ?? '').trim().toLowerCase()
))
const emailUsageScopesDirty = computed(() => !sameContactUsageScopes(contactEmailForm.usageScopes, initialContactUsageScopes(emailContact.value, defaultContactUsageScopes.value)))
const emailContactDirty = computed(() => (
  (Boolean(emailContactSnapshot.value) && emailContactFormSignature() !== emailContactSnapshot.value)
  || Boolean(contactEmailForm.code.trim())
  || Boolean(contactEmailVerificationChallengeEmail.value)
))
const hasContactDraftChanges = computed(() => (
  wechatContactDirty.value || emailContactDirty.value || apiPaymentEditorDirty.value
))
const currentSettingsDirty = computed(() => {
  if (activeSection.value === 'profile') return profileSettingsDirty.value
  if (activeSection.value === 'contacts') return hasContactDraftChanges.value
  if (activeSection.value === 'privacy') return privacySettingsDirty.value
  return false
})
const completedContactSettingsCount = computed(() => (
  Number(wechatBound.value) + Number(emailBound.value) + (canPublishApiService.value ? Number(savedApiPaymentComplete.value) : 0)
))
const linuxDoLinkPassword = ref('')
const linuxDoReauthLoading = ref(false)
const linuxDoLinkLoading = ref(false)
const linuxDoRecentlyReauthenticated = ref(false)
const linuxDoLinkSuccessHandled = ref(false)
const savedApiPaymentOptions = computed(() => (
  (apiPaymentSettings.value?.paymentOptions ?? []).filter(option => option.enabled && isApiPaymentOptionComplete(option))
))
const buyerPreviewOpen = ref(false)
useUnsavedChangesGuard(currentSettingsDirty, '当前账户设置尚未保存，确认离开此页面？')

watch(
  [profile, () => route.query.linuxdoLinked],
  ([currentProfile, linked]) => {
    if (linked !== '1' || !currentProfile?.linuxDoBinding.bound || linuxDoLinkSuccessHandled.value) return
    linuxDoLinkSuccessHandled.value = true
    toast.success('linux.do 已绑定，当前会话已更新。')
    const query = { ...route.query }
    delete query.linuxdoLinked
    void router.replace({ path: route.path, query })
  },
  { immediate: true },
)
const accountRecoveryMissingItems = computed(() => profile.value ? accountRecoveryRequirements(profile.value).filter(item => !item.completed) : [])
const accountRecoveryComplete = computed(() => profile.value ? isAccountRecoveryComplete(profile.value) : false)
const canConfigureBackupPassword = computed(() => Boolean(profile.value?.linuxDoBinding.bound))
const accountRecoveryReturnTo = computed(() => sanitizeAccountRecoveryReturnTo(route.query.returnTo))
const quotaPublishRecovery = computed(() => LIMITED_API_QUOTA_OFFERS_ENABLED
  && (accountRecoveryReturnTo.value === '/api-market/quota/new' || accountRecoveryReturnTo.value === '/my/api-services?intent=quota'))
const accountRecoveryDialogTitle = computed(() => {
  if (accountRecoveryComplete.value) return quotaPublishRecovery.value ? '账号设置完成' : '账号设置'
  return quotaPublishRecovery.value ? '发布限量额度包前先完成账号设置' : '完善账号安全'
})
const accountRecoveryDialogDescription = computed(() => {
  if (accountRecoveryComplete.value) {
    if (quotaPublishRecovery.value) return '继续选择 API 服务并发布限量额度包。'
    return canConfigureBackupPassword.value ? '可以在这里更新邮箱或备用密码。' : '可以在这里更新验证邮箱。'
  }
  if (quotaPublishRecovery.value) {
    return canConfigureBackupPassword.value ? '完成邮箱验证和备用密码设置后，会继续进入限量额度包发布流程。' : '完成邮箱验证后，会继续进入限量额度包发布流程。'
  }
  return '补全后即可访问个人中心其他页面和业务页。'
})
const accountRecoveryContinueLabel = computed(() => quotaPublishRecovery.value ? '继续发布限量额度包' : '继续访问原页面')
const accountRecoveryDialogOpen = ref(false)
const dismissedAccountRecoveryDialogKey = ref('')
const accountSetupDialogMode = ref<AccountSetupDialogMode>('required')
const accountSetupActiveStep = ref<AccountSetupStep>('email')
const accountEmailVerificationResendAvailableAt = ref<number | null>(null)
const accountEmailVerificationNow = ref(Date.now())
const accountEmailVerificationDevCode = ref('')
const accountEmailVerificationDevCodeEmail = ref('')
const contactEmailVerificationResendAvailableAt = ref<number | null>(null)
const contactEmailVerificationNow = ref(Date.now())
const contactEmailVerificationDevCode = ref('')
const contactEmailVerificationChallengeEmail = ref('')
let accountEmailVerificationTimer: number | null = null
let contactEmailVerificationTimer: number | null = null
const accountRecoveryDialogKey = computed(() => {
  if (!profile.value || activeSection.value !== 'account' || accountRecoveryComplete.value) return ''
  const missingIds = accountRecoveryMissingItems.value.map(item => item.id).join(',')
  return `${accountRecoveryReturnTo.value ?? ''}:${missingIds}`
})
const accountSetupSteps = computed<AccountSetupStepItem[]>(() => {
  const steps: AccountSetupStepItem[] = [{
    id: 'email',
    step: 1,
    label: '绑定邮箱',
    completed: Boolean(profile.value?.emailVerified),
    active: accountSetupActiveStep.value === 'email',
  }]
  if (canConfigureBackupPassword.value) {
    steps.push({
      id: 'password',
      step: 2,
      label: '设置备用密码',
      completed: Boolean(profile.value?.passwordConfigured),
      active: accountSetupActiveStep.value === 'password',
    })
  }
  steps.push({
    id: 'complete',
    step: canConfigureBackupPassword.value ? 3 : 2,
    label: '完成',
    completed: accountRecoveryComplete.value || accountSetupActiveStep.value === 'complete',
    active: accountSetupActiveStep.value === 'complete',
  })
  return steps
})
const accountSecuritySideDescription = computed(() => {
  if (quotaPublishRecovery.value) return canConfigureBackupPassword.value ? '先完成邮箱和备用密码设置，之后会回到额度包发布流程。' : '先完成邮箱验证，之后会回到额度包发布流程。'
  return canConfigureBackupPassword.value ? '完成邮箱和备用密码设置后，可以使用更多账号功能。' : '完成邮箱验证后，可以使用更多账号功能。'
})
const accountSecurityBenefits = computed(() => [
  {
    title: '账号安全',
    description: canConfigureBackupPassword.value ? '邮箱和备用密码让账号恢复路径更清晰。' : '验证邮箱让账号恢复路径更清晰。',
    icon: ShieldCheck,
  },
  ...(canConfigureBackupPassword.value ? [{
    title: '便捷登录',
    description: '认证入口暂不可用时，也可使用站内账号登录。',
    icon: KeyRound,
  }] : []),
  {
    title: '重要通知',
    description: '接收订单状态、系统通知等重要信息。',
    icon: Bell,
  },
])
const accountEmailVerificationCooldownSeconds = computed(() => {
  if (!accountEmailVerificationResendAvailableAt.value) return 0
  return Math.max(0, Math.ceil((accountEmailVerificationResendAvailableAt.value - accountEmailVerificationNow.value) / 1000))
})
const contactEmailVerificationCooldownSeconds = computed(() => {
  if (!contactEmailVerificationResendAvailableAt.value) return 0
  return Math.max(0, Math.ceil((contactEmailVerificationResendAvailableAt.value - contactEmailVerificationNow.value) / 1000))
})
const accountEmailVerificationSendDisabled = computed(() => startEmailVerificationMutation.isPending.value || !accountEmailForm.email.trim() || accountEmailVerificationCooldownSeconds.value > 0)
const accountEmailVerificationButtonLabel = computed(() => {
  if (startEmailVerificationMutation.isPending.value) return '发送中'
  if (accountEmailVerificationCooldownSeconds.value > 0) return `${accountEmailVerificationCooldownSeconds.value} 秒后重发`
  return '发送验证码'
})
const contactEmailVerificationButtonLabel = computed(() => {
  if (startContactEmailVerificationMutation.isPending.value) return '发送中'
  if (contactEmailVerificationCooldownSeconds.value > 0) return `${contactEmailVerificationCooldownSeconds.value} 秒后重发`
  return '发送验证码'
})
const visibleAccountEmailVerificationDevCode = computed(() => {
  if (!accountEmailVerificationDevCode.value) return ''
  const currentEmail = accountEmailForm.email.trim().toLowerCase()
  if (!currentEmail || currentEmail !== accountEmailVerificationDevCodeEmail.value) return ''
  return accountEmailVerificationDevCode.value
})
const visibleContactEmailVerificationDevCode = computed(() => {
  if (!contactEmailVerificationDevCode.value) return ''
  const currentEmail = contactEmailForm.email.trim().toLowerCase()
  if (!currentEmail || currentEmail !== contactEmailVerificationChallengeEmail.value) return ''
  return contactEmailVerificationDevCode.value
})
const passwordChecks = computed(() => getPasswordChecks(passwordForm.newPassword))
const passwordPassedCount = computed(() => passwordChecks.value.filter(item => item.completed).length)
const passwordRulesComplete = computed(() => passwordPassedCount.value === passwordChecks.value.length)
const passwordStrength = computed(() => getBackupPasswordStrength(passwordForm.newPassword))
const passwordStrengthSegmentClass = computed(() => {
  if (passwordStrength.value.tone === 'success') return 'bg-success'
  if (passwordStrength.value.tone === 'warning') return 'bg-warning'
  if (passwordStrength.value.tone === 'danger') return 'bg-destructive'
  return 'bg-border'
})
const passwordStrengthTextClass = computed(() => {
  if (passwordStrength.value.tone === 'success') return 'text-success'
  if (passwordStrength.value.tone === 'warning') return 'text-warning'
  if (passwordStrength.value.tone === 'danger') return 'text-destructive'
  return 'text-muted-foreground'
})
const confirmPasswordMismatch = computed(() => (
  confirmPasswordTouched.value
  && Boolean(passwordForm.confirmPassword)
  && passwordForm.confirmPassword !== passwordForm.newPassword
))
const canSubmitAccountPassword = computed(() => (
  canConfigureBackupPassword.value
  && passwordRulesComplete.value
  && Boolean(passwordForm.confirmPassword)
  && passwordForm.confirmPassword === passwordForm.newPassword
))

watch(accountRecoveryDialogKey, key => {
  if (!key || dismissedAccountRecoveryDialogKey.value === key) return
  openAccountSetupDialog('required')
}, { immediate: true })

watch(accountRecoveryComplete, complete => {
  if (complete && accountRecoveryDialogOpen.value && accountSetupDialogMode.value === 'required') {
    accountSetupActiveStep.value = 'complete'
  }
})

function syncProfileDraft(currentProfile: UserProfile) {
  profileForm.displayName = currentProfile.displayName
  profileForm.username = currentProfile.username
  profileForm.bio = currentProfile.bio ?? ''
  profileForm.regionCode = currentProfile.regionCode ?? ''
  profileForm.timezone = currentProfile.timezone ?? 'Asia/Shanghai'
  profileForm.avatarMode = currentProfile.avatarMode
  profileForm.avatarUrl = currentProfile.customAvatarUrl ?? ''
  profileSnapshot.value = profileFormSignature()
}

function syncPrivacyDraft(currentProfile: UserProfile) {
  Object.assign(privacyForm, currentProfile.privacy)
  privacySnapshot.value = privacyFormSignature()
}

function syncWechatDraft(contact: UserContactMethod | null) {
  wechatForm.displayValue = contact?.displayValue ?? ''
  wechatContactSnapshot.value = wechatFormSignature()
}

function syncEmailContactDraft(contact: UserContactMethod | null) {
  contactEmailForm.email = contact?.displayValue ?? ''
  contactEmailForm.code = ''
  contactEmailForm.usageScopes = initialContactUsageScopes(contact, defaultContactUsageScopes.value)
  emailContactSnapshot.value = emailContactFormSignature()
}

watch(profile, currentProfile => {
  if (!currentProfile) return
  if (!profileSettingsDirty.value) syncProfileDraft(currentProfile)
  if (!privacySettingsDirty.value) syncPrivacyDraft(currentProfile)
  if (!accountEmailForm.email) accountEmailForm.email = currentProfile.email || ''
}, { immediate: true })

watch([wechatContact, defaultContactUsageScopes], ([contact]) => {
  if (wechatContactDirty.value) return
  syncWechatDraft(contact)
}, { immediate: true })

watch([emailContact, defaultContactUsageScopes], ([contact]) => {
  if (emailContactDirty.value) return
  syncEmailContactDraft(contact)
}, { immediate: true })

const avatarText = computed(() => (profile.value?.displayName || profile.value?.username || '我').slice(0, 1).toUpperCase())
const profileErrorMessage = computed(() => {
  const error = profileQuery.error.value
  return error instanceof Error ? error.message : '无法读取个人资料，请登录后重试。'
})

function handleBlockedSettingsNavigation() {
  openAccountSetupDialog('required')
}

function defaultAccountSetupStep(mode: AccountSetupDialogMode): AccountSetupStep {
  if (mode === 'email') return 'email'
  if (mode === 'password') return canConfigureBackupPassword.value ? 'password' : (profile.value?.emailVerified ? 'complete' : 'email')
  if (!profile.value?.emailVerified) return 'email'
  if (canConfigureBackupPassword.value && !profile.value.passwordConfigured) return 'password'
  return 'complete'
}

function openAccountSetupDialog(mode: AccountSetupDialogMode = 'required') {
  accountSetupDialogMode.value = mode
  accountSetupActiveStep.value = defaultAccountSetupStep(mode)
  confirmPasswordTouched.value = false
  accountRecoveryDialogOpen.value = true
}

function setAccountRecoveryDialogOpen(open: boolean) {
  accountRecoveryDialogOpen.value = open
  if (open) return
  const key = accountRecoveryDialogKey.value
  if (key) dismissedAccountRecoveryDialogKey.value = key
  accountSetupDialogMode.value = 'required'
  accountSetupActiveStep.value = defaultAccountSetupStep('required')
}

function scopeLabels(scopes: ContactUsageScope[]) {
  return scopes.map(scope => CONTACT_USAGE_SCOPE_OPTIONS.find(item => item.value === scope)?.label ?? scope).join('、')
}

function stopAccountEmailVerificationTimer() {
  if (accountEmailVerificationTimer === null) return
  window.clearInterval(accountEmailVerificationTimer)
  accountEmailVerificationTimer = null
}

function startAccountEmailVerificationTimer() {
  accountEmailVerificationResendAvailableAt.value = Date.now() + 60 * 1000
  accountEmailVerificationNow.value = Date.now()
  stopAccountEmailVerificationTimer()
  accountEmailVerificationTimer = window.setInterval(() => {
    accountEmailVerificationNow.value = Date.now()
    if (accountEmailVerificationCooldownSeconds.value === 0) stopAccountEmailVerificationTimer()
  }, 1000)
}

function stopContactEmailVerificationTimer() {
  if (contactEmailVerificationTimer === null) return
  window.clearInterval(contactEmailVerificationTimer)
  contactEmailVerificationTimer = null
}

function startContactEmailVerificationTimer() {
  contactEmailVerificationResendAvailableAt.value = Date.now() + 60 * 1000
  contactEmailVerificationNow.value = Date.now()
  stopContactEmailVerificationTimer()
  contactEmailVerificationTimer = window.setInterval(() => {
    contactEmailVerificationNow.value = Date.now()
    if (contactEmailVerificationCooldownSeconds.value === 0) stopContactEmailVerificationTimer()
  }, 1000)
}

onUnmounted(() => {
  stopAccountEmailVerificationTimer()
  stopContactEmailVerificationTimer()
})

function saveProfile() {
  if (!profile.value) return
  updateProfileMutation.mutate({
    displayName: profileForm.displayName,
    username: profileForm.username,
    bio: profileForm.bio || null,
    regionCode: profileForm.regionCode || null,
    timezone: profileForm.timezone || null,
    avatarMode: profileForm.avatarMode,
    avatarUrl: profileForm.avatarMode === 'custom_url' ? profileForm.avatarUrl.trim() : null,
    privacy: profile.value.privacy,
  }, {
    onSuccess: updatedProfile => {
      syncProfileDraft(updatedProfile)
      if (!privacySettingsDirty.value) syncPrivacyDraft(updatedProfile)
      toast.success('个人资料已保存。')
    },
    onError: error => toast.error(error instanceof Error ? error.message : '保存失败'),
  })
}

function savePassword() {
  if (!canConfigureBackupPassword.value) {
    toast.warning('当前账号未绑定 linux.do，不能设置备用密码。')
    return
  }
  if (profile.value?.passwordConfigured && !passwordForm.currentPassword.trim()) {
    toast.warning('请输入当前密码。')
    return
  }
  const passwordValidationMessage = getBackupPasswordValidationMessage(passwordForm.newPassword)
  if (passwordValidationMessage) {
    toast.warning(passwordValidationMessage)
    return
  }
  if (passwordForm.newPassword !== passwordForm.confirmPassword) {
    confirmPasswordTouched.value = true
    toast.warning('两次输入的密码不一致，请重新输入')
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
      confirmPasswordTouched.value = false
      accountSetupActiveStep.value = profile.value?.emailVerified ? 'complete' : 'email'
      toast.success('密码已更新。')
    },
    onError: error => toast.error(error instanceof Error ? error.message : '密码更新失败。'),
  })
}

function continueAfterAccountRecovery() {
  const returnTo = accountRecoveryReturnTo.value
  if (!returnTo) return
  router.push(returnTo)
}

function startEmailVerification() {
  accountEmailVerificationDevCode.value = ''
  accountEmailVerificationDevCodeEmail.value = ''
  startEmailVerificationMutation.mutate(accountEmailForm.email, {
    onSuccess: challenge => {
      const devCode = challenge.devCode?.trim() ?? ''
      accountEmailForm.email = challenge.email
      accountEmailForm.code = devCode
      accountEmailVerificationDevCode.value = devCode
      accountEmailVerificationDevCodeEmail.value = challenge.email.trim().toLowerCase()
      startAccountEmailVerificationTimer()
      toast.success(devCode ? '开发验证码已填入。' : '验证码已发送，请查看邮箱。')
    },
    onError: error => toast.error(error instanceof Error ? error.message : '验证码发送失败。'),
  })
}

function confirmEmailVerification() {
  confirmEmailVerificationMutation.mutate({
    email: accountEmailForm.email,
    code: accountEmailForm.code,
  }, {
    onSuccess: updatedProfile => {
      accountEmailForm.code = ''
      accountEmailVerificationDevCode.value = ''
      accountEmailVerificationDevCodeEmail.value = ''
      stopAccountEmailVerificationTimer()
      accountEmailVerificationResendAvailableAt.value = null
      accountSetupActiveStep.value = updatedProfile.linuxDoBinding.bound && !updatedProfile.passwordConfigured ? 'password' : 'complete'
      toast.success('邮箱已绑定。')
    },
    onError: error => toast.error(error instanceof Error ? error.message : '邮箱绑定失败。'),
  })
}

function savePrivacy() {
  if (!profile.value) return
  updateProfileMutation.mutate({
    displayName: profile.value.displayName,
    username: profile.value.username,
    bio: profile.value.bio,
    regionCode: profile.value.regionCode,
    timezone: profile.value.timezone,
    avatarMode: profile.value.avatarMode,
    avatarUrl: profile.value.customAvatarUrl,
    privacy: privacyForm,
  }, {
    onSuccess: updatedProfile => {
      syncPrivacyDraft(updatedProfile)
      if (!profileSettingsDirty.value) syncProfileDraft(updatedProfile)
      toast.success('隐私设置已保存。')
    },
    onError: error => toast.error(error instanceof Error ? error.message : '保存失败'),
  })
}

function saveWechatContact() {
  const displayValue = wechatForm.displayValue.trim()
  if (!displayValue) {
    toast.warning('请先填写微信号。')
    return
  }
  const current = wechatContact.value
  const payload = buildContactMethodPayload({
    type: 'wechat',
    label: '微信',
    displayValue,
    usageScopes: ALL_CONTACT_USAGE_SCOPES,
    current,
  })
  const mutationOptions = {
    onSuccess: (savedContact: UserContactMethod) => {
      syncWechatDraft(savedContact)
      toast.success(current ? '微信联系方式已更新。' : '微信联系方式已配置。')
    },
    onError: (error: Error) => toast.error(error.message),
  }
  if (current) {
    updateContactMutation.mutate({ contactId: current.id, payload }, mutationOptions)
    return
  }
  createContactMutation.mutate(payload, mutationOptions)
}

function persistEmailContactForVerification(onSaved: (contact: UserContactMethod) => void) {
  const displayValue = contactEmailForm.email.trim().toLowerCase()
  if (!displayValue) {
    toast.warning('请先填写邮箱。')
    return
  }
  if (!contactEmailForm.usageScopes.length) {
    toast.warning('请至少选择一个适用场景。')
    return
  }
  const current = emailContact.value
  const payload = buildContactMethodPayload({
    type: 'email',
    label: '邮箱',
    displayValue,
    usageScopes: contactEmailForm.usageScopes,
    current,
  })
  const mutationOptions = {
    onSuccess: (savedContact: UserContactMethod) => {
      syncEmailContactDraft(savedContact)
      onSaved(savedContact)
    },
    onError: (error: Error) => toast.error(error.message),
  }
  if (current && !emailValueDirty.value && !emailUsageScopesDirty.value) {
    onSaved(current)
    return
  }
  if (current) {
    updateContactMutation.mutate({ contactId: current.id, payload }, mutationOptions)
    return
  }
  createContactMutation.mutate(payload, mutationOptions)
}

function startContactEmailVerification() {
  contactEmailVerificationDevCode.value = ''
  contactEmailVerificationChallengeEmail.value = ''
  persistEmailContactForVerification((contact) => {
    startContactEmailVerificationMutation.mutate(contact.id, {
      onSuccess: challenge => {
        const devCode = challenge.devCode?.trim() ?? ''
        contactEmailForm.email = challenge.email
        contactEmailForm.code = devCode
        contactEmailVerificationDevCode.value = devCode
        contactEmailVerificationChallengeEmail.value = challenge.email.trim().toLowerCase()
        startContactEmailVerificationTimer()
        toast.success(devCode ? '开发验证码已填入。' : '验证码已发送，请查看邮箱。')
      },
      onError: error => toast.error(error instanceof Error ? error.message : '验证码发送失败。'),
    })
  })
}

function saveEmailUsageScopes() {
  const current = emailContact.value
  if (!current || emailValueDirty.value) return
  if (!contactEmailForm.usageScopes.length) {
    toast.warning('请至少选择一个适用场景。')
    return
  }
  updateContactMutation.mutate({
    contactId: current.id,
    payload: buildContactMethodPayload({
      type: 'email',
      label: current.label || '邮箱',
      displayValue: contactEmailForm.email,
      usageScopes: contactEmailForm.usageScopes,
      current,
    }),
  }, {
    onSuccess: savedContact => {
      syncEmailContactDraft(savedContact)
      toast.success('邮箱适用场景已更新。')
    },
    onError: (error: Error) => toast.error(error.message),
  })
}

function confirmContactEmailVerification() {
  const current = emailContact.value
  if (!current) {
    toast.warning('请先发送验证码。')
    return
  }
  if (!contactEmailVerificationChallengeEmail.value || contactEmailForm.email.trim().toLowerCase() !== contactEmailVerificationChallengeEmail.value) {
    toast.warning('邮箱已变化，请重新发送验证码。')
    return
  }
  confirmContactEmailVerificationMutation.mutate({
    contactId: current.id,
    code: contactEmailForm.code,
  }, {
    onSuccess: savedContact => {
      contactEmailVerificationDevCode.value = ''
      contactEmailVerificationChallengeEmail.value = ''
      stopContactEmailVerificationTimer()
      contactEmailVerificationResendAvailableAt.value = null
      syncEmailContactDraft(savedContact)
      toast.success('交易联系邮箱已验证。')
    },
    onError: error => toast.error(error instanceof Error ? error.message : '交易联系邮箱验证失败。'),
  })
}

function useVerifiedAccountEmailForContact() {
  const accountEmail = profile.value?.email?.trim().toLowerCase()
  if (!profile.value?.emailVerified || !accountEmail) return
  contactEmailForm.email = accountEmail
  contactEmailForm.code = ''
  contactEmailVerificationDevCode.value = ''
  contactEmailVerificationChallengeEmail.value = ''
  stopContactEmailVerificationTimer()
  contactEmailVerificationResendAvailableAt.value = null
}

function removeContact(contact: UserContactMethod) {
  deleteContactMutation.mutate(contact.id, {
    onSuccess: () => {
      if (contact.type === 'wechat') wechatForm.displayValue = ''
      if (contact.type === 'email') {
        contactEmailForm.email = ''
        contactEmailForm.code = ''
        contactEmailVerificationChallengeEmail.value = ''
      }
      if (contact.type === 'wechat') syncWechatDraft(null)
      if (contact.type === 'email') syncEmailContactDraft(null)
      toast.success('联系方式已解除绑定。')
    },
    onError: error => toast.error(error instanceof Error ? error.message : '删除失败'),
  })
}

function setDefaultContact(contact: UserContactMethod) {
  setDefaultContactMutation.mutate(contact.id, {
    onSuccess: () => toast.success('默认联系方式已更新。'),
    onError: error => toast.error(error instanceof Error ? error.message : '设置失败'),
  })
}

async function retryDashboardTasks() {
  const requests: Array<Promise<unknown>> = [
    buyerRideApplicationsQuery.refetch(),
    buyerApiOrdersQuery.refetch(),
  ]
  if (canPublishCarpool.value) requests.push(ownerRideApplicationsQuery.refetch())
  if (canPublishApiService.value) requests.push(merchantApiOrdersQuery.refetch())
  await Promise.allSettled(requests)
}

async function retryDashboardPublished() {
  const requests: Array<Promise<unknown>> = []
  if (canPublishCarpool.value) requests.push(carpoolsQuery.refetch())
  if (canPublishApiService.value) requests.push(apiServicesQuery.refetch())
  await Promise.allSettled(requests)
}

async function retryDashboardCompleteness() {
  const requests: Array<Promise<unknown>> = [contactsQuery.refetch()]
  if (canPublishApiService.value) requests.push(apiServicesQuery.refetch(), apiPaymentSettingsQuery.refetch())
  await Promise.allSettled(requests)
}

async function confirmRecentPassword() {
  if (!linuxDoLinkPassword.value) {
    toast.warning('请输入当前密码。')
    return
  }
  linuxDoReauthLoading.value = true
  try {
    await reauthenticatePassword(linuxDoLinkPassword.value)
    linuxDoLinkPassword.value = ''
    linuxDoRecentlyReauthenticated.value = true
    toast.success('身份验证完成，可继续绑定 linux.do。')
  } catch (error) {
    linuxDoRecentlyReauthenticated.value = false
    toast.error(backendErrorMessage(error, '当前密码验证失败。'))
  } finally {
    linuxDoReauthLoading.value = false
  }
}

async function linkLinuxDoIdentity() {
  if (!linuxDoRecentlyReauthenticated.value) {
    toast.warning('请先使用当前密码完成身份验证。')
    return
  }
  linuxDoLinkLoading.value = true
  try {
    const { authorizationUrl } = await startLinuxDoLink('/my/account?linuxdoLinked=1')
    window.location.assign(authorizationUrl)
  } catch (error) {
    toast.error(backendErrorMessage(error, '启动 linux.do 绑定失败。'))
    linuxDoLinkLoading.value = false
  }
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
        <h1 class="text-lg font-semibold tracking-tight">请先登录后查看个人中心</h1>
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
  <div v-else class="my-center-reference space-y-5">
    <Dialog :open="accountRecoveryDialogOpen" @update:open="setAccountRecoveryDialogOpen">
      <DialogContent class="account-security-dialog w-[calc(100vw-1rem)] gap-0 overflow-hidden p-0 sm:max-w-[860px]">
        <div class="account-security-layout grid max-h-[calc(100dvh-1rem)] min-h-[560px] overflow-y-auto md:grid-cols-[300px_minmax(0,1fr)] md:overflow-hidden">
          <aside class="account-security-side flex flex-col p-6 sm:p-7">
            <div class="account-security-side-icon grid h-16 w-16 place-items-center rounded-2xl">
              <ShieldCheck class="h-8 w-8" />
            </div>
            <div class="account-security-side-copy mt-6">
              <h2 class="text-2xl font-semibold tracking-tight">{{ quotaPublishRecovery ? '发布限量额度包' : '完善账号安全' }}</h2>
              <p class="mt-3 text-sm leading-6 text-muted-foreground">
                {{ accountSecuritySideDescription }}
              </p>
            </div>

            <div class="account-security-benefits mt-8 space-y-4">
              <div v-for="item in accountSecurityBenefits" :key="item.title" class="flex gap-3">
                <div class="account-security-benefit-icon grid h-10 w-10 shrink-0 place-items-center rounded-xl">
                  <component :is="item.icon" class="h-5 w-5" />
                </div>
                <div class="min-w-0">
                  <h3 class="text-sm font-semibold">{{ item.title }}</h3>
                  <p class="mt-1 text-xs leading-5 text-muted-foreground">{{ item.description }}</p>
                </div>
              </div>
            </div>

            <p class="account-security-side-foot mt-auto flex gap-2 pt-8 text-xs leading-5 text-muted-foreground">
              <ShieldCheck class="mt-0.5 h-4 w-4 shrink-0 text-success" />
              这些设置只用于站内账号通知、找回和登录。
            </p>
          </aside>

          <div class="flex min-w-0 flex-col">
            <DialogHeader class="account-security-header px-5 py-5 pr-12 sm:px-6">
              <DialogTitle>{{ accountRecoveryDialogTitle }}</DialogTitle>
              <DialogDescription>
                {{ accountRecoveryDialogDescription }}
              </DialogDescription>
            </DialogHeader>

            <div class="account-security-body flex-1 px-5 py-5 sm:px-6">
              <ol class="grid grid-cols-3" aria-label="账号安全步骤">
                <li
                  v-for="(step, index) in accountSetupSteps"
                  :key="step.id"
                  class="relative min-w-0 text-center"
                  :aria-current="step.active ? 'step' : undefined"
                >
                  <span
                    v-if="index > 0"
                    class="absolute left-0 right-1/2 top-5 h-px bg-border"
                    :class="step.completed ? 'bg-primary/45' : ''"
                    aria-hidden="true"
                  ></span>
                  <span
                    v-if="index < accountSetupSteps.length - 1"
                    class="absolute left-1/2 right-0 top-5 h-px bg-border"
                    :class="accountSetupSteps[index + 1]?.completed ? 'bg-primary/45' : ''"
                    aria-hidden="true"
                  ></span>
                  <div
                    class="relative z-10 mx-auto grid h-10 w-10 place-items-center rounded-full border bg-card text-sm font-semibold shadow-sm"
                    :class="[
                      step.active ? 'border-primary bg-primary text-primary-foreground' : '',
                      step.completed && !step.active ? 'border-primary/20 bg-primary/10 text-primary' : '',
                      !step.active && !step.completed ? 'border-border text-muted-foreground' : '',
                    ]"
                  >
                    <CheckCircle2 v-if="step.completed && !step.active" class="h-4 w-4" />
                    <span v-else>{{ step.step }}</span>
                  </div>
                  <div
                    class="mt-2 truncate text-xs font-medium"
                    :class="step.active ? 'text-primary' : 'text-muted-foreground'"
                  >
                    {{ step.label }}
                  </div>
                </li>
              </ol>

              <section v-if="accountSetupActiveStep === 'email'" class="mt-7 space-y-5">
                <div>
                  <h3 class="text-base font-semibold">绑定验证邮箱</h3>
                  <p class="mt-2 text-sm leading-6 text-muted-foreground">
                    用于接收验证码、重要通知和后续找回密码。
                  </p>
                </div>

                <div class="space-y-4">
                  <label class="block space-y-2">
                    <span class="text-sm font-medium">邮箱地址</span>
                    <span class="relative block">
                      <Mail class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                      <Input v-model="accountEmailForm.email" class="h-11 pl-10" type="email" autocomplete="email" placeholder="name@example.com" />
                    </span>
                  </label>

                  <label class="block space-y-2">
                    <span class="text-sm font-medium">验证码</span>
                    <span class="grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto]">
                      <span class="relative block">
                        <MailCheck class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                        <Input v-model="accountEmailForm.code" class="h-11 pl-10" inputmode="numeric" maxlength="6" placeholder="6 位验证码" />
                      </span>
                      <Button
                        variant="outline"
                        class="h-11 shrink-0 sm:min-w-[140px]"
                        :disabled="accountEmailVerificationSendDisabled"
                        @click="startEmailVerification"
                      >
                        <MailCheck class="h-4 w-4" />{{ accountEmailVerificationButtonLabel }}
                      </Button>
                    </span>
                    <span v-if="visibleAccountEmailVerificationDevCode" class="block rounded-md border border-dashed border-amber-300 bg-amber-50 px-3 py-2 text-xs leading-5 text-amber-900">
                      开发验证码：<span class="font-semibold tabular-nums">{{ visibleAccountEmailVerificationDevCode }}</span>
                    </span>
                  </label>
                </div>
              </section>

              <section v-else-if="accountSetupActiveStep === 'password' && canConfigureBackupPassword" class="mt-7 space-y-5">
                <div>
                  <h3 class="text-base font-semibold">{{ profile.passwordConfigured ? '修改备用密码' : '设置备用密码' }}</h3>
                  <p class="mt-2 text-sm leading-6 text-muted-foreground">
                    设置后可在 linux.do 暂不可用时，使用站内用户名和备用密码登录。
                  </p>
                </div>

                <div class="grid gap-4">
                  <div v-if="profile.passwordConfigured" class="space-y-2">
                    <label for="account-current-password" class="text-sm font-medium">当前密码</label>
                    <PasswordVisibilityInput
                      id="account-current-password"
                      v-model="passwordForm.currentPassword"
                      label="当前密码"
                      autocomplete="current-password"
                    />
                  </div>
                  <div class="space-y-2">
                    <label for="account-new-password" class="text-sm font-medium">新密码</label>
                    <PasswordVisibilityInput
                      id="account-new-password"
                      v-model="passwordForm.newPassword"
                      label="新密码"
                      autocomplete="new-password"
                    />
                  </div>
                  <div class="space-y-2">
                    <label for="account-confirm-password" class="text-sm font-medium">确认新密码</label>
                    <PasswordVisibilityInput
                      id="account-confirm-password"
                      v-model="passwordForm.confirmPassword"
                      label="确认新密码"
                      autocomplete="new-password"
                      :invalid="confirmPasswordMismatch"
                      :described-by="confirmPasswordMismatch ? 'account-confirm-password-error' : undefined"
                      @blur="confirmPasswordTouched = true"
                    />
                    <span
                      v-if="confirmPasswordMismatch"
                      id="account-confirm-password-error"
                      role="alert"
                      class="flex items-center gap-1.5 text-xs font-medium text-destructive"
                    >
                      <TriangleAlert class="h-3.5 w-3.5 shrink-0" />
                      两次输入的密码不一致，请重新输入
                    </span>
                  </div>
                </div>

                <div class="rounded-2xl border border-[#E2E8F0] bg-[#F8FAFC] p-4">
                  <div class="flex items-start justify-between gap-3">
                    <div class="min-w-0">
                      <h4 class="text-sm font-semibold">密码要求</h4>
                      <p class="mt-1 text-xs leading-5 text-muted-foreground">
                        8–32 位，且包含字母、数字、特殊字符
                      </p>
                    </div>
                    <span
                      class="shrink-0 rounded-full border px-2.5 py-1 text-xs font-semibold"
                      :class="passwordRulesComplete ? 'border-success/20 bg-success/10 text-success' : 'border-[#E2E8F0] bg-white text-muted-foreground'"
                    >
                      已满足 {{ passwordPassedCount }}/4
                    </span>
                  </div>
                  <div class="mt-3 flex items-center gap-2">
                    <span class="shrink-0 text-xs text-muted-foreground">强度</span>
                    <div
                      class="grid w-40 max-w-[45%] grid-cols-4 gap-1.5"
                      role="progressbar"
                      :aria-valuenow="passwordPassedCount"
                      aria-valuemin="0"
                      aria-valuemax="4"
                      :aria-label="`密码强度：${passwordStrength.label}，已满足 ${passwordPassedCount}/4`"
                    >
                      <span
                        v-for="index in 4"
                        :key="index"
                        class="h-1.5 rounded-full transition-colors"
                        :class="index <= passwordPassedCount ? passwordStrengthSegmentClass : 'bg-border'"
                      ></span>
                    </div>
                    <span class="text-xs font-semibold" :class="passwordStrengthTextClass">{{ passwordStrength.label }}</span>
                  </div>
                  <ul class="mt-3 grid gap-x-5 gap-y-2 text-xs leading-5 text-muted-foreground sm:grid-cols-2">
                    <li
                      v-for="item in passwordChecks"
                      :key="item.id"
                      class="flex items-center gap-2"
                      :class="item.completed ? 'font-medium text-success' : ''"
                    >
                      <CheckCircle2 v-if="item.completed" class="h-3.5 w-3.5 shrink-0" />
                      <span v-else class="h-3.5 w-3.5 shrink-0 rounded-full border border-[#CBD5E1] bg-white"></span>
                      <span>{{ item.label }}</span>
                    </li>
                  </ul>
                </div>
              </section>

              <section v-else class="mt-7 space-y-5 text-center">
                <div class="mx-auto grid h-16 w-16 place-items-center rounded-2xl bg-success/10 text-success">
                  <CheckCircle2 class="h-8 w-8" />
                </div>
                <div>
                  <h3 class="text-lg font-semibold">账号安全设置完成</h3>
                  <p class="mt-2 text-sm leading-6 text-muted-foreground">
                    {{ quotaPublishRecovery ? '现在可以继续选择 API 服务并发布限量额度包。' : '现在可以继续访问原页面，或留在账号页检查其他设置。' }}
                  </p>
                </div>
                <dl class="grid gap-3 rounded-lg border border-border bg-card/70 p-4 text-left text-sm" :class="canConfigureBackupPassword ? 'sm:grid-cols-2' : ''">
                  <div>
                    <dt class="text-muted-foreground">绑定邮箱</dt>
                    <dd class="mt-1 font-medium">{{ profile.emailVerified ? profile.email : '待同步' }}</dd>
                  </div>
                  <div v-if="canConfigureBackupPassword">
                    <dt class="text-muted-foreground">备用密码</dt>
                    <dd class="mt-1 font-medium">{{ profile.passwordConfigured ? '已设置' : '待同步' }}</dd>
                  </div>
                </dl>
              </section>

            </div>

            <DialogFooter class="account-security-footer gap-2 border-t border-border px-5 py-4 sm:justify-end sm:px-6">
              <Button
                v-if="accountSetupActiveStep === 'email'"
                :disabled="confirmEmailVerificationMutation.isPending.value || !accountEmailForm.code.trim()"
                @click="confirmEmailVerification"
              >
                下一步
              </Button>
              <Button
                v-else-if="accountSetupActiveStep === 'password'"
                :disabled="setPasswordMutation.isPending.value || !canSubmitAccountPassword"
                @click="savePassword"
              >
                <LockKeyhole class="h-4 w-4" />{{ profile.passwordConfigured ? '保存密码' : '完成设置' }}
              </Button>
              <Button v-else-if="accountRecoveryReturnTo" :disabled="!accountRecoveryComplete" @click="continueAfterAccountRecovery">
                {{ accountRecoveryContinueLabel }}
              </Button>
              <Button v-else @click="setAccountRecoveryDialogOpen(false)">进入个人中心</Button>
            </DialogFooter>
          </div>
        </div>
      </DialogContent>
    </Dialog>

    <BuyerPreviewDrawer
      :open="buyerPreviewOpen"
      :display-name="profile.displayName"
      :avatar-text="avatarText"
      :avatar-url="profile.avatarUrl"
      :saved-wechat="wechatContact?.enabled ? wechatContact.displayValue : ''"
      :saved-email="emailContact?.enabled && emailContact.verified ? emailContact.displayValue : ''"
      :saved-payment-options="savedApiPaymentOptions"
      :show-payment="canPublishApiService"
      :has-unsaved-changes="hasContactDraftChanges"
      @update:open="buyerPreviewOpen = $event"
    />

    <header v-if="activeSection === 'overview'" class="my-center-overview-heading"><h1>个人中心</h1><p>管理交易、发布内容与账户资料</p></header>

    <PersonalCenterDashboard
      v-if="activeSection === 'overview'"
      :profile="profile"
      :metrics="dashboardMetrics"
      :pending-tasks="dashboardPendingTasks"
      :published-items="dashboardPublishedItems"
      :completeness="dashboardCompleteness"
      :account-alert="dashboardAccountAlert"
      :tasks-loading="dashboardTasksLoading"
      :tasks-error="dashboardTasksError"
      :tasks-unavailable="dashboardTasksUnavailable"
      :published-loading="dashboardPublishedLoading"
      :published-error="dashboardPublishedError"
      :published-unavailable="dashboardPublishedUnavailable"
      :show-first-transaction-guide="showFirstTransactionGuide"
      :completeness-loading="dashboardCompletenessLoading"
      :completeness-error="dashboardCompletenessError"
      :enabled-contact-count="enabledContactCount"
      :contacts-loading="contactsQuery.isPending.value"
      :contacts-error="contactsQuery.isError.value"
      :buyer-ride-count="buyerRideApplicationsQuery.isSuccess.value ? (rideApplications?.length ?? 0) : null"
      :related-api-order-count="dashboardApiOrdersError || dashboardApiOrdersLoading ? null : uniqueRelatedApiOrderCount(apiOrders ?? [], merchantApiOrders ?? [])"
      :can-publish-carpool="canPublishCarpool"
      :can-publish-api-service="canPublishApiService"
      @retry-tasks="retryDashboardTasks"
      @retry-published="retryDashboardPublished"
      @retry-completeness="retryDashboardCompleteness"
    />

    <AccountSettingsShell
      v-else
      :contact-label="canPublishApiService ? '联系与收款' : '联系方式'"
      :locked="!accountRecoveryComplete"
      @blocked-navigation="handleBlockedSettingsNavigation"
    >
      <template v-if="activeSection === 'contacts'" #description>
        <p class="my-center-page-subtitle">{{ canPublishApiService ? '完善联系方式与 API 收款信息，只向有效交易参与方展示必要资料。' : '完善联系方式，只向有效交易参与方展示必要资料。' }}</p>
      </template>
      <template #actions>
        <Button v-if="activeSection === 'contacts'" variant="outline" @click="buyerPreviewOpen = true">
          <Eye class="h-4 w-4" />预览买家看到的信息
        </Button>
        <Button v-else variant="outline" @click="router.push(`/u/${profile.username}`)"><Eye class="h-4 w-4" />查看公开主页</Button>
      </template>

    <section v-if="activeSection === 'profile'" class="my-center-settings-layout">
      <main class="min-w-0">
      <Card class="p-5">
        <h2 class="font-semibold">个人资料设置</h2>
        <div class="my-center-profile-avatar-row">
          <div class="grid h-20 w-20 shrink-0 place-items-center overflow-hidden rounded-full bg-primary text-xl font-semibold text-primary-foreground"><img v-if="profile.avatarUrl" :src="profile.avatarUrl" alt="当前头像" class="h-full w-full object-cover" /><span v-else>{{ avatarText }}</span></div>
          <div><strong class="block">{{ profile.displayName }}</strong><p class="mt-1 text-sm text-muted-foreground">@{{ profile.username }} · 头像可跟随 linux.do 或使用 HTTPS 图片</p></div>
        </div>
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
        <Button class="mt-5" :disabled="updateProfileMutation.isPending.value || !profileSettingsDirty" @click="saveProfile"><Save class="h-4 w-4" />保存个人资料</Button>
      </Card>
      </main>

      <aside class="my-center-profile-aside space-y-4"><Card class="p-5">
        <h2 class="font-semibold">头像</h2>
        <div class="mt-4 space-y-3">
          <RadioGroup v-model="profileForm.avatarMode" aria-label="头像来源" class="gap-3">
            <div class="flex items-center gap-2 text-sm">
              <RadioGroupItem id="avatar-mode-linuxdo" value="linuxdo" />
              <Label for="avatar-mode-linuxdo" class="cursor-pointer font-normal">跟随 linux.do 头像</Label>
            </div>
            <div class="flex items-center gap-2 text-sm">
              <RadioGroupItem id="avatar-mode-custom-url" value="custom_url" />
              <Label for="avatar-mode-custom-url" class="cursor-pointer font-normal">使用 HTTPS 图片 URL</Label>
            </div>
          </RadioGroup>
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
      <Card class="p-4"><div class="flex items-center justify-between gap-3"><h2 class="font-semibold">资料完整度</h2><strong class="text-primary">{{ profileCompleteness === null ? '暂不可用' : `${profileCompleteness}%` }}</strong></div><div class="mt-3 h-2 overflow-hidden rounded-full bg-muted" role="progressbar" aria-label="资料完整度" aria-valuemin="0" aria-valuemax="100" :aria-valuenow="profileCompleteness ?? undefined"><div class="h-full rounded-full bg-primary" :style="{ width: `${profileCompleteness ?? 0}%` }"></div></div><p class="mt-3 text-xs leading-5 text-muted-foreground">显示名称、简介、身份绑定、恢复方式和联系方式共同组成完整度；发布 API 服务后还会检查收款设置。</p></Card>
      <Card class="p-4"><div class="flex gap-3"><ShieldCheck class="mt-0.5 h-5 w-5 shrink-0 text-primary" /><div><h2 class="font-semibold">公开信息说明</h2><p class="mt-2 text-xs leading-5 text-muted-foreground">公开主页展示名称、简介和你允许公开的交易信号，不会公开完整联系方式。</p></div></div></Card>
      </aside>
    </section>

    <section v-else-if="activeSection === 'contacts'" class="my-center-contacts-layout">
      <aside class="my-center-contacts-aside">
        <ConfigurationProgressCard
          :completed-count="completedContactSettingsCount"
          :wechat-complete="wechatBound"
          :email-complete="emailBound"
          :payment-complete="savedApiPaymentComplete"
          :show-payment="canPublishApiService"
          @preview="buyerPreviewOpen = true"
        />
      </aside>

      <main class="contact-payment-main-grid min-w-0">
        <div class="contact-payment-group-heading">
          <div>
            <h2>联系方式</h2>
            <p>当前真实支持微信和验证邮箱，买家只能在有效联系窗口查看。</p>
          </div>
          <Badge variant="secondary">已配置 {{ enabledContactCount }} / 2</Badge>
        </div>

        <ContactMethodCard
          title="微信"
          description="配置微信号后即可作为联系窗口方式，平台不做外部验证。"
          :status-label="wechatBound ? '已配置' : '未配置'"
          :status-variant="wechatBound ? 'verified' : 'secondary'"
          :dirty="wechatContactDirty"
          :is-default="wechatContact?.isDefault"
          :current-summary="wechatContact ? `当前：${wechatContact.maskedValue} · 自动用于拼车和 API 交易` : ''"
        >
          <template #icon><MessageCircle class="h-5 w-5" /></template>
          <template v-if="wechatContact" #actions>
            <Button
              v-if="!wechatContact.isDefault"
              size="icon"
              variant="outline"
              :disabled="setDefaultContactMutation.isPending.value"
              aria-label="将微信设为默认联系方式"
              title="设为默认"
              @click="setDefaultContact(wechatContact)"
            >
              <Star class="h-4 w-4" />
            </Button>
            <Button
              size="icon"
              variant="outline"
              :disabled="deleteContactMutation.isPending.value"
              aria-label="删除微信配置"
              title="删除配置"
              @click="removeContact(wechatContact)"
            >
              <Trash2 class="h-4 w-4" />
            </Button>
          </template>
          <div class="grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto]">
            <label class="space-y-2">
              <span class="text-sm font-medium">微信号</span>
              <Input v-model="wechatForm.displayValue" autocomplete="off" placeholder="例如 c2c_orbit" />
            </label>
            <Button
              class="sm:self-end"
              :disabled="contactSaving || !wechatForm.displayValue.trim() || !wechatContactDirty"
              @click="saveWechatContact"
            >
              <Save class="h-4 w-4" />保存微信
            </Button>
          </div>
          <p class="mt-3 text-xs leading-5 text-muted-foreground">微信配置后自动用于拼车和 API 交易，不代表平台已验证该微信号。</p>
        </ContactMethodCard>

        <ContactMethodCard
          title="邮箱"
          description="邮箱必须通过验证码后才会启用为联系方式。"
          :status-label="emailBound ? '已验证' : '未验证'"
          :status-variant="emailBound ? 'verified' : 'secondary'"
          :dirty="emailContactDirty"
          :is-default="emailContact?.isDefault"
          :current-summary="emailContact ? `当前：${emailContact.maskedValue} · 适用：${scopeLabels(emailContact.usageScopes)}` : ''"
        >
          <template #icon><Mail class="h-5 w-5" /></template>
          <template v-if="emailContact" #actions>
            <Button
              v-if="emailBound && !emailContact.isDefault"
              size="icon"
              variant="outline"
              :disabled="setDefaultContactMutation.isPending.value"
              aria-label="将邮箱设为默认联系方式"
              title="设为默认"
              @click="setDefaultContact(emailContact)"
            >
              <Star class="h-4 w-4" />
            </Button>
            <Button
              size="icon"
              variant="outline"
              :disabled="deleteContactMutation.isPending.value"
              aria-label="解除邮箱绑定"
              title="解除绑定"
              @click="removeContact(emailContact)"
            >
              <Trash2 class="h-4 w-4" />
            </Button>
          </template>
          <div class="grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto]">
            <label class="space-y-2">
              <span class="text-sm font-medium">邮箱地址</span>
              <Input v-model="contactEmailForm.email" type="email" autocomplete="email" placeholder="name@example.com" />
            </label>
            <Button
              class="sm:self-end"
              variant="outline"
              :disabled="emailBindingPending || contactEmailVerificationCooldownSeconds > 0 || !contactEmailForm.email.trim()"
              @click="startContactEmailVerification"
            >
              <MailCheck class="h-4 w-4" />{{ contactEmailVerificationButtonLabel }}
            </Button>
            <label class="space-y-2">
              <span class="text-sm font-medium">验证码</span>
              <Input v-model="contactEmailForm.code" inputmode="numeric" maxlength="6" placeholder="6 位验证码" />
              <span v-if="visibleContactEmailVerificationDevCode" class="block rounded-md border border-dashed border-warning/35 bg-warning/10 px-3 py-2 text-xs leading-5 text-warning">
                开发验证码：<span class="font-semibold tabular-nums">{{ visibleContactEmailVerificationDevCode }}</span>
              </span>
            </label>
            <Button
              class="sm:self-end"
              :disabled="emailBindingPending || !contactEmailForm.code.trim() || !contactEmailForm.usageScopes.length || contactEmailForm.email.trim().toLowerCase() !== contactEmailVerificationChallengeEmail"
              @click="confirmContactEmailVerification"
            >
              验证并绑定邮箱
            </Button>
          </div>
          <div v-if="profile?.emailVerified && profile.email" class="mt-3 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
            <Button type="button" size="sm" variant="ghost" class="h-8 px-2" @click="useVerifiedAccountEmailForContact">
              <Mail class="h-3.5 w-3.5" />使用已验证账号邮箱
            </Button>
            <span>保存后仍需单独验证，并会在符合交易披露条件时向交易对方展示。</span>
          </div>
          <div class="mt-4 grid gap-3 border-t border-border pt-4 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end">
            <ContactUsageScopeSelector
              v-model="contactEmailForm.usageScopes"
              :options="availableContactUsageScopeOptions"
            />
            <Button
              v-if="emailContact"
              variant="outline"
              :disabled="contactSaving || emailValueDirty || !emailUsageScopesDirty || !contactEmailForm.usageScopes.length"
              @click="saveEmailUsageScopes"
            >
              <Save class="h-4 w-4" />保存适用场景
            </Button>
          </div>
        </ContactMethodCard>

        <ApiPaymentSettingsEditor
          v-if="canPublishApiService"
          :settings="apiPaymentSettingsValue"
          @dirty-change="apiPaymentEditorDirty = $event"
        />
      </main>
    </section>

    <section v-else-if="activeSection === 'account'">
      <Card class="p-5">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <div class="flex flex-wrap items-center gap-2">
              <h2 class="font-semibold">账号状态</h2>
              <Badge :variant="accountRecoveryComplete ? 'verified' : 'secondary'">{{ accountRecoveryComplete ? '已完成' : '待完善' }}</Badge>
            </div>
            <p class="mt-2 max-w-2xl text-sm leading-6 text-muted-foreground">
              linux.do 身份、密码、邮箱和账号限制集中在这里查看；需要补全时会在弹框内完成。
            </p>
          </div>
          <div class="flex flex-wrap gap-2">
            <Button v-if="canConfigureBackupPassword && !profile.passwordConfigured" @click="openAccountSetupDialog('password')"><LockKeyhole class="h-4 w-4" />设置备用密码</Button>
            <Button v-else-if="canConfigureBackupPassword" variant="outline" @click="openAccountSetupDialog('password')"><LockKeyhole class="h-4 w-4" />修改备用密码</Button>
            <Button v-if="!profile.emailVerified" variant="outline" @click="openAccountSetupDialog('email')"><MailCheck class="h-4 w-4" />绑定邮箱</Button>
            <Button v-else variant="outline" @click="openAccountSetupDialog('email')"><MailCheck class="h-4 w-4" />更新邮箱</Button>
            <Button v-if="accountRecoveryComplete && accountRecoveryReturnTo" @click="continueAfterAccountRecovery">{{ accountRecoveryContinueLabel }}</Button>
          </div>
        </div>

        <div class="mt-5 grid gap-x-10 gap-y-5 lg:grid-cols-2">
          <div class="space-y-3 text-sm">
            <h3 class="text-sm font-semibold">linux.do 身份</h3>
            <div class="flex justify-between gap-4"><span class="text-muted-foreground">绑定状态</span><span>{{ profile.linuxDoBinding.bound ? '已绑定 linux.do' : '未绑定' }}</span></div>
            <div class="flex justify-between gap-4"><span class="text-muted-foreground">用户名</span><span>{{ profile.linuxDoBinding.linuxDoUsername ? `@${profile.linuxDoBinding.linuxDoUsername}` : '—' }}</span></div>
            <div class="flex justify-between gap-4"><span class="text-muted-foreground">用户 ID</span><span>{{ profile.linuxDoBinding.linuxDoUserId ?? '—' }}</span></div>
            <div class="flex justify-between gap-4"><span class="text-muted-foreground">信任等级</span><span>{{ profile.linuxDoBinding.trustLevel ?? '—' }}</span></div>
            <div class="flex justify-between gap-4"><span class="text-muted-foreground">头像同步</span><span>{{ profile.avatarMode === 'linuxdo' ? '跟随 linux.do' : '自定义头像' }}</span></div>
            <div class="flex justify-between gap-4"><span class="text-muted-foreground">最近同步</span><span>{{ profile.linuxDoBinding.lastSyncedAt }}</span></div>
          </div>

          <div class="space-y-3 text-sm">
            <h3 class="text-sm font-semibold">邮箱、备用密码与限制</h3>
            <div class="flex justify-between gap-4"><span class="text-muted-foreground">账号状态</span><span>{{ profile.accountStatus }}</span></div>
            <div class="flex justify-between gap-4"><span class="text-muted-foreground">绑定邮箱</span><span>{{ profile.emailVerified ? profile.email : '未绑定' }}</span></div>
            <div class="flex justify-between gap-4"><span class="text-muted-foreground">备用密码</span><span>{{ canConfigureBackupPassword ? (profile.passwordConfigured ? '已设置' : '未设置') : '不适用（仅 linux.do 账号）' }}</span></div>
            <div class="flex justify-between gap-4"><span class="text-muted-foreground">功能限制</span><span>{{ profile.restrictions.length ? profile.restrictions.join('、') : '无' }}</span></div>
            <div class="space-y-2">
              <span class="block text-muted-foreground">系统铭牌</span>
              <div class="flex flex-wrap gap-2"><Badge v-for="badge in profile.badges" :key="badge.id" variant="secondary">{{ badge.label }}</Badge></div>
            </div>
            <div class="space-y-2">
              <span class="block text-muted-foreground">社区身份</span>
              <div v-if="profile.communityIdentities.length" class="flex flex-wrap gap-2">
                <Badge v-for="identity in profile.communityIdentities" :key="`${identity.code}-${identity.grantedAt}`" variant="outline">
                  {{ identity.name }}<span v-if="identity.revokedAt" class="ml-1 text-muted-foreground">（已撤销）</span>
                </Badge>
              </div>
              <span v-else class="text-muted-foreground">暂无</span>
              <p class="text-xs leading-5 text-muted-foreground">社区身份只记录参与经历，不代表交易信用认证、平台担保或服务能力评价。</p>
            </div>
          </div>
        </div>

        <div v-if="!profile.linuxDoBinding.bound" class="mt-5 rounded-lg border border-border bg-muted/20 p-4">
          <h3 class="font-semibold">绑定 linux.do 身份</h3>
          <p class="mt-1 text-sm leading-6 text-muted-foreground">先使用当前密码完成近期身份验证，再前往 linux.do 授权。绑定会保留当前账号、邮箱、密码和交易历史。</p>
          <div class="mt-4 grid gap-3 md:grid-cols-[minmax(0,1fr)_auto_auto] md:items-end">
            <div class="space-y-2">
              <label for="linuxdo-link-password" class="text-sm font-medium">当前密码</label>
              <PasswordVisibilityInput
                id="linuxdo-link-password"
                v-model="linuxDoLinkPassword"
                label="当前密码"
                autocomplete="current-password"
              />
            </div>
            <Button variant="outline" :disabled="linuxDoReauthLoading || !linuxDoLinkPassword" @click="confirmRecentPassword">
              <KeyRound class="h-4 w-4" />{{ linuxDoReauthLoading ? '验证中…' : linuxDoRecentlyReauthenticated ? '已验证' : '验证密码' }}
            </Button>
            <Button :disabled="linuxDoLinkLoading || !linuxDoRecentlyReauthenticated" @click="linkLinuxDoIdentity">
              <Link2 class="h-4 w-4" />{{ linuxDoLinkLoading ? '跳转中…' : '绑定 linux.do' }}
            </Button>
          </div>
        </div>

	        <div class="mt-5 flex flex-wrap gap-2">
	          <Button variant="outline" @click="router.push('/my/profile')">切换头像跟随模式</Button>
          <Button variant="outline" @click="router.push('/my/reports')"><ShieldAlert class="h-4 w-4" />举报与申诉</Button>
        </div>
        <p class="mt-4 rounded-md border border-border bg-accent/50 p-3 text-xs leading-5 text-muted-foreground">
          linux.do 绑定不可自助解绑或换绑；异常情况请联系管理员人工处理。
        </p>
      </Card>
    </section>

    <section v-else-if="activeSection === 'privacy'" class="grid gap-4 lg:grid-cols-2">
      <Card class="p-5">
        <h2 class="font-semibold">公开主页隐私设置</h2>
        <div class="mt-4 space-y-3">
          <div class="flex items-center justify-between gap-4 text-sm"><span id="privacy-last-active">展示最近活跃时间</span><Switch v-model="privacyForm.showLastActiveAt" aria-labelledby="privacy-last-active" /></div>
          <div class="flex items-center justify-between gap-4 text-sm"><span id="privacy-created-at">展示加入时间</span><Switch v-model="privacyForm.showCreatedAt" aria-labelledby="privacy-created-at" /></div>
          <div class="flex items-center justify-between gap-4 text-sm"><span id="privacy-completion-stats">展示近期完成数量</span><Switch v-model="privacyForm.showCompletionStats" aria-labelledby="privacy-completion-stats" /></div>
          <div class="flex items-center justify-between gap-4 text-sm"><span id="privacy-response-median">展示响应中位时间</span><Switch v-model="privacyForm.showResponseMedian" aria-labelledby="privacy-response-median" /></div>
          <div class="flex items-center justify-between gap-4 text-sm"><span id="privacy-resolved-disputes">展示已处理纠纷摘要</span><Switch v-model="privacyForm.showResolvedDisputeSummary" aria-labelledby="privacy-resolved-disputes" /></div>
          <div class="flex items-center justify-between gap-4 text-sm"><span id="privacy-public-report">允许他人从公开主页举报</span><Switch v-model="privacyForm.allowPublicProfileReport" aria-labelledby="privacy-public-report" /></div>
        </div>
        <Button class="mt-5" :disabled="updateProfileMutation.isPending.value || !privacySettingsDirty" @click="savePrivacy"><Save class="h-4 w-4" />保存隐私设置</Button>
      </Card>
      <Card class="p-5">
        <h2 class="font-semibold">不能关闭的公开信号</h2>
        <div class="mt-4 space-y-3 text-sm text-muted-foreground">
          <p>买家/卖家的公共最小信誉摘要、账号处罚状态、严重未解决纠纷提示、系统认证铭牌和已绑定 linux.do 状态始终会在必要位置展示。</p>
          <p>上方开关只控制资料统计和详细记录，不会把公共信誉摘要改成零，也不会隐藏有效限制或风险状态。</p>
          <p>隐私设置不影响有效意向参与者查看必要联系方式。</p>
          <p>公开主页不会展示微信、邮箱、登录邮箱、手机号、IP、设备信息或意向敏感详情。</p>
        </div>
      </Card>
    </section>
    </AccountSettingsShell>
  </div>
</template>
