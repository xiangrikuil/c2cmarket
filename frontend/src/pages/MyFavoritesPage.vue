<script setup lang="ts">
import { computed, ref } from 'vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import PageTitle from '@/components/market/PageTitle.vue'
import EmptyState from '@/components/market/EmptyState.vue'
import SkeletonTable from '@/components/market/SkeletonTable.vue'
import StatusTabs from '@/components/market/StatusTabs.vue'
import { useFavorites, useToggleFavoriteMutation } from '@/queries/useMarketQueries'
import type { FavoriteListItem } from '@/lib/api'
import { toast } from 'vue-sonner'

const { data, isLoading } = useFavorites()
const toggleFavoriteMutation = useToggleFavoriteMutation()
const activeCategory = ref('全部')
const rows = computed(() => (data.value ?? []).filter(item => activeCategory.value === '全部'
  || (activeCategory.value === '拼车' && item.targetType === 'carpool')
  || (activeCategory.value === 'API 服务' && item.targetType === 'api-service')))

function isAvailable(item: FavoriteListItem) {
  return !/已下架|暂停|离线|不可用|已关闭/.test(item.status)
}

function removeFavorite(item: FavoriteListItem) {
  toggleFavoriteMutation.mutate({ targetType: item.targetType, targetId: item.targetId }, {
    onSuccess: () => toast.success('已取消收藏。'),
    onError: error => toast.error(error instanceof Error ? error.message : '操作失败'),
  })
}
</script>

<template>
  <div class="space-y-5">
    <PageTitle title="我的收藏" description="统一展示已收藏的车源和 API 服务，并同步当前公开状态。" />
    <StatusTabs v-model="activeCategory" :items="['全部', '拼车', 'API 服务', '官网套餐']" />

    <SkeletonTable v-if="isLoading" :rows="4" :columns="2" />
    <EmptyState v-else-if="rows.length === 0" title="当前分类暂无收藏" description="可在车源详情或 API 服务详情中点击收藏。" />

    <div v-else class="grid gap-4 md:grid-cols-2">
      <Card v-for="item in rows" :key="item.id" class="p-5">
        <div class="flex items-start justify-between gap-4">
          <div class="min-w-0">
            <Badge variant="secondary">{{ item.targetType === 'carpool' ? '车源' : 'API 服务' }}</Badge>
            <RouterLink :to="item.to" class="mt-3 block text-lg font-semibold hover:underline">{{ item.title }}</RouterLink>
            <p class="mt-1 text-sm text-muted-foreground">{{ item.subtitle }}</p>
          </div>
          <Badge :variant="isAvailable(item) ? 'default' : 'secondary'">{{ isAvailable(item) ? item.status : `当前不可用 · ${item.status}` }}</Badge>
        </div>
        <div class="mt-5 flex flex-wrap justify-end gap-2">
          <RouterLink v-if="isAvailable(item)" :to="item.to"><Button variant="outline">查看详情</Button></RouterLink>
          <Button variant="outline" :disabled="toggleFavoriteMutation.isPending.value" @click="removeFavorite(item)">取消收藏</Button>
        </div>
      </Card>
    </div>
  </div>
</template>
