import { computed, ref, watch, type Ref } from 'vue'
import type { OwnerAPIHealthProbeConfig } from '@/types/apiHealth'

export function useOwnerAPIHealthProbeForm(config: Ref<OwnerAPIHealthProbeConfig | null | undefined>) {
  const baseUrl = ref('')
  const model = ref('')
  const credential = ref('')
  const enabled = ref(false)
  const touched = ref(false)

  watch(config, value => {
    baseUrl.value = value?.baseUrl ?? ''
    model.value = value?.model ?? ''
    enabled.value = value?.enabled ?? false
    credential.value = ''
    touched.value = false
  }, { immediate: true })

  const validation = computed(() => {
    const errors: Record<'baseUrl' | 'model' | 'credential', string> = {
      baseUrl: '',
      model: '',
      credential: '',
    }
    const trimmedBaseURL = baseUrl.value.trim()
    try {
      const parsed = new URL(trimmedBaseURL)
      if (parsed.protocol !== 'https:' || parsed.username || parsed.password || parsed.search || parsed.hash) {
        errors.baseUrl = '请输入不含账号、查询参数或片段的 HTTPS 地址。'
      }
    } catch {
      errors.baseUrl = '请输入有效的 HTTPS 地址。'
    }
    if (!model.value.trim()) errors.model = '请输入实际用于探测的模型。'
    if (enabled.value && !config.value?.credentialConfigured && !credential.value.trim()) {
      errors.credential = '首次启用前必须填写探针专用凭据。'
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
    }
  }

  return {
    baseUrl,
    model,
    credential,
    enabled,
    touched,
    validation,
    valid,
    markTouched,
    clearCredential,
    payload,
  }
}
