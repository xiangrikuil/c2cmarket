<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { Heart } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { canOpenApiMerchantProfile, getApiMerchantAvatarText, getApiMerchantDisplayName, getApiMerchantProfileUrl, getApiMerchantVisibilityLabel, type ApiService } from '@/lib/api'
import { merchantIdentityLabel } from './utils'

const props = defineProps<{
  service: ApiService
  favorited: boolean
}>()

const emit = defineEmits<{
  toggleFavorite: []
}>()

const merchantUrl = computed(() => getApiMerchantProfileUrl(props.service))
const merchantDisplayName = computed(() => getApiMerchantDisplayName(props.service))
</script>

<template>
  <section class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
    <div class="min-w-0">
      <div class="mb-2 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
        <span>API 服务</span>
        <span>·</span>
        <Badge variant="verified">已审核</Badge>
      </div>
      <h1 class="text-2xl font-semibold leading-tight tracking-tight md:text-[28px]">{{ service.title }}</h1>
      <p class="mt-2 max-w-3xl text-sm leading-6 text-muted-foreground">
        清晰展示官网公开模型价格、商户倍率与实际计费价格。平台只保存意向和规则快照，不接触 API Key、token、面板账号或密码。
      </p>
    </div>

    <div class="flex items-center justify-between gap-3 rounded-xl border border-border bg-card px-4 py-3 lg:min-w-72">
      <component :is="merchantUrl ? RouterLink : 'div'" :to="merchantUrl || undefined" class="flex min-w-0 items-center gap-3">
        <span class="grid h-9 w-9 shrink-0 place-items-center rounded-full bg-primary text-sm font-semibold text-primary-foreground">
          {{ getApiMerchantAvatarText(service) }}
        </span>
        <span class="min-w-0">
          <span class="block truncate text-sm font-semibold">{{ merchantDisplayName }}</span>
          <span class="mt-1 flex flex-wrap items-center gap-1.5">
            <Badge variant="identity">{{ merchantIdentityLabel(service.merchantType) }}</Badge>
            <Badge variant="trust">信任等级{{ service.trustLevel }}</Badge>
            <Badge variant="verified">已绑定 linux.do</Badge>
            <Badge v-if="!canOpenApiMerchantProfile(service)" variant="secondary">{{ getApiMerchantVisibilityLabel(service) }}</Badge>
          </span>
          <span class="mt-1 flex items-center gap-1.5 text-xs text-muted-foreground">
            <span class="h-1.5 w-1.5 rounded-full" :class="service.publiclyOrderable ? 'bg-emerald-500' : 'bg-muted-foreground/50'" />
            {{ service.publiclyOrderable ? '可提交意向' : '暂不可接单' }} · 通常 {{ service.responseMedianMinutes }} 分钟内响应
          </span>
        </span>
      </component>
      <div class="flex shrink-0 items-center gap-2">
        <Button variant="outline" size="sm" class="h-8 gap-1 px-2.5" @click="emit('toggleFavorite')">
          <Heart class="h-3.5 w-3.5" :class="favorited ? 'fill-current' : ''" />
          {{ favorited ? '已收藏' : '收藏' }}
        </Button>
        <RouterLink v-if="merchantUrl" :to="merchantUrl">
          <Button variant="outline" size="sm" class="h-8 px-3">查看主页</Button>
        </RouterLink>
      </div>
    </div>
  </section>
</template>
