<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { Save } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import PaymentMethodCard from '@/components/contact-payment/PaymentMethodCard.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { RadioGroup } from '@/components/ui/radio-group'
import {
  apiPaymentMethodLabels,
  apiPaymentSettingsMissingReason,
  apiPaymentSettingsSummary,
  cloneApiPaymentAccountSettings,
  defaultApiPaymentWindowMinutes,
  isApiPaymentAccountSettingsComplete,
  type ApiPaymentAccountSettings,
  type ApiPaymentMethod,
  type ApiPaymentOption,
} from '@/lib/apiPaymentSettings'
import { containsSensitiveContent } from '@/lib/formValidation'
import { useUpdateApiPaymentAccountSettingsMutation } from '@/queries/useMarketQueries'

const props = withDefaults(defineProps<{
  settings: ApiPaymentAccountSettings
  layout?: 'page' | 'dialog'
}>(), {
  layout: 'page',
})

const emit = defineEmits<{
  cancel: []
  'dirty-change': [dirty: boolean]
  saved: [settings: ApiPaymentAccountSettings]
}>()

const apiPaymentQrMaxBytes = 512 * 1024
const updateMutation = useUpdateApiPaymentAccountSettingsMutation()
const savedSnapshot = ref(cloneApiPaymentAccountSettings(props.settings))
const form = reactive<Omit<ApiPaymentAccountSettings, 'updatedAt'>>({
  paymentWindowMinutes: props.settings.paymentWindowMinutes,
  paymentOptions: props.settings.paymentOptions.map(option => ({ ...option })),
})
const pendingQrRemoval = ref<ApiPaymentMethod | null>(null)

function settingsSignature(settings: Pick<ApiPaymentAccountSettings, 'paymentWindowMinutes' | 'paymentOptions'>) {
  return JSON.stringify({
    paymentWindowMinutes: settings.paymentWindowMinutes,
    paymentOptions: settings.paymentOptions.map(option => ({
      paymentMethod: option.paymentMethod,
      enabled: option.enabled,
      paymentInstructions: option.paymentInstructions.trim(),
      paymentQrCodeDataUrl: option.paymentQrCodeDataUrl,
    })),
  })
}

const complete = computed(() => isApiPaymentAccountSettingsComplete(form))
const missingReason = computed(() => apiPaymentSettingsMissingReason(form))
const summary = computed(() => apiPaymentSettingsSummary(form))
const dirty = computed(() => settingsSignature(form) !== settingsSignature(savedSnapshot.value))
const selectedPaymentMethod = computed<ApiPaymentMethod | ''>({
  get: () => form.paymentOptions.find(option => option.enabled)?.paymentMethod ?? '',
  set: value => {
    if (!value) return
    selectPaymentMethod(value)
  },
})

watch(dirty, value => emit('dirty-change', value), { immediate: true })

watch(
  () => props.settings,
  settings => {
    if (dirty.value) return
    resetDraft(settings)
  },
  { deep: true },
)

function resetDraft(settings: ApiPaymentAccountSettings) {
  const snapshot = cloneApiPaymentAccountSettings(settings)
  savedSnapshot.value = snapshot
  form.paymentWindowMinutes = snapshot.paymentWindowMinutes
  form.paymentOptions = snapshot.paymentOptions.map(option => ({ ...option }))
}

function optionDirty(option: ApiPaymentOption) {
  const savedOption = savedSnapshot.value.paymentOptions.find(item => item.paymentMethod === option.paymentMethod)
  return settingsSignature({
    paymentWindowMinutes: defaultApiPaymentWindowMinutes,
    paymentOptions: [option],
  }) !== settingsSignature({
    paymentWindowMinutes: defaultApiPaymentWindowMinutes,
    paymentOptions: [savedOption ?? {
      paymentMethod: option.paymentMethod,
      enabled: false,
      paymentInstructions: '',
      paymentQrCodeDataUrl: null,
    }],
  })
}

function selectPaymentMethod(paymentMethod: ApiPaymentMethod) {
  for (const option of form.paymentOptions) {
    option.enabled = option.paymentMethod === paymentMethod
  }
}

