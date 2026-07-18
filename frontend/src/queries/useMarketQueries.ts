import { computed, type Ref } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import {
  addFeedbackSupplement,
  cancelApiOrder,
  createContactMethod,
  createContactReport,
  createPublicUserReport,
  confirmApiOrderComplete,
  confirmApiOrderPayment,
  reportApiOrderPaymentIssue,
  createApiOrderFromIntent,
  confirmEmailVerification,
  deleteContactMethod,
  deleteCustomAvatar,
  getAdminFeedbackTicket,
  getAdminFeedbackTickets,
  getAdminOverview,
  getAdminSectionRows,
  getApiPurchaseIntentById,
  getApiPurchaseIntentEvents,
  getApiPaymentAccountSettings,
  getApiOrderNotifications,
  getApiOrderById,
  getCarpoolApplicationById,
  getCarpoolApplicationEligibility,
  getCarpoolApplicationContacts,
  getCarpoolApplicationEvents,
  getCarpoolNotifications,
  getMerchantApiPurchaseIntents,
  getMerchantApiOrders,
  getMerchantCarpoolApplications,
  getMyContactMethods,
  getMyFeedbackTicket,
  getMyFeedbackTickets,
  getMyOfficialPriceLeads,
  getMyApiPurchaseIntents,
  getMyApiOrders,
  getMyApiServiceById,
  getMyApiServices,
  getMyCarpools,
  getMyCarpoolApplications,
  getMyProfile,
  getApiServiceById,
  getApiServices,
  getOtherApiMarketServices,
  getSub2ApiMarketServices,
  getCarpoolById,
  getCarpoolOpeningChannels,
  getCarpoolPaymentMethods,
  getCarpoolProductCatalog,
  getCarpoolRegions,
  getCarpools,
  getFavorites,
  getFeedbackUnreadCount,
  getHomeMarket,
  getModelCatalog,
  getNotifications,
  getOfficialPriceById,
  getOfficialPrices,
  getPublicMerchantProfile,
  getPublicUserProfile,
  getReviewCenterRows,
  getTransactionTrendSummary,
  handleFeedbackTicket,
  isFavorite,
  markAllNotificationsRead,
  markFeedbackRead,
  markNotificationRead,
  openApiOrderDispute,
  pauseApiService,
  publishApiService,
  searchMarket,
  sendContactVerification,
  setBackupPassword,
  setDefaultContactMethod,
  startEmailVerification,
  submitApiOrderDeliveryCredential,
  submitApiOrderPayment,
  submitFeedback,
  submitReview,
  resumeApiService,
  updateContactMethod,
  updateApiPaymentAccountSettings,
  updateMyProfile,
  toggleFavorite,
  useLinuxDoAvatar,
  verifyContactMethod,
  type AdminSection,
  type ApiOrderFilters,
  type ApiOrderPaymentIssueReason,
  type ApiPaymentOption,
  type SubmitApiOrderDeliveryCredentialPayload,
  type ApiPaymentAccountSettings,
  type ApiPurchaseIntentFilters,
  type ApiServiceFilters,
  type FavoriteTargetType,
  type FeedbackAdminHandlePayload,
  type FeedbackSupplementPayload,
  type OtherApiMarketFilters,
  type SubmitFeedbackPayload,
  type Sub2ApiMarketFilters,
  type CarpoolApplicationFilters,
  type CreateContactReportRequest,
  type CreatePublicProfileReportRequest,
  type SaveContactMethodRequest,
  type SetBackupPasswordRequest,
  type SubmitReviewPayload,
  type TransactionTrendRange,
  type UpdateMyProfileRequest,
  type UserProfile,
} from '@/lib/api'
import {
  closeDemand,
  getDemandById,
  getDemands,
  getMyDemands,
  submitDemand,
  type SubmitDemandPayload,
} from '@/features/demand/api'

function valueOf<T>(value: Ref<T> | T): T {
  return typeof value === 'object' && value !== null && 'value' in value ? value.value : value
}

export function useHomeMarket() {
  return useQuery({
    queryKey: ['home-market'],
    queryFn: getHomeMarket,
  })
}

export function transactionTrendQueryKey(productId: string, range: TransactionTrendRange) {
  return ['market', 'transaction-trend', productId, range] as const
}

