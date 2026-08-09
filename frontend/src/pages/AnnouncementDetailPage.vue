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
const contentUpdatedAt = computed(() => announcement.value ? formatTime(announcement.value.contentUpdatedAt) : '')
const wasUpdatedAfterPublish = computed(() => announcement.value
  ? new Date(announcement.value.contentUpdatedAt).getTime() > new Date(announcement.value.publishAt).getTime()
  : false)
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
  <div class="announcement-reference-page space-y-5">
    <div class="announcement-reference-heading rounded-xl border px-5 py-4">
      <PageTitle title="公告详情" description="平台规则、功能更新与业务公告。" />
    </div>

    <Card v-if="isLoading" class="mx-auto max-w-4xl p-6 text-sm text-muted-foreground">公告加载中...</Card>

    <Card v-else-if="!announcement" class="mx-auto max-w-4xl p-8 text-center">
      <h2 class="text-xl font-semibold">公告不存在或当前不可见</h2>
      <p class="mt-2 text-sm text-muted-foreground">该公告可能仍是草稿、待发布、已下线，或链接输入有误。</p>
      <div class="mt-5 flex justify-center">
        <Button variant="outline" @click="router.push('/my/notifications?tab=announcements')">
          <ArrowLeft class="h-4 w-4" />
          返回公告列表
        </Button>
      </div>
    </Card>

    <template v-else>
      <Card class="announcement-reference-article mx-auto max-w-4xl overflow-hidden p-0">
        <article>
          <header class="announcement-reference-article-header p-5 md:p-7">
            <div class="flex flex-wrap items-center gap-2">
              <Badge variant="outline">{{ announcementCategoryLabels[announcement.category] }}</Badge>
              <Badge :variant="announcement.level === 'important' ? 'default' : 'secondary'">{{ announcementLevelLabels[announcement.level] }}</Badge>
              <Badge v-if="announcement.isPinned" variant="secondary">置顶</Badge>
            </div>
            <h1 class="mt-4 text-2xl font-semibold tracking-tight md:text-3xl">{{ announcement.title }}</h1>
            <p class="mt-2 max-w-3xl text-sm leading-6 text-muted-foreground">{{ announcement.summary }}</p>
            <div class="mt-4 flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
              <span>发布于 {{ publishedAt }}</span>
              <span v-if="wasUpdatedAfterPublish">更新于 {{ contentUpdatedAt }}</span>
            </div>
          </header>

          <div class="border-t border-border px-5 py-6 md:px-7 md:py-8">
            <AnnouncementDetailContent :content-markdown="announcement.contentMarkdown" />
            <div v-if="announcement.ctaLabel && announcement.ctaUrl" class="mt-7">
              <Button v-if="ctaIsExternal" as-child>
                <a :href="announcement.ctaUrl" target="_blank" rel="noopener noreferrer">
                  {{ announcement.ctaLabel }}
                  <ExternalLink class="h-4 w-4" />
                </a>
              </Button>
              <Button v-else as-child>
                <RouterLink :to="announcement.ctaUrl">
                  {{ announcement.ctaLabel }}
                  <ExternalLink class="h-4 w-4" />
                </RouterLink>
              </Button>
            </div>
          </div>
        </article>
      </Card>

      <div class="mx-auto flex max-w-4xl">
        <Button variant="outline" @click="router.push('/my/notifications?tab=announcements')">
          <ArrowLeft class="h-4 w-4" />
          返回公告列表
        </Button>
      </div>
    </template>
  </div>
</template>
