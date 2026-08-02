<script setup lang="ts">
import { ArrowLeft, Code2 } from 'lucide-vue-next'
import { RouterLink } from 'vue-router'
import { Badge } from '@/components/ui/badge'
import type { ApiService } from '@/lib/api'

defineProps<{
  service: ApiService
  iconSrc?: string | null
}>()
</script>

<template>
  <header class="api-service-detail-header min-w-0 px-1 pb-1">
    <RouterLink to="/api-market" class="inline-flex items-center gap-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground">
      <ArrowLeft class="h-4 w-4" />返回 API 市场
    </RouterLink>
    <div class="mt-3 flex min-w-0 items-start gap-3.5">
      <span class="api-service-detail-logo">
        <img v-if="iconSrc" :src="iconSrc" alt="" class="h-6 w-6 object-contain" />
        <Code2 v-else class="h-6 w-6" />
      </span>
      <div class="min-w-0 flex-1">
        <div class="flex flex-wrap items-center gap-2">
          <h1 class="min-w-0 text-2xl font-semibold leading-tight md:text-[26px]">{{ service.title }}</h1>
          <Badge :variant="service.publiclyOrderable ? 'verified' : 'secondary'">
            {{ service.publiclyOrderable ? '可创建订单' : '暂不可下单' }}
          </Badge>
          <Badge variant="secondary">{{ service.delivery }}</Badge>
        </div>
        <p class="mt-1.5 truncate text-sm text-muted-foreground" :title="service.models.join(' / ')">
          API 服务 · 支持 {{ service.models.length }} 个模型<span v-if="service.models.length"> · {{ service.models.slice(0, 3).join(' / ') }}</span>
        </p>
      </div>
    </div>
  </header>
</template>
