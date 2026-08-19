<script setup lang="ts">
import { ref, watch } from 'vue'
import { Copy, ExternalLink, Eye, EyeOff } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { getApiOrderDeliveryKindLabel, type ApiOrderDeliveryCredential } from '@/lib/api'
import { formatOrderDateTime } from '@/lib/apiOrderUi'

const props = withDefaults(defineProps<{
  credential: ApiOrderDeliveryCredential
  title?: string
}>(), {
  title: '卖家交付内容',
})

const apiKeyVisible = ref(false)
const passwordVisible = ref(false)

watch(
  () => [props.credential.apiKey, props.credential.password],
  () => {
    apiKeyVisible.value = false
    passwordVisible.value = false
  },
)

async function copyValue(value: string | undefined, label: string) {
  if (!value) return
  try {
    await navigator.clipboard.writeText(value)
    toast.success(`已复制${label}。`)
  } catch {
    toast.error('复制失败，请手动选择文本。')
  }
}

function maskCredential(value: string | undefined) {
  if (!value) return ''
  if (value.length <= 8) return '••••••••'
  const maskedLength = Math.min(18, Math.max(8, value.length - 7))
  return `${value.slice(0, 3)}${'•'.repeat(maskedLength)}${value.slice(-4)}`
}
</script>

<template>
  <Card class="overflow-hidden border-primary/20">
    <div class="border-b border-border bg-primary/5 px-4 py-4 sm:px-5">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div class="min-w-0">
          <h2 class="font-semibold">{{ title }}</h2>
          <p class="mt-1 text-xs leading-5 text-muted-foreground">
            {{ getApiOrderDeliveryKindLabel(credential.deliveryKind) }} · {{ formatOrderDateTime(credential.submittedAt) }}
          </p>
        </div>
        <Badge :variant="credential.destroyedAt ? 'secondary' : 'verified'">
          {{ credential.destroyedAt ? '已销毁' : '保留期内可查看' }}
        </Badge>
      </div>
    </div>

    <div v-if="credential.destroyedAt" class="p-4 sm:p-5">
      <div class="rounded-md border border-border bg-muted/40 p-4 text-sm leading-6 text-muted-foreground">
        历史凭证已按保留策略销毁，平台仅保留交付类型、提交时间和销毁时间等审计事实。
        <div class="mt-1 text-xs">销毁时间：{{ formatOrderDateTime(credential.destroyedAt) }}</div>
      </div>
    </div>

    <div v-else class="space-y-3 p-4 text-sm sm:p-5">
      <div v-if="credential.apiBaseUrl" class="rounded-md border border-border p-3">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <span class="text-muted-foreground">API Base URL</span>
          <span class="flex shrink-0 gap-1.5">
            <Button as-child size="icon" variant="outline" title="打开 API Base URL" aria-label="打开 API Base URL">
              <a :href="credential.apiBaseUrl" target="_blank" rel="noopener noreferrer"><ExternalLink class="h-4 w-4" /></a>
            </Button>
            <Button size="icon" variant="outline" title="复制 API Base URL" aria-label="复制 API Base URL" @click="copyValue(credential.apiBaseUrl, 'API Base URL')">
              <Copy class="h-4 w-4" />
            </Button>
          </span>
        </div>
        <div class="mt-2 break-all font-mono text-xs leading-5">{{ credential.apiBaseUrl }}</div>
      </div>

      <div v-if="credential.apiKey" class="rounded-md border border-border p-3">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <span class="text-muted-foreground">API Key</span>
          <span class="flex shrink-0 gap-1.5">
            <Button size="icon" variant="outline" :title="apiKeyVisible ? '隐藏 API Key' : '显示 API Key'" :aria-label="apiKeyVisible ? '隐藏 API Key' : '显示 API Key'" @click="apiKeyVisible = !apiKeyVisible">
              <EyeOff v-if="apiKeyVisible" class="h-4 w-4" />
              <Eye v-else class="h-4 w-4" />
            </Button>
            <Button size="icon" variant="outline" title="复制 API Key" aria-label="复制 API Key" @click="copyValue(credential.apiKey, 'API Key')">
              <Copy class="h-4 w-4" />
            </Button>
          </span>
        </div>
        <div class="mt-2 break-all font-mono text-xs leading-5">{{ apiKeyVisible ? credential.apiKey : maskCredential(credential.apiKey) }}</div>
      </div>

      <div v-if="credential.panelLoginUrl" class="rounded-md border border-border p-3">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <span class="text-muted-foreground">登录地址</span>
          <span class="flex shrink-0 gap-1.5">
            <Button as-child size="icon" variant="outline" title="打开登录地址" aria-label="打开登录地址">
              <a :href="credential.panelLoginUrl" target="_blank" rel="noopener noreferrer"><ExternalLink class="h-4 w-4" /></a>
            </Button>
            <Button size="icon" variant="outline" title="复制登录地址" aria-label="复制登录地址" @click="copyValue(credential.panelLoginUrl, '登录地址')">
              <Copy class="h-4 w-4" />
            </Button>
          </span>
        </div>
        <div class="mt-2 break-all font-mono text-xs leading-5">{{ credential.panelLoginUrl }}</div>
      </div>

      <div v-if="credential.username" class="rounded-md border border-border p-3">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <span class="text-muted-foreground">用户名</span>
          <Button size="icon" variant="outline" title="复制用户名" aria-label="复制用户名" @click="copyValue(credential.username, '用户名')">
            <Copy class="h-4 w-4" />
          </Button>
        </div>
        <div class="mt-2 break-all font-mono text-xs leading-5">{{ credential.username }}</div>
      </div>

      <div v-if="credential.password" class="rounded-md border border-border p-3">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <span class="text-muted-foreground">初始密码</span>
          <span class="flex shrink-0 gap-1.5">
            <Button size="icon" variant="outline" :title="passwordVisible ? '隐藏初始密码' : '显示初始密码'" :aria-label="passwordVisible ? '隐藏初始密码' : '显示初始密码'" @click="passwordVisible = !passwordVisible">
              <EyeOff v-if="passwordVisible" class="h-4 w-4" />
              <Eye v-else class="h-4 w-4" />
            </Button>
            <Button size="icon" variant="outline" title="复制初始密码" aria-label="复制初始密码" @click="copyValue(credential.password, '初始密码')">
              <Copy class="h-4 w-4" />
            </Button>
          </span>
        </div>
        <div class="mt-2 break-all font-mono text-xs leading-5">{{ passwordVisible ? credential.password : maskCredential(credential.password) }}</div>
      </div>

      <div v-if="credential.instructions" class="whitespace-pre-line break-words rounded-md border border-border bg-muted/40 p-3 leading-6">
        <div class="mb-1 text-xs text-muted-foreground">使用说明</div>
        {{ credential.instructions }}
      </div>
    </div>
  </Card>
</template>