export function useTransactionTrendSummary(productId: Ref<string> | string, range: Ref<TransactionTrendRange> | TransactionTrendRange) {
  return useQuery({
    queryKey: computed(() => transactionTrendQueryKey(valueOf(productId), valueOf(range))),
    queryFn: () => getTransactionTrendSummary(valueOf(productId), valueOf(range)),
    staleTime: 60_000,
    refetchOnWindowFocus: false,
    placeholderData: previousData => previousData,
  })
}

export function useOfficialPrices() {
  return useQuery({ queryKey: ['official-prices'], queryFn: getOfficialPrices })
}

export function useOfficialPrice(id: Ref<string> | string) {
  return useQuery({
    queryKey: computed(() => ['official-prices', valueOf(id)]),
    queryFn: () => getOfficialPriceById(valueOf(id)),
  })
}

export function useMyOfficialPriceLeads() {
  return useQuery({
    queryKey: ['my-official-price-leads'],
    queryFn: getMyOfficialPriceLeads,
    refetchOnMount: 'always',
  })
}

export function useCarpools() {
  return useQuery({ queryKey: ['carpools'], queryFn: getCarpools })
}

export function useCarpool(id: Ref<string> | string) {
  return useQuery({
    queryKey: computed(() => ['carpools', valueOf(id)]),
    queryFn: () => getCarpoolById(valueOf(id)),
  })
}

export function useCarpoolApplicationEligibility(id: Ref<string> | string, enabled: Ref<boolean> | boolean = true) {
  return useQuery({
    queryKey: computed(() => ['carpools', valueOf(id), 'eligibility']),
    queryFn: () => getCarpoolApplicationEligibility(valueOf(id)),
    enabled: computed(() => valueOf(enabled) && Boolean(valueOf(id))),
    retry: false,
  })
}

export function useMyCarpools(enabled: Ref<boolean> | boolean = true) {
  return useQuery({
    queryKey: ['my-carpools'],
    queryFn: getMyCarpools,
    enabled: computed(() => valueOf(enabled)),
    refetchOnMount: 'always',
  })
}

export function useMyCarpoolApplications(filters: Ref<CarpoolApplicationFilters> | CarpoolApplicationFilters = {}) {
  return useQuery({
    queryKey: computed(() => ['my-carpool-applications', valueOf(filters)]),
    queryFn: () => getMyCarpoolApplications(valueOf(filters)),
    refetchOnMount: 'always',
  })
}

export function useMerchantCarpoolApplications(filters: Ref<CarpoolApplicationFilters> | CarpoolApplicationFilters = {}) {
  return useQuery({
    queryKey: computed(() => ['merchant-carpool-applications', valueOf(filters)]),
    queryFn: () => getMerchantCarpoolApplications(valueOf(filters)),
    refetchOnMount: 'always',
  })
}

export function useCarpoolApplication(id: Ref<string> | string) {
  return useQuery({
    queryKey: computed(() => ['carpool-application', valueOf(id)]),
    queryFn: () => getCarpoolApplicationById(valueOf(id)),
  })
}

export function useCarpoolApplicationEvents(id: Ref<string> | string) {
  return useQuery({
    queryKey: computed(() => ['carpool-application-events', valueOf(id)]),
    queryFn: () => getCarpoolApplicationEvents(valueOf(id)),
  })
}

export function useCarpoolProductCatalog() {
  return useQuery({ queryKey: ['carpool-product-catalog', 'active'], queryFn: getCarpoolProductCatalog })
}

export function useCarpoolRegions() {
  return useQuery({ queryKey: ['carpool-regions', 'active'], queryFn: getCarpoolRegions })
}

export function useCarpoolOpeningChannels() {
  return useQuery({ queryKey: ['carpool-opening-channels', 'active'], queryFn: getCarpoolOpeningChannels })
}

export function useCarpoolPaymentMethods() {
  return useQuery({ queryKey: ['carpool-payment-methods', 'active'], queryFn: getCarpoolPaymentMethods })
}

export function useDemands() {
  return useQuery({ queryKey: ['demands'], queryFn: getDemands })
}

export function useMyDemands() {
  return useQuery({ queryKey: ['my-demands'], queryFn: getMyDemands })
}

export function useDemand(id: Ref<string> | string) {
  return useQuery({
    queryKey: computed(() => ['demands', valueOf(id)]),
    queryFn: () => getDemandById(valueOf(id)),
    enabled: computed(() => Boolean(valueOf(id))),
  })
}

