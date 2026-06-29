<script setup lang="ts">
import { computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, ExternalLink } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import AnnouncementDetailContent from '@/components/announcements/AnnouncementDetailContent.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import PageTitle from '@/components/market/PageTitle.vue'
import { announcementCategoryLabels, announcementLevelLabels } from '@/lib/announcementUtils'
import { useAnnouncementDetail, useMarkAnnouncementRead } from '@/queries/useAnnouncementQueries'

const route = useRoute()
const router = useRouter()
const slug = computed(() => String(route.params.slug ?? ''))
const { data: announcement, isLoading } = useAnnouncementDetail(slug)
const markReadMutation = useMarkAnnouncementRead()

const publishedAt = computed(() => announcement.value ? formatTime(announcement.value.publishAt) : '')
const updatedAt = computed(() => announcement.value ? formatTime(announcement.value.updatedAt) : '')
const ctaIsExternal = computed(() => Boolean(announcement.value?.ctaUrl?.startsWith('https://')))

watch(announcement, item => {
  if (!item) return
  markReadMutation.mutate(item.id, {
    onError: error => toast.error(error instanceof Error ? error.message : '公告已读状态更新失败'),
  })
}, { immediate: true })

function formatTime(value: string) {
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(new Date(value))
}
</script>

<template>
  <div class="space-y-5">
    <PageTitle title="公告详情" description="平台公告与业务通知独立展示。" />

    <Card v-if="isLoading" class="p-6 text-sm text-muted-foreground">公告加载中...</Card>

    <Card v-else-if="!announcement" class="p-8 text-center">
      <h2 class="text-xl font-semibold">公告不存在或当前不可见</h2>
      <p class="mt-2 text-sm text-muted-foreground">该公告可能仍是草稿、待发布、已下线，或链接输入有误。</p>
      <div class="mt-5 flex justify-center">
        <Button variant="outline" @click="router.push('/my/notifications?tab=announcements')">
          <ArrowLeft class="h-4 w-4" />
          返回公告列表
        </Button>
      </div>
    </Card>

    <article v-else class="space-y-4">
      <Card class="p-5">
        <div class="flex flex-wrap items-center gap-2">
          <Badge variant="outline">{{ announcementCategoryLabels[announcement.category] }}</Badge>
          <Badge :variant="announcement.level === 'important' ? 'default' : 'secondary'">{{ announcementLevelLabels[announcement.level] }}</Badge>
          <Badge v-if="announcement.isPinned" variant="secondary">置顶</Badge>
        </div>
        <h1 class="mt-4 text-2xl font-semibold tracking-tight md:text-3xl">{{ announcement.title }}</h1>
        <p class="mt-2 max-w-3xl text-sm leading-6 text-muted-foreground">{{ announcement.summary }}</p>
        <div class="mt-4 flex flex-wrap gap-3 text-xs text-muted-foreground">
          <span>发布时间：{{ publishedAt }}</span>
          <span>更新时间：{{ updatedAt }}</span>
          <span>已读状态：已自动记录</span>
        </div>
      </Card>

      <Card class="p-5">
        <AnnouncementDetailContent :content-markdown="announcement.contentMarkdown" />
        <div v-if="announcement.ctaLabel && announcement.ctaUrl" class="mt-6">
          <a v-if="ctaIsExternal" :href="announcement.ctaUrl" target="_blank" rel="noopener noreferrer">
            <Button>
              {{ announcement.ctaLabel }}
              <ExternalLink class="h-4 w-4" />
            </Button>
          </a>
          <RouterLink v-else :to="announcement.ctaUrl">
            <Button>
              {{ announcement.ctaLabel }}
              <ExternalLink class="h-4 w-4" />
            </Button>
          </RouterLink>
        </div>
      </Card>

      <Button variant="outline" @click="router.push('/my/notifications?tab=announcements')">
        <ArrowLeft class="h-4 w-4" />
        返回公告列表
      </Button>
    </article>
  </div>
</template>
