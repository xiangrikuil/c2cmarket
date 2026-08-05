import { computed, ref, watch, type Ref } from 'vue'
import type { OwnerAPIHealthProbeConfig } from '@/types/apiHealth'

export function useOwnerAPIHealthProbeForm(config: Ref<OwnerAPIHealthProbeConfig | null | undefined>) {
  const baseUrl = ref('')
  const model = ref('')
  const credential = ref('')
  const enabled = ref(false)
  const acknowledgeInsecureHttp = ref(false)
  const touched = ref(false)

  watch(config, value => {
    baseUrl.value = value?.baseUrl ?? ''
    model.value = value?.model ?? ''
    enabled.value = value?.enabled ?? false
    credential.value = ''
    acknowledgeInsecureHttp.value = false
    touched.value = false
  }, { immediate: true })

  const isInsecureHttp = computed(() => {
    try {
      return new URL(baseUrl.value.trim()).protocol === 'http:'
    } catch {
      return false
    }
  })

  const validation = computed(() => {
    const errors: Record<'baseUrl' | 'model' | 'credential' | 'acknowledgeInsecureHttp', string> = {
      baseUrl: '',
      model: '',
      credential: '',
      acknowledgeInsecureHttp: '',
    }
    const trimmedBaseURL = baseUrl.value.trim()
    try {
      const parsed = new URL(trimmedBaseURL)
      if (!['http:', 'https:'].includes(parsed.protocol) || parsed.username || parsed.password || parsed.search || parsed.hash) {
        errors.baseUrl = '请输入不含账号、查询参数或片段的 HTTP 或 HTTPS API 请求地址。'
      }
    } catch {
      errors.baseUrl = '请输入有效的 HTTP 或 HTTPS API 请求地址。'
    }
    if (!model.value.trim()) errors.model = '请输入实际用于探测的模型。'
    if (enabled.value && !config.value?.credentialConfigured && !credential.value.trim()) {
      errors.credential = '首次启用前必须填写探针专用 API Key。'
    }
    if (isInsecureHttp.value && !acknowledgeInsecureHttp.value) {
      errors.acknowledgeInsecureHttp = '使用 HTTP 请求地址前必须确认未加密传输风险。'
    }
    return errors
  })

  const valid = computed(() => Object.values(validation.value).every(message => !message))

  function markTouched() {
    touched.value = true
  }

  function clearCredential() {
    credential.value = ''
  }

  function payload(apiServiceId: string) {
    const trimmedCredential = credential.value.trim()
    return {
      apiServiceId,
      version: config.value?.version ?? 0,
      baseUrl: baseUrl.value.trim(),
      model: model.value.trim(),
      ...(trimmedCredential ? { credential: trimmedCredential } : {}),
      enabled: enabled.value,
      acknowledgeInsecureHttp: isInsecureHttp.value && acknowledgeInsecureHttp.value,
    }
  }

  return {
    baseUrl,
    model,
    credential,
    enabled,
    acknowledgeInsecureHttp,
    isInsecureHttp,
    touched,
    validation,
    valid,
    markTouched,
    clearCredential,
    payload,
  }
}