export function useSubmitDemandMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (payload: SubmitDemandPayload) => submitDemand(payload),
    onSuccess(data) {
      queryClient.setQueryData(['demands', data.id], data)
      queryClient.invalidateQueries({ queryKey: ['demands'] })
      queryClient.invalidateQueries({ queryKey: ['my-demands'] })
      queryClient.invalidateQueries({ queryKey: ['home-market'] })
      queryClient.invalidateQueries({ queryKey: ['admin-section'] })
      queryClient.invalidateQueries({ queryKey: ['notifications'] })
    },
  })
}

export function useCloseDemandMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => closeDemand(id),
    onSuccess(data) {
      queryClient.setQueryData(['demands', data.id], data)
      queryClient.invalidateQueries({ queryKey: ['demands'] })
      queryClient.invalidateQueries({ queryKey: ['my-demands'] })
      queryClient.invalidateQueries({ queryKey: ['home-market'] })
      queryClient.invalidateQueries({ queryKey: ['admin-section'] })
      queryClient.invalidateQueries({ queryKey: ['notifications'] })
    },
  })
}

export function useModelCatalog() {
  return useQuery({ queryKey: ['model-catalog', 'active'], queryFn: getModelCatalog })
}

export function useApiServices(filters: Ref<ApiServiceFilters> | ApiServiceFilters = {}) {
  return useQuery({
    queryKey: computed(() => ['api-services', valueOf(filters)]),
    queryFn: () => getApiServices(valueOf(filters)),
  })
}

export function sub2ApiMarketQueryKey(filters: Sub2ApiMarketFilters) {
  return ['api-market', 'sub2api', filters] as const
}

export function otherApiMarketQueryKey(filters: OtherApiMarketFilters) {
  return ['api-market', 'other', filters] as const
}

export function useSub2ApiMarketQuery(filters: Ref<Sub2ApiMarketFilters> | Sub2ApiMarketFilters = {}) {
  return useQuery({
    queryKey: computed(() => sub2ApiMarketQueryKey(valueOf(filters))),
    queryFn: () => getSub2ApiMarketServices(valueOf(filters)),
    placeholderData: previousData => previousData,
  })
}

export function useOtherApiMarketQuery(filters: Ref<OtherApiMarketFilters> | OtherApiMarketFilters = {}) {
  return useQuery({
    queryKey: computed(() => otherApiMarketQueryKey(valueOf(filters))),
    queryFn: () => getOtherApiMarketServices(valueOf(filters)),
    placeholderData: previousData => previousData,
  })
}

export function useApiService(id: Ref<string> | string) {
  return useQuery({
    queryKey: computed(() => ['api-services', valueOf(id)]),
    retry: false,
    queryFn: () => getApiServiceById(valueOf(id)),
  })
}

export function useMyApiServices(enabled: Ref<boolean> | boolean = true) {
  return useQuery({
    queryKey: ['my-api-services'],
    queryFn: getMyApiServices,
    enabled: computed(() => valueOf(enabled)),
    refetchOnMount: 'always',
  })
}

export function useMyApiService(id: Ref<string> | string) {
  return useQuery({
    queryKey: computed(() => ['my-api-services', valueOf(id)]),
    retry: false,
    queryFn: () => getMyApiServiceById(valueOf(id)),
  })
}

export function myProfileQueryKey() {
  return ['my-profile'] as const
}

export function myContactMethodsQueryKey() {
  return ['my-contact-methods'] as const
}

export function apiPaymentAccountSettingsQueryKey() {
  return ['api-payment-account-settings'] as const
}

export function publicUserProfileQueryKey(username: string) {
  return ['public-user-profile', username] as const
}

export function orderContactsQueryKey(kind: 'carpool-application' | 'api-order', id: string) {
  return ['order-contacts', kind, id] as const
}

export function useMyProfileQuery(enabled: Ref<boolean> | boolean = true) {
  return useQuery({
    queryKey: myProfileQueryKey(),
    queryFn: getMyProfile,
    enabled: computed(() => valueOf(enabled)),
    retry: false,
    staleTime: 30_000,
    refetchOnWindowFocus: false,
  })
}

export function useUpdateMyProfileMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (payload: UpdateMyProfileRequest) => updateMyProfile(payload),
    onSuccess(data) {
      queryClient.setQueryData(myProfileQueryKey(), data)
      queryClient.invalidateQueries({ queryKey: ['public-user-profile', data.username] })
    },
  })
}

