<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Copy, Eye, FilePenLine, Plus, Send, XCircle } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import AnnouncementDetailContent from '@/components/announcements/AnnouncementDetailContent.vue'
import PageTitle from '@/components/market/PageTitle.vue'
import SoftTable from '@/components/market/SoftTable.vue'
import StatusTabs from '@/components/market/StatusTabs.vue'
import TablePagination from '@/components/market/TablePagination.vue'
import CompactStats from '@/components/market/CompactStats.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Textarea } from '@/components/ui/textarea'
import { usePagination } from '@/composables/usePagination'
import {
  announcementAuditActionLabels,
  announcementCategoryLabels,
  announcementChannelLabels,
  announcementLevelLabels,
  announcementStatusLabels,
  formatAnnouncementDateTime,
  getAnnouncementDisplayStatus,
} from '@/lib/announcementUtils'
import {
  useAdminAnnouncements,
  useAnnouncementAuditLogs,
  useDuplicateAnnouncement,
  useOfflineAnnouncement,
  usePublishAnnouncement,
} from '@/queries/useAnnouncementQueries'
import type { Announcement, AnnouncementStatus } from '@/types/announcement'

type StatusFilter = '全部' | '草稿' | '待发布' | '发布中' | '已下线' | '已结束'

const router = useRouter()
const { data: announcements, isLoading } = useAdminAnnouncements()
const { data: auditLogs } = useAnnouncementAuditLogs()
const publishMutation = usePublishAnnouncement()
const offlineMutation = useOfflineAnnouncement()
const duplicateMutation = useDuplicateAnnouncement()
const activeStatus = ref<StatusFilter>('全部')
const previewId = ref('')
const offlineTargetId = ref('')
const offlineReason = ref('')
const offlineConfirmed = ref(false)
const statusFilters: StatusFilter[] = ['全部', '草稿', '待发布', '发布中', '已下线', '已结束']

const rows = computed(() => announcements.value ?? [])
const statusCounts = computed(() => rows.value.reduce<Record<AnnouncementStatus, number>>((counts, item) => {
  const status = getAnnouncementDisplayStatus(item)
  counts[status] += 1
  return counts
}, {
  draft: 0,
  scheduled: 0,
  published: 0,
  offline: 0,
  expired: 0,
  archived: 0,
}))

const visibleRows = computed(() => {
  if (activeStatus.value === '全部') return rows.value
  return rows.value.filter(item => statusFilterLabel(getAnnouncementDisplayStatus(item)) === activeStatus.value)
})
const pagination = usePagination(visibleRows, 10)
const previewAnnouncement = computed(() => rows.value.find(item => item.id === previewId.value) ?? rows.value[0] ?? null)
const offlineTarget = computed(() => rows.value.find(item => item.id === offlineTargetId.value) ?? null)
const recentAuditLogs = computed(() => (auditLogs.value ?? []).slice(0, 8))

function statusFilterLabel(status: AnnouncementStatus): StatusFilter {
  if (status === 'draft') return '草稿'
  if (status === 'scheduled') return '待发布'
  if (status === 'published') return '发布中'
  if (status === 'offline') return '已下线'
  if (status === 'expired') return '已结束'
  return '全部'
}

function badgeVariant(status: AnnouncementStatus) {
  if (status === 'published') return 'default'
  if (status === 'offline') return 'destructive'
  return 'secondary'
}

function canPublish(item: Announcement) {
  return ['draft', 'offline', 'expired'].includes(getAnnouncementDisplayStatus(item))
}

function canOffline(item: Announcement) {
  return ['published', 'scheduled'].includes(getAnnouncementDisplayStatus(item))
}

function channelsText(item: Announcement) {
  return item.channels.map(channel => announcementChannelLabels[channel]).join('、')
}

function editAnnouncement(item: Announcement) {
  router.push(`/admin/announcements/${item.id}/edit`)
}

