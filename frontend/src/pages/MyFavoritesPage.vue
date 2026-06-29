<script setup lang="ts">
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import PageTitle from '@/components/market/PageTitle.vue'
import { useFavorites, useToggleFavoriteMutation } from '@/queries/useMarketQueries'
import type { FavoriteListItem } from '@/lib/api'
import { toast } from 'vue-sonner'

const { data } = useFavorites()
const toggleFavoriteMutation = useToggleFavoriteMutation()

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

    <div v-if="(data ?? []).length === 0" class="rounded-xl border border-border bg-card p-8 text-center text-sm text-muted-foreground">
      暂无收藏。可在车源详情或 API 服务详情中点击收藏。
    </div>

    <div v-else class="grid gap-4 md:grid-cols-2">
      <Card v-for="item in data" :key="item.id" class="p-5">
        <div class="flex items-start justify-between gap-4">
          <div class="min-w-0">
            <Badge variant="secondary">{{ item.targetType === 'carpool' ? '车源' : 'API 服务' }}</Badge>
            <RouterLink :to="item.to" class="mt-3 block text-lg font-semibold hover:underline">{{ item.title }}</RouterLink>
            <p class="mt-1 text-sm text-muted-foreground">{{ item.subtitle }}</p>
          </div>
          <Badge>{{ item.status }}</Badge>
        </div>
        <div class="mt-5 flex flex-wrap justify-end gap-2">
          <RouterLink :to="item.to"><Button variant="outline">查看详情</Button></RouterLink>
          <Button variant="outline" :disabled="toggleFavoriteMutation.isPending.value" @click="removeFavorite(item)">取消收藏</Button>
        </div>
      </Card>
    </div>
  </div>
</template>