export function useUploadAvatarMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => Promise.reject(new Error('当前版本不支持本地头像上传，请填写 HTTPS 图片 URL。')),
    onSuccess() {
      queryClient.invalidateQueries({ queryKey: myProfileQueryKey() })
      queryClient.invalidateQueries({ queryKey: ['public-user-profile'] })
    },
  })
}

export function useSetBackupPasswordMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (payload: SetBackupPasswordRequest) => setBackupPassword(payload),
    onSuccess() {
      queryClient.setQueryData<UserProfile | undefined>(myProfileQueryKey(), current => current ? { ...current, passwordConfigured: true } : current)
      queryClient.invalidateQueries({ queryKey: myProfileQueryKey() })
    },
  })
}

export function useStartEmailVerificationMutation() {
  return useMutation({
    mutationFn: (email: string) => startEmailVerification(email),
  })
}

export function useConfirmEmailVerificationMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (payload: { email: string, code: string }) => confirmEmailVerification(payload),
    onSuccess(data) {
      queryClient.setQueryData(myProfileQueryKey(), data)
      queryClient.invalidateQueries({ queryKey: ['public-user-profile', data.username] })
    },
  })
}

export function useDeleteCustomAvatarMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: deleteCustomAvatar,
    onSuccess(data) {
      queryClient.setQueryData(myProfileQueryKey(), data)
      queryClient.invalidateQueries({ queryKey: ['public-user-profile', data.username] })
    },
  })
}

export function useUseLinuxDoAvatarMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: useLinuxDoAvatar,
    onSuccess(data) {
      queryClient.setQueryData(myProfileQueryKey(), data)
      queryClient.invalidateQueries({ queryKey: ['public-user-profile', data.username] })
    },
  })
}

export function useMyContactMethodsQuery() {
  return useQuery({
    queryKey: myContactMethodsQueryKey(),
    queryFn: getMyContactMethods,
    staleTime: 30_000,
    refetchOnWindowFocus: false,
  })
}

export function useCreateContactMethodMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (payload: SaveContactMethodRequest) => createContactMethod(payload),
    onSuccess() {
      queryClient.invalidateQueries({ queryKey: myContactMethodsQueryKey() })
    },
  })
}

export function useUpdateContactMethodMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ contactId, payload }: { contactId: string, payload: SaveContactMethodRequest }) => updateContactMethod(contactId, payload),
    onSuccess() {
      queryClient.invalidateQueries({ queryKey: myContactMethodsQueryKey() })
    },
  })
}

export function useDeleteContactMethodMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (contactId: string) => deleteContactMethod(contactId),
    onSuccess() {
      queryClient.invalidateQueries({ queryKey: myContactMethodsQueryKey() })
    },
  })
}

export function useSetDefaultContactMethodMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (contactId: string) => setDefaultContactMethod(contactId),
    onSuccess() {
      queryClient.invalidateQueries({ queryKey: myContactMethodsQueryKey() })
    },
  })
}

export function useSendContactVerificationMutation() {
  return useMutation({ mutationFn: (contactId: string) => sendContactVerification(contactId) })
}

export function useVerifyContactMethodMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (contactId: string) => verifyContactMethod(contactId),
    onSuccess() {
      queryClient.invalidateQueries({ queryKey: myContactMethodsQueryKey() })
    },
  })
}

export function useApiPaymentAccountSettingsQuery() {
  return useQuery({
    queryKey: apiPaymentAccountSettingsQueryKey(),
    queryFn: getApiPaymentAccountSettings,
    staleTime: 30_000,
    refetchOnWindowFocus: false,
  })
}

export function useUpdateApiPaymentAccountSettingsMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (payload: Omit<ApiPaymentAccountSettings, 'updatedAt'>) => updateApiPaymentAccountSettings(payload),
    onSuccess(data) {
      queryClient.setQueryData(apiPaymentAccountSettingsQueryKey(), data)
    },
  })
}

export function usePublicMerchantProfile(username: Ref<string> | string) {
  return useQuery({
    queryKey: computed(() => ['public-merchant-profile', valueOf(username)]),
    queryFn: () => getPublicMerchantProfile(valueOf(username)),
  })
}

export function usePublicUserProfileQuery(username: Ref<string> | string) {
  return useQuery({
    queryKey: computed(() => publicUserProfileQueryKey(valueOf(username))),
    queryFn: () => getPublicUserProfile(valueOf(username)),
    staleTime: 30_000,
    refetchOnWindowFocus: false,
  })
}