async function publishAnnouncement(item: Announcement) {
  if (!canPublish(item)) {
    toast.warning('当前公告不能重复发布，请先编辑发布时间或状态。')
    return
  }

  try {
    const result = await publishMutation.mutateAsync(item.id)
    toast.success(getAnnouncementDisplayStatus(result) === 'scheduled' ? '公告已设置为待发布。' : '公告已发布。')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '公告发布失败')
  }
}

function startOffline(item: Announcement) {
  offlineTargetId.value = item.id
  offlineReason.value = ''
  offlineConfirmed.value = false
}

async function confirmOffline() {
  if (!offlineTarget.value) return
  if (!offlineReason.value.trim()) {
    toast.warning('请填写下线原因。')
    return
  }
  if (!offlineConfirmed.value) {
    toast.warning('请先勾选二次确认。')
    return
  }

  try {
    await offlineMutation.mutateAsync({ id: offlineTarget.value.id, reason: offlineReason.value.trim() })
    toast.success('公告已下线，原因已写入审计记录。')
    offlineTargetId.value = ''
    offlineReason.value = ''
    offlineConfirmed.value = false
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '公告下线失败')
  }
}

async function duplicateAnnouncement(item: Announcement) {
  try {
    const duplicated = await duplicateMutation.mutateAsync(item.id)
    toast.success('公告已复制为新草稿。')
    router.push(`/admin/announcements/${duplicated.id}/edit`)
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '公告复制失败')
  }
}
</script>

