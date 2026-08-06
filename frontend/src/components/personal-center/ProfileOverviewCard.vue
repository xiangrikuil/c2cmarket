<script setup lang="ts">
import { computed } from 'vue'
import { Eye, UserRound } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import type { PersonalCenterMetric } from '@/lib/personalCenterDashboard'
import type { UserProfile } from '@/lib/api'

const props = defineProps<{
  profile: UserProfile
  metrics: PersonalCenterMetric[]
}>()

const avatarText = computed(() => (props.profile.displayName || props.profile.username || '我').slice(0, 1).toUpperCase())
</script>

<template>
  <Card class="gap-4 border-border bg-card p-4 shadow-sm sm:p-5">
    <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
      <div class="flex min-w-0 items-start gap-3.5">
        <div class="grid h-14 w-14 shrink-0 place-items-center overflow-hidden rounded-full bg-primary text-lg font-semibold text-primary-foreground">
          <img v-if="profile.avatarUrl" :src="profile.avatarUrl" alt="当前头像" class="h-full w-full object-cover" />
          <span v-else>{{ avatarText }}</span>
        </div>
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <h2 class="text-xl font-semibold">{{ profile.displayName || profile.username }}</h2>
            <Badge :variant="profile.linuxDoBinding.bound ? 'verified' : 'secondary'">
              {{ profile.linuxDoBinding.bound ? 'Linux.do 已绑定' : 'Linux.do 未绑定' }}
            </Badge>
            <Badge variant="trust">
              {{ profile.linuxDoBinding.trustLevel === null ? '信任等级暂无数据' : `信任等级 ${profile.linuxDoBinding.trustLevel}` }}
            </Badge>
            <Badge v-for="badge in profile.badges" :key="badge.id" :variant="badge.type === 'system' ? 'default' : 'secondary'">
              {{ badge.label }}
            </Badge>
          </div>
          <p class="mt-1 text-sm text-muted-foreground">
            @{{ profile.username }}
            <template v-if="profile.linuxDoBinding.linuxDoUsername"> · linux.do @{{ profile.linuxDoBinding.linuxDoUsername }}</template>
          </p>
          <p v-if="profile.bio" class="mt-2 max-w-3xl text-sm leading-6 text-muted-foreground">{{ profile.bio }}</p>
        </div>
      </div>

      <div class="grid grid-cols-2 gap-2 sm:flex sm:flex-wrap">
        <Button as-child variant="outline" class="min-w-0">
          <RouterLink :to="`/u/${profile.username}`"><Eye class="h-4 w-4" />查看公开主页</RouterLink>
        </Button>
        <Button as-child class="min-w-0">
          <RouterLink to="/my/profile"><UserRound class="h-4 w-4" />编辑个人资料</RouterLink>
        </Button>
      </div>
    </div>

    <dl v-if="metrics.some(item => item.available || item.loading)" class="grid grid-cols-2 border-t border-border pt-3 lg:grid-cols-4">
      <div
        v-for="(metric, index) in metrics.filter(item => item.available || item.loading)"
        :key="metric.id"
        class="min-w-0 px-3 py-2 first:pl-0 even:border-l even:border-border lg:border-l lg:border-border lg:first:border-l-0"
        :class="index > 1 ? 'border-t border-border lg:border-t-0' : ''"
      >
        <dt class="text-xs text-muted-foreground">{{ metric.label }}</dt>
        <dd v-if="metric.loading" class="mt-2 h-5 w-12 animate-pulse rounded bg-muted" aria-label="正在加载" />
        <dd v-else class="mt-1 text-xl font-semibold text-foreground">{{ metric.value }}</dd>
        <small class="mt-1 block truncate text-xs text-muted-foreground">{{ metric.hint }}</small>
      </div>
    </dl>
  </Card>
</template>