export function useCarpoolApplicationContactsQuery(applicationId: Ref<string> | string) {
  return useQuery({
    queryKey: computed(() => orderContactsQueryKey('carpool-application', valueOf(applicationId))),
    queryFn: () => getCarpoolApplicationContacts(valueOf(applicationId)),
    staleTime: 0,
    refetchOnWindowFocus: false,
  })
}

export function useCreateContactReportMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (payload: CreateContactReportRequest) => createContactReport(payload),
    onSuccess() {
      queryClient.invalidateQueries({ queryKey: ['admin-section'] })
    },
  })
}

export function useCreatePublicUserReportMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (payload: CreatePublicProfileReportRequest) => createPublicUserReport(payload),
    onSuccess(_data, payload) {
      queryClient.invalidateQueries({ queryKey: ['admin-section'] })
      queryClient.invalidateQueries({ queryKey: publicUserProfileQueryKey(payload.username) })
    },
  })
}

export function useMyApiPurchaseIntents(filters: Ref<ApiPurchaseIntentFilters> | ApiPurchaseIntentFilters = {}) {
  return useQuery({
    queryKey: computed(() => ['my-api-purchase-intents', valueOf(filters)]),
    queryFn: () => getMyApiPurchaseIntents(valueOf(filters)),
    refetchOnMount: 'always',
  })
}

export function useMerchantApiPurchaseIntents(filters: Ref<ApiPurchaseIntentFilters> | ApiPurchaseIntentFilters = {}) {
  return useQuery({
    queryKey: computed(() => ['merchant-api-purchase-intents', valueOf(filters)]),
    queryFn: () => getMerchantApiPurchaseIntents(valueOf(filters)),
    refetchOnMount: 'always',
  })
}

export function useApiPurchaseIntent(id: Ref<string> | string) {
  return useQuery({
    queryKey: computed(() => ['api-purchase-intents', valueOf(id)]),
    queryFn: () => getApiPurchaseIntentById(valueOf(id)),
  })
}

export function useApiPurchaseIntentEvents(id: Ref<string> | string) {
  return useQuery({
    queryKey: computed(() => ['api-purchase-intent-events', valueOf(id)]),
    queryFn: () => getApiPurchaseIntentEvents(valueOf(id)),
  })
}

export function useMyApiOrders(filters: Ref<ApiOrderFilters> | ApiOrderFilters = {}) {
  return useQuery({
    queryKey: computed(() => ['my-api-orders', valueOf(filters)]),
    queryFn: () => getMyApiOrders(valueOf(filters)),
    refetchOnMount: 'always',
  })
}

export function useMerchantApiOrders(filters: Ref<ApiOrderFilters> | ApiOrderFilters = {}) {
  return useQuery({
    queryKey: computed(() => ['merchant-api-orders', valueOf(filters)]),
    queryFn: () => getMerchantApiOrders(valueOf(filters)),
    refetchOnMount: 'always',
  })
}

export function useApiOrder(id: Ref<string> | string, perspective: Ref<'buyer' | 'merchant'> | 'buyer' | 'merchant' = 'buyer') {
  return useQuery({
    queryKey: computed(() => ['api-orders', valueOf(perspective), valueOf(id)]),
    queryFn: () => getApiOrderById(valueOf(id), valueOf(perspective)),
    enabled: computed(() => Boolean(valueOf(id))),
    refetchOnMount: 'always',
  })
}

function invalidateApiOrderQueries(queryClient: ReturnType<typeof useQueryClient>, id?: string) {
  queryClient.invalidateQueries({ queryKey: ['my-api-orders'] })
  queryClient.invalidateQueries({ queryKey: ['merchant-api-orders'] })
  queryClient.invalidateQueries({ queryKey: ['api-orders'] })
  queryClient.invalidateQueries({ queryKey: ['my-api-purchase-intents'] })
  queryClient.invalidateQueries({ queryKey: ['merchant-api-purchase-intents'] })
  queryClient.invalidateQueries({ queryKey: ['api-purchase-intents'] })
  queryClient.invalidateQueries({ queryKey: ['admin-section'] })
  queryClient.invalidateQueries({ queryKey: ['notifications'] })
  queryClient.invalidateQueries({ queryKey: ['navigation-badges'] })
  queryClient.invalidateQueries({ queryKey: ['api-order-notifications'] })
  if (id) {
    queryClient.invalidateQueries({ queryKey: ['api-orders', 'buyer', id] })
    queryClient.invalidateQueries({ queryKey: ['api-orders', 'merchant', id] })
  }
}

export function useCreateApiOrderFromIntentMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ intentId, paymentMethod }: { intentId: string, paymentMethod: ApiPaymentOption['paymentMethod'] }) => createApiOrderFromIntent(intentId, paymentMethod),
    onSuccess(data) {
      queryClient.setQueryData(['api-orders', 'buyer', data.id], data)
      invalidateApiOrderQueries(queryClient, data.id)
    },
  })
}

export function useSubmitApiOrderPaymentMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, paymentSummary, version }: { id: string, paymentSummary: string, version: number }) => submitApiOrderPayment(id, paymentSummary, version),
    onSuccess(data) {
      queryClient.setQueryData(['api-orders', 'buyer', data.id], data)
      invalidateApiOrderQueries(queryClient, data.id)
    },
  })
}

export function useCancelApiOrderMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, reason, version }: { id: string, reason: string, version: number }) => cancelApiOrder(id, reason, version),
    onSuccess(data) {
      queryClient.setQueryData(['api-orders', 'buyer', data.id], data)
      invalidateApiOrderQueries(queryClient, data.id)
    },
  })
}

export function useConfirmApiOrderCompleteMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, version }: { id: string, version: number }) => confirmApiOrderComplete(id, version),
    onSuccess(data) {
      queryClient.setQueryData(['api-orders', 'buyer', data.id], data)
      invalidateApiOrderQueries(queryClient, data.id)
    },
  })
}

export function useOpenApiOrderDisputeMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, reason, version, perspective }: { id: string, reason: string, version: number, perspective: 'buyer' | 'merchant' }) => openApiOrderDispute(id, reason, version, perspective),
    onSuccess(data, variables) {
      queryClient.setQueryData(['api-orders', variables.perspective, data.id], data)
      invalidateApiOrderQueries(queryClient, data.id)
    },
  })
}

export function useConfirmApiOrderPaymentMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, version }: { id: string, version: number }) => confirmApiOrderPayment(id, version),
    onSuccess(data) {
      queryClient.setQueryData(['api-orders', 'merchant', data.id], data)
      invalidateApiOrderQueries(queryClient, data.id)
    },
  })
}

export function useReportApiOrderPaymentIssueMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, reason, note, version }: { id: string, reason: ApiOrderPaymentIssueReason, note: string, version: number }) => reportApiOrderPaymentIssue(id, reason, note, version),
    onSuccess(data) {
      queryClient.setQueryData(['api-orders', 'merchant', data.id], data)
      invalidateApiOrderQueries(queryClient, data.id)
    },
  })
}

export function useSubmitApiOrderDeliveryCredentialMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, payload, version }: { id: string, payload: SubmitApiOrderDeliveryCredentialPayload, version: number }) => submitApiOrderDeliveryCredential(id, payload, version),
    onSuccess(data) {
      queryClient.setQueryData(['api-orders', 'merchant', data.id], data)
      invalidateApiOrderQueries(queryClient, data.id)
    },
  })
}

export function useApiOrderNotifications() {
  return useQuery({
    queryKey: ['api-order-notifications'],
    queryFn: getApiOrderNotifications,
    refetchOnMount: 'always',
  })
}

export function usePublishApiServiceMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => publishApiService(id),
    onSuccess(data) {
      queryClient.setQueryData(['api-services', data.id], data)
      queryClient.invalidateQueries({ queryKey: ['my-api-services'] })
      queryClient.invalidateQueries({ queryKey: ['api-services'] })
      queryClient.invalidateQueries({ queryKey: ['api-market'] })
      queryClient.invalidateQueries({ queryKey: ['home-market'] })
      queryClient.invalidateQueries({ queryKey: ['admin-section'] })
    },
  })
}

export function usePauseApiServiceMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => pauseApiService(id),
    onSuccess(data) {
      queryClient.setQueryData(['api-services', data.id], data)
      queryClient.invalidateQueries({ queryKey: ['my-api-services'] })
      queryClient.invalidateQueries({ queryKey: ['api-services'] })
      queryClient.invalidateQueries({ queryKey: ['api-market'] })
      queryClient.invalidateQueries({ queryKey: ['home-market'] })
      queryClient.invalidateQueries({ queryKey: ['admin-section'] })
    },
  })
}