<template>
  <div class="space-y-5">
    <PageTitle
      title="公告管理"
      description="管理平台公告的草稿、发布、下线、复制和审计记录；公告与业务通知保持独立。"
      action-text="新建公告"
      action-to="/admin/announcements/new"
    />

    <CompactStats :items="[{ label: '草稿', value: statusCounts.draft, hint: '尚未发布' }, { label: '待发布', value: statusCounts.scheduled, hint: '发布时间在未来' }, { label: '发布中', value: statusCounts.published, hint: '用户端可见' }, { label: '已下线', value: statusCounts.offline, hint: '仅管理端可见' }, { label: '已结束', value: statusCounts.expired, hint: '公告历史可见' }]" :loading="isLoading" />

    <Card v-if="offlineTarget" class="border-destructive/30 p-5">
      <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <Badge variant="destructive">下线确认</Badge>
            <span class="font-semibold">{{ offlineTarget.title }}</span>
          </div>
          <p class="mt-2 text-sm text-muted-foreground">下线会立刻让用户端列表、详情和首页公告条不可见，并写入公告审计记录。</p>
        </div>
        <Button variant="ghost" size="sm" @click="offlineTargetId = ''">取消</Button>
      </div>
      <div class="mt-4 grid gap-3 lg:grid-cols-[1fr_auto] lg:items-end">
        <label class="space-y-2">
          <span class="text-sm font-medium">下线原因</span>
          <Textarea v-model="offlineReason" class="min-h-24" placeholder="填写下线原因，例如：内容过期、规则调整或需要重新审核。" />
        </label>
        <div class="space-y-3">
          <label class="flex items-start gap-2 rounded-md border border-border bg-muted/30 p-3 text-xs leading-5 text-muted-foreground">
            <input v-model="offlineConfirmed" type="checkbox" class="mt-1 h-4 w-4 accent-primary" />
            <span>二次确认：我已核对公告状态和影响范围，确认下线并保留审计记录。</span>
          </label>
          <Button class="w-full" variant="destructive" :disabled="offlineMutation.isPending.value" @click="confirmOffline">
            确认下线
          </Button>
        </div>
      </div>
    </Card>

    <StatusTabs v-model="activeStatus" :items="statusFilters" />

    <div v-if="isLoading" class="rounded-md border border-border p-8 text-center text-sm text-muted-foreground">
      公告加载中...
    </div>

    <div v-else class="space-y-5">
      <div class="space-y-3 lg:hidden">
        <Card v-for="item in pagination.paginatedRows.value" :key="item.id" class="p-4">
          <div class="flex flex-wrap items-center gap-2">
            <Badge :variant="badgeVariant(getAnnouncementDisplayStatus(item))">{{ announcementStatusLabels[getAnnouncementDisplayStatus(item)] }}</Badge>
            <Badge variant="outline">{{ announcementCategoryLabels[item.category] }}</Badge>
            <Badge :variant="item.level === 'important' ? 'default' : 'secondary'">{{ announcementLevelLabels[item.level] }}</Badge>
            <Badge v-if="item.isPinned" variant="secondary">置顶</Badge>
          </div>
          <h2 class="mt-3 font-semibold">{{ item.title }}</h2>
          <p class="mt-1 text-sm leading-6 text-muted-foreground">{{ item.summary }}</p>
          <div class="mt-3 grid gap-2 text-xs text-muted-foreground">
            <span>展示位置：{{ channelsText(item) }}</span>
            <span>面向用户：全部用户</span>
            <span>发布时间：{{ formatAnnouncementDateTime(item.publishAt) }}</span>
            <span>结束时间：{{ formatAnnouncementDateTime(item.expireAt) }}</span>
          </div>
          <div class="mt-4 flex flex-wrap gap-2">
            <Button size="sm" variant="outline" @click="previewId = item.id"><Eye class="h-4 w-4" />预览</Button>
            <Button size="sm" variant="outline" @click="editAnnouncement(item)"><FilePenLine class="h-4 w-4" />编辑</Button>
            <Button size="sm" :disabled="!canPublish(item) || publishMutation.isPending.value" @click="publishAnnouncement(item)"><Send class="h-4 w-4" />发布</Button>
            <Button size="sm" variant="outline" :disabled="!canOffline(item)" @click="startOffline(item)"><XCircle class="h-4 w-4" />下线</Button>
            <Button size="sm" variant="outline" :disabled="duplicateMutation.isPending.value" @click="duplicateAnnouncement(item)"><Copy class="h-4 w-4" />复制</Button>
          </div>
        </Card>
      </div>

      <div class="hidden lg:block">
        <SoftTable :columns="['公告', '分类 / 级别', '展示位置', '面向用户', '时间', '状态', '操作']">
          <tr v-for="item in pagination.paginatedRows.value" :key="item.id">
            <td class="max-w-[280px]">
              <div class="font-semibold">{{ item.title }}</div>
              <div class="mt-1 line-clamp-2 text-xs leading-5 text-muted-foreground">{{ item.summary }}</div>
            </td>
            <td>
              <div class="flex flex-wrap gap-1">
                <Badge variant="outline">{{ announcementCategoryLabels[item.category] }}</Badge>
                <Badge :variant="item.level === 'important' ? 'default' : 'secondary'">{{ announcementLevelLabels[item.level] }}</Badge>
                <Badge v-if="item.isPinned" variant="secondary">置顶</Badge>
              </div>
            </td>
            <td class="text-sm text-muted-foreground">{{ channelsText(item) }}</td>
            <td class="text-sm text-muted-foreground">全部用户</td>
            <td class="text-xs leading-5 text-muted-foreground">
              <div>发布：{{ formatAnnouncementDateTime(item.publishAt) }}</div>
              <div>结束：{{ formatAnnouncementDateTime(item.expireAt) }}</div>
            </td>
            <td><Badge :variant="badgeVariant(getAnnouncementDisplayStatus(item))">{{ announcementStatusLabels[getAnnouncementDisplayStatus(item)] }}</Badge></td>
            <td>
              <div class="flex flex-wrap gap-1.5">
                <Button size="sm" variant="outline" @click="previewId = item.id"><Eye class="h-4 w-4" />预览</Button>
                <Button size="sm" variant="outline" @click="editAnnouncement(item)"><FilePenLine class="h-4 w-4" />编辑</Button>
                <Button size="sm" :disabled="!canPublish(item) || publishMutation.isPending.value" @click="publishAnnouncement(item)"><Send class="h-4 w-4" />发布</Button>
                <Button size="sm" variant="outline" :disabled="!canOffline(item)" @click="startOffline(item)"><XCircle class="h-4 w-4" />下线</Button>
                <Button size="sm" variant="outline" :disabled="duplicateMutation.isPending.value" @click="duplicateAnnouncement(item)"><Copy class="h-4 w-4" />复制</Button>
              </div>
            </td>
          </tr>
          <tr v-if="visibleRows.length === 0">
            <td colspan="7" class="py-10 text-center text-sm text-muted-foreground">当前筛选下暂无公告。</td>
          </tr>
          <template #footer>
            <TablePagination
              v-model:page="pagination.page.value"
              :page-count="pagination.pageCount.value"
              :total="pagination.total.value"
              :start-item="pagination.startItem.value"
              :end-item="pagination.endItem.value"
            />
          </template>
        </SoftTable>
      </div>

      <div class="lg:hidden">
        <TablePagination
          v-model:page="pagination.page.value"
          :page-count="pagination.pageCount.value"
          :total="pagination.total.value"
          :start-item="pagination.startItem.value"
          :end-item="pagination.endItem.value"
        />
      </div>
    </div>

    <div class="grid gap-5 xl:grid-cols-[minmax(0,1.1fr)_minmax(320px,0.9fr)]">
      <Card class="p-5">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 class="font-semibold">公告预览</h2>
            <p class="mt-1 text-sm text-muted-foreground">管理预览复用用户端 Markdown 渲染与清洗逻辑。</p>
          </div>
          <RouterLink v-if="previewAnnouncement" :to="`/admin/announcements/${previewAnnouncement.id}/edit`">
            <Button size="sm" variant="outline">编辑当前预览</Button>
          </RouterLink>
        </div>
        <div v-if="previewAnnouncement" class="mt-5 space-y-4">
          <div class="flex flex-wrap items-center gap-2">
            <Badge :variant="badgeVariant(getAnnouncementDisplayStatus(previewAnnouncement))">{{ announcementStatusLabels[getAnnouncementDisplayStatus(previewAnnouncement)] }}</Badge>
            <Badge variant="outline">{{ announcementCategoryLabels[previewAnnouncement.category] }}</Badge>
            <Badge :variant="previewAnnouncement.level === 'important' ? 'default' : 'secondary'">{{ announcementLevelLabels[previewAnnouncement.level] }}</Badge>
          </div>
          <div>
            <h3 class="text-xl font-semibold">{{ previewAnnouncement.title }}</h3>
            <p class="mt-2 text-sm leading-6 text-muted-foreground">{{ previewAnnouncement.summary }}</p>
          </div>
          <AnnouncementDetailContent :content-markdown="previewAnnouncement.contentMarkdown" />
        </div>
        <div v-else class="mt-5 rounded-md border border-dashed border-border p-6 text-center text-sm text-muted-foreground">
          暂无可预览公告。
        </div>
      </Card>

      <Card class="p-5">
        <h2 class="font-semibold">近期公告审计</h2>
        <div class="mt-4 space-y-3">
          <div v-for="log in recentAuditLogs" :key="log.id" class="rounded-md border border-border bg-muted/30 p-3">
            <div class="flex flex-wrap items-center gap-2 text-sm">
              <Badge variant="secondary">{{ announcementAuditActionLabels[log.action] }}</Badge>
              <span class="font-medium">{{ log.announcementTitle }}</span>
            </div>
            <div class="mt-2 text-xs leading-5 text-muted-foreground">
              <div>操作人：{{ log.operatorName }}</div>
              <div>时间：{{ formatAnnouncementDateTime(log.createdAt) }}</div>
              <div v-if="log.reason">原因：{{ log.reason }}</div>
            </div>
          </div>
          <div v-if="recentAuditLogs.length === 0" class="rounded-md border border-dashed border-border p-6 text-center text-sm text-muted-foreground">
            暂无公告审计记录。
          </div>
        </div>
      </Card>
    </div>

    <RouterLink to="/admin/announcements/new" class="fixed bottom-5 right-5 lg:hidden">
      <Button size="icon" aria-label="新建公告"><Plus class="h-4 w-4" /></Button>
    </RouterLink>
  </div>
</template>