function handleQrUpload(event: Event, option: ApiPaymentOption) {
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

function confirmQrRemoval() {
  const paymentMethod = pendingQrRemoval.value
  if (!paymentMethod) return
  const option = form.paymentOptions.find(item => item.paymentMethod === paymentMethod)
  if (option) option.paymentQrCodeDataUrl = null
  pendingQrRemoval.value = null
}

function save() {
  if (!complete.value) {
    toast.warning(missingReason.value || '请先补全 API 收款设置。')
    return
  }
  if (containsSensitiveContent(form.paymentOptions.map(option => option.paymentInstructions))) {
    toast.warning('收款说明不能包含 API Key、token、密码、Session、Cookie、付款码或面板凭据。')
    return
  }
  updateMutation.mutate({
    paymentWindowMinutes: defaultApiPaymentWindowMinutes,
    paymentOptions: form.paymentOptions.map(option => ({
      ...option,
      paymentInstructions: option.paymentInstructions.trim(),
    })),
  }, {
    onSuccess: settings => {
      resetDraft(settings)
      toast.success('API 收款设置已保存。')
      emit('saved', settings)
    },
    onError: error => toast.error(error instanceof Error ? error.message : 'API 收款设置保存失败。'),
  })
}
</script>

<template>
  <section
    :class="layout === 'page' ? 'contact-payment-settings' : 'space-y-4'"
    data-api-payment-settings-editor
  >
    <div v-if="layout === 'page'" class="contact-payment-settings__header">
      <div class="min-w-0">
        <div class="flex flex-wrap items-center gap-2">
          <h2>API 收款设置</h2>
          <Badge :variant="complete ? 'verified' : 'secondary'">
            {{ complete ? '已配置' : '待配置' }}
          </Badge>
          <Badge v-if="dirty" variant="secondary">有未保存更改</Badge>
        </div>
        <p>发布 API 额度时读取这里的资料，并复制为服务接单快照。</p>
        <p class="mt-1">{{ summary }}</p>
      </div>
      <Button :disabled="updateMutation.isPending.value || !dirty" @click="save">
        <Save class="h-4 w-4" />保存 API 收款设置
      </Button>
    </div>

    <div v-else class="space-y-2">
      <div class="flex flex-wrap items-center gap-2">
        <Badge :variant="complete ? 'verified' : 'secondary'">
          {{ complete ? '已配置' : '待配置' }}
        </Badge>
        <Badge v-if="dirty" variant="secondary">有未保存更改</Badge>
      </div>
      <p class="text-xs leading-5 text-muted-foreground">{{ summary }}</p>
    </div>

    <div class="contact-payment-window">
      <span class="font-medium">买家确认付款窗口</span>
      <span class="text-muted-foreground">固定 {{ defaultApiPaymentWindowMinutes }} 分钟</span>
    </div>

    <RadioGroup v-model="selectedPaymentMethod" class="contact-payment-options-grid">
      <PaymentMethodCard
        v-for="option in form.paymentOptions"
        :key="option.paymentMethod"
        :option="option"
        :dirty="optionDirty(option)"
        :disabled="updateMutation.isPending.value"
        @select="selectPaymentMethod(option.paymentMethod)"
        @update:instructions="option.paymentInstructions = $event"
        @upload="handleQrUpload($event, option)"
        @request-remove-qr="pendingQrRemoval = option.paymentMethod"
      />
    </RadioGroup>

    <p
      class="contact-payment-status"
      :class="complete ? 'is-complete' : 'is-incomplete'"
    >
      {{ complete ? 'API 发布页将直接读取这组设置，不需要每次重新填写。' : missingReason }}
    </p>
    <p class="text-xs leading-5 text-muted-foreground">
      收款资料只在买家创建订单后用于站外确认。请勿填写银行卡号、API Key、token、账号密码、Cookie、Session 或面板凭据。
    </p>

    <div
      v-if="layout === 'dialog'"
      class="sticky bottom-0 -mx-4 -mb-4 flex flex-col-reverse gap-2 border-t border-border bg-background/95 px-4 py-3 backdrop-blur sm:-mx-5 sm:-mb-5 sm:flex-row sm:justify-end sm:px-5"
    >
      <Button variant="outline" :disabled="updateMutation.isPending.value" @click="emit('cancel')">
        取消
      </Button>
      <Button :disabled="updateMutation.isPending.value || !dirty" @click="save">
        <Save class="h-4 w-4" />保存设置
      </Button>
    </div>

    <Dialog
      :open="pendingQrRemoval !== null"
      @update:open="open => { if (!open) pendingQrRemoval = null }"
    >
      <DialogContent class="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>删除收款码？</DialogTitle>
          <DialogDescription>
            将从{{ pendingQrRemoval ? apiPaymentMethodLabels[pendingQrRemoval] : '' }}配置草稿中移除图片。删除后需保存 API 收款设置才会生效。
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" @click="pendingQrRemoval = null">取消</Button>
          <Button variant="destructive" @click="confirmQrRemoval">删除收款码</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </section>
</template>