export function useResumeApiServiceMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => resumeApiService(id),
    onSuccess(data) {
      queryClient.setQueryData(['api-services', data.id], data)
      queryClient.invalidateQueries({ queryKey: ['my-api-services'] })
      queryClient.invalidateQueries({ queryKey: ['api-services'] })
      queryClient.invalidateQueries({ queryKey: ['api-market'] })
      queryClient.invalidateQueries({ queryKey: ['home-market'] })
      queryClient.invalidateQueries({ queryKey: ['admin-section'] })
    },
  })
}

export function useCarpoolNotifications() {
  return useQuery({
    queryKey: ['carpool-notifications'],
    queryFn: getCarpoolNotifications,
    refetchOnMount: 'always',
  })
}

export function useNotifications(enabled: Ref<boolean> | boolean = true) {
  return useQuery({
    queryKey: ['notifications'],
    queryFn: getNotifications,
    enabled: computed(() => valueOf(enabled)),
    refetchOnMount: 'always',
  })
}

export function useMyFeedbackTickets() {
  return useQuery({
    queryKey: ['feedback'],
    queryFn: getMyFeedbackTickets,
    refetchOnMount: 'always',
  })
}

export function useMyFeedbackTicket(id: Ref<string> | string) {
  return useQuery({
    queryKey: computed(() => ['feedback', valueOf(id)]),
    queryFn: () => getMyFeedbackTicket(valueOf(id)),
    enabled: computed(() => Boolean(valueOf(id))),
    refetchOnMount: 'always',
  })
}

export function useFeedbackUnreadCount() {
  return useQuery({
    queryKey: ['feedback-unread-count'],
    queryFn: getFeedbackUnreadCount,
    refetchOnMount: 'always',
  })
}

export function useSubmitFeedbackMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (payload: SubmitFeedbackPayload) => submitFeedback(payload),
    onSuccess(data) {
      queryClient.setQueryData(['feedback', data.id], data)
      queryClient.invalidateQueries({ queryKey: ['feedback'] })
      queryClient.invalidateQueries({ queryKey: ['feedback-unread-count'] })
      queryClient.invalidateQueries({ queryKey: ['notifications'] })
      queryClient.invalidateQueries({ queryKey: ['navigation-badges'] })
      queryClient.invalidateQueries({ queryKey: ['admin-section'] })
    },
  })
}

export function useAddFeedbackSupplementMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, payload }: { id: string, payload: FeedbackSupplementPayload }) => addFeedbackSupplement(id, payload),
    onSuccess(data) {
      queryClient.setQueryData(['feedback', data.id], data)
      queryClient.invalidateQueries({ queryKey: ['feedback'] })
      queryClient.invalidateQueries({ queryKey: ['feedback-unread-count'] })
      queryClient.invalidateQueries({ queryKey: ['notifications'] })
      queryClient.invalidateQueries({ queryKey: ['navigation-badges'] })
      queryClient.invalidateQueries({ queryKey: ['admin-feedback'] })
      queryClient.invalidateQueries({ queryKey: ['admin-section'] })
    },
  })
}

export function useMarkFeedbackReadMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => markFeedbackRead(id),
    onSuccess(data) {
      queryClient.setQueryData(['feedback', data.id], data)
      queryClient.invalidateQueries({ queryKey: ['feedback'] })
      queryClient.invalidateQueries({ queryKey: ['feedback-unread-count'] })
      queryClient.invalidateQueries({ queryKey: ['notifications'] })
      queryClient.invalidateQueries({ queryKey: ['navigation-badges'] })
      queryClient.invalidateQueries({ queryKey: ['admin-feedback'] })
      queryClient.invalidateQueries({ queryKey: ['admin-section'] })
    },
  })
}

export function useAdminFeedbackTickets() {
  return useQuery({
    queryKey: ['admin-feedback'],
    queryFn: getAdminFeedbackTickets,
    refetchOnMount: 'always',
  })
}

export function useAdminFeedbackTicket(id: Ref<string> | string) {
  return useQuery({
    queryKey: computed(() => ['admin-feedback', valueOf(id)]),
    queryFn: () => getAdminFeedbackTicket(valueOf(id)),
    enabled: computed(() => Boolean(valueOf(id))),
    refetchOnMount: 'always',
  })
}

export function useHandleFeedbackTicketMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, payload, version }: { id: string, payload: FeedbackAdminHandlePayload, version?: number }) => handleFeedbackTicket(id, payload, version),
    onSuccess(data) {
      queryClient.setQueryData(['admin-feedback', data.id], data)
      queryClient.invalidateQueries({ queryKey: ['admin-feedback'] })
      queryClient.invalidateQueries({ queryKey: ['feedback'] })
      queryClient.invalidateQueries({ queryKey: ['feedback-unread-count'] })
      queryClient.invalidateQueries({ queryKey: ['notifications'] })
      queryClient.invalidateQueries({ queryKey: ['navigation-badges'] })
      queryClient.invalidateQueries({ queryKey: ['admin-section'] })
    },
  })
}

export function useMarkNotificationReadMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => markNotificationRead(id),
    onSuccess() {
      queryClient.invalidateQueries({ queryKey: ['notifications'] })
      queryClient.invalidateQueries({ queryKey: ['navigation-badges'] })
      queryClient.invalidateQueries({ queryKey: ['feedback'] })
      queryClient.invalidateQueries({ queryKey: ['feedback-unread-count'] })
      queryClient.invalidateQueries({ queryKey: ['api-order-notifications'] })
      queryClient.invalidateQueries({ queryKey: ['carpool-notifications'] })
    },
  })
}

export function useMarkAllNotificationsReadMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: markAllNotificationsRead,
    onSuccess() {
      queryClient.invalidateQueries({ queryKey: ['notifications'] })
      queryClient.invalidateQueries({ queryKey: ['navigation-badges'] })
      queryClient.invalidateQueries({ queryKey: ['feedback'] })
      queryClient.invalidateQueries({ queryKey: ['feedback-unread-count'] })
      queryClient.invalidateQueries({ queryKey: ['api-order-notifications'] })
      queryClient.invalidateQueries({ queryKey: ['carpool-notifications'] })
    },
  })
}

export function useFavorites() {
  return useQuery({
    queryKey: ['favorites'],
    queryFn: getFavorites,
    refetchOnMount: 'always',
  })
}

export function useFavoriteStatus(targetType: Ref<FavoriteTargetType> | FavoriteTargetType, targetId: Ref<string> | string, enabled: Ref<boolean> | boolean = true) {
  return useQuery({
    queryKey: computed(() => ['favorites', 'status', valueOf(targetType), valueOf(targetId)]),
    queryFn: () => isFavorite(valueOf(targetType), valueOf(targetId)),
    enabled: computed(() => valueOf(enabled) && Boolean(valueOf(targetId))),
  })
}

export function useToggleFavoriteMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ targetType, targetId }: { targetType: FavoriteTargetType, targetId: string }) => toggleFavorite(targetType, targetId),
    onSuccess(_data, variables) {
      queryClient.invalidateQueries({ queryKey: ['favorites'] })
      queryClient.invalidateQueries({ queryKey: ['favorites', 'status', variables.targetType, variables.targetId] })
    },
  })
}

export function useSearchMarket(keyword: Ref<string> | string) {
  return useQuery({
    queryKey: computed(() => ['search', valueOf(keyword)]),
    queryFn: () => searchMarket(valueOf(keyword)),
    enabled: computed(() => valueOf(keyword).trim().length > 0),
  })
}

export function useReviewCenterRows() {
  return useQuery({
    queryKey: ['review-center'],
    queryFn: getReviewCenterRows,
    refetchOnMount: 'always',
  })
}

export function useSubmitReviewMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (payload: SubmitReviewPayload) => submitReview(payload),
    onSuccess() {
      queryClient.invalidateQueries({ queryKey: ['review-center'] })
      queryClient.invalidateQueries({ queryKey: ['my-carpool-applications'] })
      queryClient.invalidateQueries({ queryKey: ['merchant-carpool-applications'] })
      queryClient.invalidateQueries({ queryKey: ['my-api-purchase-intents'] })
      queryClient.invalidateQueries({ queryKey: ['merchant-api-purchase-intents'] })
      queryClient.invalidateQueries({ queryKey: ['public-user-profile'] })
    },
  })
}

export function useAdminOverview() {
  return useQuery({ queryKey: ['admin-overview'], queryFn: getAdminOverview })
}

export function useAdminSectionRows(section: Ref<AdminSection> | AdminSection) {
  return useQuery({
    queryKey: computed(() => ['admin-section', valueOf(section)]),
    queryFn: () => getAdminSectionRows(valueOf(section)),
    refetchOnMount: 'always',
  })
}
