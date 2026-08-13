<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

type TurnstileWidgetOptions = {
  sitekey: string
  action: string
  callback: (token: string) => void
  'error-callback': () => void
  'expired-callback': () => void
  'timeout-callback': () => void
  'response-field': boolean
}

type TurnstileAPI = {
  render: (container: HTMLElement, options: TurnstileWidgetOptions) => string
  reset: (widgetId: string) => void
  remove: (widgetId: string) => void
}

declare global {
  interface Window {
    turnstile?: TurnstileAPI
  }
}

const props = defineProps<{
  siteKey: string
  action: string
}>()

const emit = defineEmits<{
  'update:token': [token: string]
}>()

const scriptURL = 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit'
let scriptPromise: Promise<TurnstileAPI> | null = null

const container = ref<HTMLElement | null>(null)
const errorMessage = ref('')
let api: TurnstileAPI | null = null
let widgetId: string | null = null
let disposed = false

function loadScript() {
  if (window.turnstile) return Promise.resolve(window.turnstile)
  if (scriptPromise) return scriptPromise

  scriptPromise = new Promise<TurnstileAPI>((resolve, reject) => {
    const existing = document.querySelector<HTMLScriptElement>(`script[src="${scriptURL}"]`)
    const script = existing ?? document.createElement('script')
    const handleLoad = () => {
      if (window.turnstile) resolve(window.turnstile)
      else reject(new Error('Turnstile API did not initialize.'))
    }
    const handleError = () => reject(new Error('Turnstile script failed to load.'))

    script.addEventListener('load', handleLoad, { once: true })
    script.addEventListener('error', handleError, { once: true })
    if (!existing) {
      script.src = scriptURL
      script.async = true
      script.defer = true
      document.head.appendChild(script)
    }
  }).catch((error) => {
    scriptPromise = null
    throw error
  })
  return scriptPromise
}

function clearToken(message = '') {
  emit('update:token', '')
  errorMessage.value = message
}

async function renderWidget() {
  if (!props.siteKey.trim()) {
    clearToken('人机验证配置缺失，请稍后再试。')
    return
  }
  try {
    api = await loadScript()
    if (disposed || !container.value) return
    widgetId = api.render(container.value, {
      sitekey: props.siteKey,
      action: props.action,
      callback: (token) => {
        errorMessage.value = ''
        emit('update:token', token)
      },
      'error-callback': () => clearToken('人机验证加载失败，请刷新后重试。'),
      'expired-callback': () => clearToken('人机验证已过期，请重新验证。'),
      'timeout-callback': () => clearToken('人机验证已超时，请重新验证。'),
      'response-field': false,
    })
  } catch {
    if (!disposed) clearToken('人机验证加载失败，请刷新后重试。')
  }
}

function reset() {
  clearToken()
  if (api && widgetId) api.reset(widgetId)
}

defineExpose({ reset })

onMounted(renderWidget)

onBeforeUnmount(() => {
  disposed = true
  clearToken()
  if (api && widgetId) api.remove(widgetId)
  widgetId = null
})
</script>

<template>
  <div class="space-y-2">
    <div ref="container" aria-label="Cloudflare 人机验证"></div>
    <p v-if="errorMessage" role="alert" class="text-xs leading-5 text-destructive">
      {{ errorMessage }}
    </p>
  </div>
</template>
