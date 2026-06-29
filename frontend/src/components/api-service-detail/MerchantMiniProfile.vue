<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { canOpenApiMerchantProfile, getApiMerchantAvatarText, getApiMerchantDisplayName, getApiMerchantProfileUrl, getApiMerchantVisibilityLabel, type ApiService } from '@/lib/api'
import { merchantIdentityLabel } from './utils'

const props = defineProps<{
  service: ApiService
}>()

const merchantUrl = computed(() => getApiMerchantProfileUrl(props.service))
</script>

<template>
  <div class="flex flex-col gap-3 rounded-lg border border-border bg-background p-3 sm:flex-row sm:items-center sm:justify-between">
    <component :is="merchantUrl ? RouterLink : 'div'" :to="merchantUrl || undefined" class="flex min-w-0 items-center gap-3">
      <span class="grid h-10 w-10 shrink-0 place-items-center rounded-full bg-primary text-sm font-semibold text-primary-foreground">
        {{ getApiMerchantAvatarText(service) }}
      </span>
      <span class="min-w-0">
        <span class="flex flex-wrap items-center gap-2">
          <span class="font-semibold">{{ getApiMerchantDisplayName(service) }}</span>
          <Badge variant="identity">{{ merchantIdentityLabel(service.merchantType) }}</Badge>
          <Badge variant="verified">已绑定 linux.do</Badge>
          <Badge variant="trust">信任等级{{ service.trustLevel }}</Badge>
          <Badge v-if="!canOpenApiMerchantProfile(service)" variant="secondary">{{ getApiMerchantVisibilityLabel(service) }}</Badge>
        </span>
        <span class="mt-1 block text-xs text-muted-foreground">
          近30天完成 {{ service.completed30d }} · 响应中位 {{ service.responseMedianMinutes }} 分钟 · 商户责任取消 0
        </span>
      </span>
    </component>
    <RouterLink v-if="merchantUrl" :to="merchantUrl" class="shrink-0">
      <Button variant="outline" size="sm">公开主页</Button>
    </RouterLink>
  </div>
</template>
