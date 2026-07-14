<script setup lang="ts">
import { Code2 } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import type { ApiService } from '@/lib/api'

defineProps<{
  service: ApiService
}>()
</script>

<template>
  <section class="api-service-detail-header min-w-0 rounded-xl border px-5 py-5 md:px-6">
    <div class="flex min-w-0 items-start gap-4">
      <span class="api-service-detail-logo"><Code2 class="h-7 w-7" /></span>
      <div class="min-w-0">
        <div class="mb-2 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
          <span>API 服务</span>
          <span>·</span>
          <Badge variant="verified">可创建订单</Badge>
          <Badge variant="secondary">{{ service.delivery }}</Badge>
        </div>
        <h1 class="text-2xl font-semibold leading-tight tracking-tight md:text-[30px]">{{ service.title }}</h1>
        <p class="mt-2 max-w-3xl text-sm leading-6 text-muted-foreground">比较美元额度售价、模型倍率、最低订单和商户履约记录；付款与实际接入按订单由双方确认。</p>
      </div>
    </div>
  </section>
</template>
