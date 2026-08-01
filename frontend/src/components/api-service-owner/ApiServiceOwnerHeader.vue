<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import {
  ArrowLeft,
  ChevronDown,
  CirclePause,
  CirclePlay,
  Eye,
  ListOrdered,
  PackageSearch,
  Rocket,
} from 'lucide-vue-next'
import ShortId from '@/components/market/ShortId.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  getApiMerchantDisplayName,
  getApiMerchantVisibilityLabel,
  type ApiService,
} from '@/lib/api'
import { getApiServiceOwnerStatus } from './apiServiceOwnerPresentation'

const props = defineProps<{
  service: ApiService
  iconSrc?: string | null
  actionPending?: boolean
}>()

const emit = defineEmits<{
  publish: []
  pause: []
  resume: []
}>()

const status = computed(() => getApiServiceOwnerStatus(props.service))

const statusClass = computed(() => ({
  success: 'border-success/20 bg-success/10 text-success',
  waiting: 'border-primary/20 bg-primary/10 text-primary',
  warning: 'border-warning/25 bg-warning/10 text-warning',
  neutral: 'border-border bg-muted text-muted-foreground',
}[status.value.tone]))
</script>

<template>
  <div class="space-y-3">
    <RouterLink
      to="/my/api-services"
      class="inline-flex items-center gap-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground"
    >
      <ArrowLeft class="h-4 w-4" />
      返回我的 API 服务
    </RouterLink>

    <Card class="rounded-lg p-5 shadow-xs md:p-6">
      <div class="flex flex-col gap-5 xl:flex-row xl:items-center xl:justify-between">
        <div class="flex min-w-0 items-start gap-4">
          <span class="grid h-16 w-16 shrink-0 place-items-center rounded-lg border border-border bg-background shadow-xs">
            <img v-if="iconSrc" :src="iconSrc" alt="" class="h-10 w-10 object-contain" />
            <PackageSearch v-else class="h-7 w-7 text-muted-foreground" />
          </span>

          <div class="min-w-0 pt-0.5">
            <div class="flex flex-wrap items-center gap-2.5">
              <h1 class="min-w-0 text-2xl font-semibold break-words md:text-[28px]">{{ service.title }}</h1>
              <Badge variant="outline" :class="statusClass">
                <span class="h-1.5 w-1.5 rounded-full bg-current" />
                {{ status.label }}
              </Badge>
            </div>
            <div class="mt-2 flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-muted-foreground">
              <span>{{ getApiMerchantDisplayName(service) }}</span>
              <span aria-hidden="true">·</span>
              <span>{{ getApiMerchantVisibilityLabel(service) }}</span>
              <span aria-hidden="true">·</span>
              <span>服务编号</span>
              <ShortId :value="service.id" prefix="API-SVC" copyable />
            </div>
          </div>
        </div>

        <div class="flex flex-wrap items-center gap-2">
          <Button v-if="service.publiclyOrderable" as-child variant="outline">
            <RouterLink :to="`/api-market/${service.id}?preview=owner`">
              <Eye />
              买家视角预览
            </RouterLink>
          </Button>
          <Button v-else variant="outline" disabled>
            <Eye />
            当前不可预览
          </Button>

          <Button as-child variant="outline">
            <RouterLink to="/merchant/api-orders">
              <ListOrdered />
              查看 API 销售订单
            </RouterLink>
          </Button>

          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <Button variant="outline" :disabled="actionPending">
                更多
                <ChevronDown />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" class="w-44">
              <DropdownMenuItem v-if="service.state === 'offline'" @click="emit('publish')">
                <Rocket />
                上线服务
              </DropdownMenuItem>
              <DropdownMenuItem v-if="service.online" variant="destructive" @click="emit('pause')">
                <CirclePause />
                暂停接单
              </DropdownMenuItem>
              <DropdownMenuItem v-if="service.state === 'paused'" @click="emit('resume')">
                <CirclePlay />
                恢复接单
              </DropdownMenuItem>
              <DropdownMenuItem v-if="service.state === 'reviewing'" disabled>
                <Rocket />
                审核完成后可上线
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>
    </Card>
  </div>
</template>
