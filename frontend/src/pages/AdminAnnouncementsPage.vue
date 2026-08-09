<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { Copy, Ellipsis, Eye, FilePenLine, Search, Send, TriangleAlert, XCircle } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import AnnouncementDetailContent from '@/components/announcements/AnnouncementDetailContent.vue'
import EmptyState from '@/components/market/EmptyState.vue'
import PageTitle from '@/components/market/PageTitle.vue'
import SkeletonTable from '@/components/market/SkeletonTable.vue'
import SoftTable from '@/components/market/SoftTable.vue'
import StatusTabs from '@/components/market/StatusTabs.vue'
import CursorTablePagination from '@/components/market/CursorTablePagination.vue'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { useCursorPagination } from '@/composables/useCursorPagination'
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
  useAdminAnnouncementsPage,
  useAnnouncementAuditLogs,
  useDuplicateAnnouncement,
  useOfflineAnnouncement,
  usePublishAnnouncement,
} from '@/queries/useAnnouncementQueries'
import type { Announcement, AnnouncementStatus } from '@/types/announcement'

type StatusFilter = '工作中' | '草稿' | '待发布' | '发布中' | '历史' | '全部'

const router = useRouter()
const {
  data: announcements,
} = useAdminAnnouncements()
const {
  data: auditLogs,
  isLoading: auditLogsLoading,
} = useAnnouncementAuditLogs()
const publishMutation = usePublishAnnouncement()
const offlineMutation = useOfflineAnnouncement()
const duplicateMutation = useDuplicateAnnouncement()

const activeStatus = ref<StatusFilter>('工作中')
const keyword = ref('')
const previewId = ref('')
const previewOpen = ref(false)
const offlineTargetId = ref('')
const offlineReason = ref('')
const offlineConfirmed = ref(false)
const offlineOpen = ref(false)
const statusFilters: StatusFilter[] = ['工作中', '草稿', '待发布', '发布中', '历史', '全部']

const rows = computed(() => announcements.value ?? [])
const statusCounts = computed(() => rows.value.reduce<Record<AnnouncementStatus, number>>((counts, item) => {
  counts[getAnnouncementDisplayStatus(item)] += 1
  return counts
}, {
  draft: 0,
  scheduled: 0,
  published: 0,
  offline: 0,
  expired: 0,
  archived: 0,
}))
const statusFilterCounts = computed(() => ({
  工作中: statusCounts.value.draft + statusCounts.value.scheduled + statusCounts.value.published,
  草稿: statusCounts.value.draft,
  待发布: statusCounts.value.scheduled,
  发布中: statusCounts.value.published,
  历史: statusCounts.value.offline + statusCounts.value.expired,
  全部: rows.value.length,
}))
const statusGroup = computed(() => {
  if (activeStatus.value === '工作中') return 'working' as const
  if (activeStatus.value === '草稿') return 'draft' as const
  if (activeStatus.value === '待发布') return 'scheduled' as const
  if (activeStatus.value === '发布中') return 'published' as const
  if (activeStatus.value === '历史') return 'history' as const
  return 'all' as const
})
const pageFilters = computed(() => ({ q: keyword.value.trim() || undefined, statusGroup: statusGroup.value }))
const pagination = useCursorPagination([activeStatus, keyword], 10)
const pageRequest = computed(() => ({ limit: pagination.pageSize, cursor: pagination.cursor.value }))
const pageQuery = useAdminAnnouncementsPage(pageFilters, pageRequest)
const visibleRows = computed(() => pageQuery.data.value?.items ?? [])
const error = pageQuery.error
const isFetching = pageQuery.isFetching
const isLoading = computed(() => pageQuery.isLoading.value || pageQuery.isFetching.value)
const refetch = pageQuery.refetch
const previewAnnouncement = computed(() => rows.value.find(item => item.id === previewId.value) ?? null)
const offlineTarget = computed(() => rows.value.find(item => item.id === offlineTargetId.value) ?? null)
const previewAuditLogs = computed(() => (auditLogs.value ?? [])
  .filter(log => log.announcementId === previewId.value)
  .slice(0, 8))
const errorMessage = computed(() => error.value instanceof Error
  ? error.value.message
  : '公告列表读取失败，请稍后重试。')
const hasActiveFilter = computed(() => Boolean(keyword.value.trim()) || activeStatus.value !== '工作中')

watch(previewOpen, (open) => {
  if (!open) previewId.value = ''
})

watch(offlineOpen, (open) => {
  if (!open) resetOfflineForm()
})

function badgeVariant(status: AnnouncementStatus): 'verified' | 'capability' | 'secondary' | 'status' {
  if (status === 'published') return 'verified'
  if (status === 'scheduled') return 'capability'
  if (status === 'draft') return 'status'
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

function openPreview(item: Announcement) {
  previewId.value = item.id
  previewOpen.value = true
}

function editAnnouncement(item: Announcement) {
  previewOpen.value = false
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
  } catch (mutationError) {
    toast.error(mutationError instanceof Error ? mutationError.message : '公告发布失败')
  }
}

function startOffline(item: Announcement) {
  if (!canOffline(item)) {
    toast.warning('当前公告不能下线。')
    return
  }
  previewOpen.value = false
  offlineTargetId.value = item.id
  offlineReason.value = ''
  offlineConfirmed.value = false
  offlineOpen.value = true
}

function resetOfflineForm() {
  offlineTargetId.value = ''
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
    await offlineMutation.mutateAsync({
      id: offlineTarget.value.id,
      reason: offlineReason.value.trim(),
    })
    toast.success('公告已下线，原因已写入审计记录。')
    offlineOpen.value = false
  } catch (mutationError) {
    toast.error(mutationError instanceof Error ? mutationError.message : '公告下线失败')
  }
}

async function duplicateAnnouncement(item: Announcement) {
  try {
    const duplicated = await duplicateMutation.mutateAsync(item.id)
    toast.success('公告已复制为新草稿。')
    previewOpen.value = false
    router.push(`/admin/announcements/${duplicated.id}/edit`)
  } catch (mutationError) {
    toast.error(mutationError instanceof Error ? mutationError.message : '公告复制失败')
  }
}

function clearFilters() {
  keyword.value = ''
  activeStatus.value = '工作中'
}
</script>

<template>
  <div class="space-y-5">
    <PageTitle
      title="公告管理"
      description="管理公告草稿、发布计划与历史记录，预览和审计按当前公告展开。"
      action-text="新建公告"
      action-to="/admin/announcements/new"
    />

    <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
      <StatusTabs
        v-model="activeStatus"
        :items="statusFilters"
        :counts="isLoading || error ? undefined : statusFilterCounts"
        class="mb-0"
      />
      <div class="relative w-full lg:max-w-xs">
        <Search class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          v-model="keyword"
          class="pl-9"
          placeholder="搜索公告"
          aria-label="搜索公告"
          :disabled="isLoading || Boolean(error)"
        />
      </div>
    </div>

    <template v-if="isLoading">
      <SkeletonTable class="hidden lg:block" :rows="7" :columns="5" />
      <SkeletonTable class="lg:hidden" :rows="4" :columns="1" />
    </template>

    <Alert v-else-if="error" variant="destructive">
      <TriangleAlert class="h-4 w-4" />
      <AlertTitle>公告列表读取失败</AlertTitle>
      <AlertDescription class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <span>{{ errorMessage }}</span>
        <Button size="sm" variant="outline" :disabled="isFetching" @click="refetch()">
          {{ isFetching ? '正在重试...' : '重新读取' }}
        </Button>
      </AlertDescription>
    </Alert>

    <EmptyState
      v-else-if="visibleRows.length === 0"
      :title="keyword.trim() ? '没有匹配的公告' : '当前视图暂无公告'"
      :description="keyword.trim() ? '调整关键词或状态筛选后再试。' : '当前没有需要处理的公告，可以新建公告草稿。'"
    >
      <template #action>
        <Button v-if="hasActiveFilter" variant="outline" @click="clearFilters">重置筛选</Button>
        <Button v-else @click="router.push('/admin/announcements/new')">新建公告</Button>
      </template>
    </EmptyState>

    <template v-else>
      <div class="space-y-3 lg:hidden">
        <Card v-for="item in visibleRows" :key="item.id" class="p-4">
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <h2 class="font-semibold leading-6">{{ item.title }}</h2>
              <p class="mt-1 line-clamp-2 text-sm leading-6 text-muted-foreground">{{ item.summary }}</p>
            </div>
            <Badge :variant="badgeVariant(getAnnouncementDisplayStatus(item))">
              {{ announcementStatusLabels[getAnnouncementDisplayStatus(item)] }}
            </Badge>
          </div>
          <div class="mt-3 flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground">
            <span>{{ announcementCategoryLabels[item.category] }}</span>
            <span v-if="item.level === 'important'" class="font-medium text-primary">重要</span>
            <span v-if="item.isPinned">置顶</span>
          </div>
          <dl class="mt-4 grid gap-3 border-y border-border py-3 text-sm sm:grid-cols-2">
            <div>
              <dt class="text-xs text-muted-foreground">展示范围</dt>
              <dd class="mt-1 font-medium">{{ channelsText(item) }}</dd>
            </div>
            <div>
              <dt class="text-xs text-muted-foreground">发布时间</dt>
              <dd class="mt-1 font-medium">{{ formatAnnouncementDateTime(item.publishAt) }}</dd>
            </div>
          </dl>
          <div class="mt-3 flex items-center justify-between gap-2">
            <Button size="sm" variant="outline" @click="openPreview(item)">
              <Eye class="h-4 w-4" />
              预览
            </Button>
            <DropdownMenu>
              <DropdownMenuTrigger as-child>
                <Button size="icon" variant="outline" :aria-label="`${item.title}更多操作`">
                  <Ellipsis class="h-4 w-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" class="w-40">
                <DropdownMenuItem @select="editAnnouncement(item)">
                  <FilePenLine />
                  编辑
                </DropdownMenuItem>
                <DropdownMenuItem
                  :disabled="!canPublish(item) || publishMutation.isPending.value"
                  @select="publishAnnouncement(item)"
                >
                  <Send />
                  发布
                </DropdownMenuItem>
                <DropdownMenuItem
                  variant="destructive"
                  :disabled="!canOffline(item) || offlineMutation.isPending.value"
                  @select="startOffline(item)"
                >
                  <XCircle />
                  下线
                </DropdownMenuItem>
                <DropdownMenuItem
                  :disabled="duplicateMutation.isPending.value"
                  @select="duplicateAnnouncement(item)"
                >
                  <Copy />
                  复制为草稿
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </Card>
      </div>

      <div class="hidden lg:block">
        <SoftTable
          class="announcement-workbench-table"
          :columns="['公告', '展示范围', '发布时间', '状态', '操作']"
        >
          <tr v-for="item in visibleRows" :key="item.id">
            <td>
              <div class="line-clamp-1 font-semibold">{{ item.title }}</div>
              <div class="mt-1 line-clamp-1 text-xs leading-5 text-muted-foreground">{{ item.summary }}</div>
              <div class="mt-1.5 flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground">
                <span>{{ announcementCategoryLabels[item.category] }}</span>
                <span v-if="item.level === 'important'" class="font-medium text-primary">重要</span>
                <span v-if="item.isPinned">置顶</span>
              </div>
            </td>
            <td class="text-sm">
              <div class="font-medium">{{ channelsText(item) }}</div>
              <div class="mt-1 text-xs text-muted-foreground">全部用户</div>
            </td>
            <td class="text-xs leading-5 text-muted-foreground">
              <div>{{ formatAnnouncementDateTime(item.publishAt) }}</div>
              <div v-if="item.expireAt">结束 {{ formatAnnouncementDateTime(item.expireAt) }}</div>
              <div v-else>长期有效</div>
            </td>
            <td>
              <Badge :variant="badgeVariant(getAnnouncementDisplayStatus(item))">
                {{ announcementStatusLabels[getAnnouncementDisplayStatus(item)] }}
              </Badge>
            </td>
            <td>
              <div class="flex items-center gap-2">
                <Button size="sm" variant="outline" @click="openPreview(item)">
                  <Eye class="h-4 w-4" />
                  预览
                </Button>
                <DropdownMenu>
                  <DropdownMenuTrigger as-child>
                    <Button size="icon" variant="outline" :aria-label="`${item.title}更多操作`">
                      <Ellipsis class="h-4 w-4" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" class="w-40">
                    <DropdownMenuItem @select="editAnnouncement(item)">
                      <FilePenLine />
                      编辑
                    </DropdownMenuItem>
                    <DropdownMenuItem
                      :disabled="!canPublish(item) || publishMutation.isPending.value"
                      @select="publishAnnouncement(item)"
                    >
                      <Send />
                      发布
                    </DropdownMenuItem>
                    <DropdownMenuItem
                      variant="destructive"
                      :disabled="!canOffline(item) || offlineMutation.isPending.value"
                      @select="startOffline(item)"
                    >
                      <XCircle />
                      下线
                    </DropdownMenuItem>
                    <DropdownMenuItem
                      :disabled="duplicateMutation.isPending.value"
                      @select="duplicateAnnouncement(item)"
                    >
                      <Copy />
                      复制为草稿
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            </td>
          </tr>
          <template #footer>
            <CursorTablePagination
              :page="pagination.page.value"
              :item-count="visibleRows.length"
              :has-next-page="Boolean(pageQuery.data.value?.nextCursor)"
              :loading="pageQuery.isFetching.value"
              @previous="pagination.previous"
              @next="pagination.next(pageQuery.data.value?.nextCursor)"
            />
          </template>
        </SoftTable>
      </div>

      <div class="lg:hidden">
        <CursorTablePagination
          :page="pagination.page.value"
          :item-count="visibleRows.length"
          :has-next-page="Boolean(pageQuery.data.value?.nextCursor)"
          :loading="pageQuery.isFetching.value"
          @previous="pagination.previous"
          @next="pagination.next(pageQuery.data.value?.nextCursor)"
        />
      </div>
    </template>

    <Dialog v-model:open="previewOpen">
      <DialogContent class="bottom-0 left-auto right-0 top-0 flex h-dvh max-h-dvh w-full max-w-full translate-x-0 translate-y-0 grid-cols-1 gap-0 overflow-hidden rounded-none border-l border-r-0 p-0 shadow-xl duration-200 data-[state=closed]:slide-out-to-right data-[state=open]:slide-in-from-right data-[state=closed]:zoom-out-100 data-[state=open]:zoom-in-100 sm:max-w-2xl">
        <div class="flex h-full min-h-0 flex-col">
          <DialogHeader class="border-b border-border px-5 py-4 pr-12">
            <div class="flex flex-wrap items-center gap-2">
              <DialogTitle>公告详情</DialogTitle>
              <Badge
                v-if="previewAnnouncement"
                :variant="badgeVariant(getAnnouncementDisplayStatus(previewAnnouncement))"
              >
                {{ announcementStatusLabels[getAnnouncementDisplayStatus(previewAnnouncement)] }}
              </Badge>
            </div>
            <DialogDescription>查看公告内容、展示范围和审计记录。</DialogDescription>
          </DialogHeader>

          <div v-if="previewAnnouncement" class="min-h-0 flex-1 overflow-y-auto px-5 py-5">
            <section>
              <div class="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                <Badge variant="outline">{{ announcementCategoryLabels[previewAnnouncement.category] }}</Badge>
                <Badge v-if="previewAnnouncement.level === 'important'" variant="trust">重要</Badge>
                <Badge v-if="previewAnnouncement.isPinned" variant="secondary">置顶</Badge>
              </div>
              <h2 class="mt-4 text-xl font-semibold leading-7">{{ previewAnnouncement.title }}</h2>
              <p class="mt-2 text-sm leading-6 text-muted-foreground">{{ previewAnnouncement.summary }}</p>
            </section>

            <dl class="mt-5 grid gap-x-5 gap-y-4 border-y border-border py-5 text-sm sm:grid-cols-2">
              <div>
                <dt class="text-xs text-muted-foreground">展示范围</dt>
                <dd class="mt-1 font-medium">{{ channelsText(previewAnnouncement) }}</dd>
              </div>
              <div>
                <dt class="text-xs text-muted-foreground">面向用户</dt>
                <dd class="mt-1 font-medium">全部用户</dd>
              </div>
              <div>
                <dt class="text-xs text-muted-foreground">发布时间</dt>
                <dd class="mt-1 font-medium">{{ formatAnnouncementDateTime(previewAnnouncement.publishAt) }}</dd>
              </div>
              <div>
                <dt class="text-xs text-muted-foreground">结束时间</dt>
                <dd class="mt-1 font-medium">{{ formatAnnouncementDateTime(previewAnnouncement.expireAt) }}</dd>
              </div>
              <div>
                <dt class="text-xs text-muted-foreground">最近更新</dt>
                <dd class="mt-1 font-medium">{{ formatAnnouncementDateTime(previewAnnouncement.updatedAt) }}</dd>
              </div>
              <div>
                <dt class="text-xs text-muted-foreground">内容版本</dt>
                <dd class="mt-1 font-medium">v{{ previewAnnouncement.version }}</dd>
              </div>
            </dl>

            <section class="py-5">
              <h3 class="text-sm font-semibold">公告预览</h3>
              <div class="mt-3 rounded-lg border border-border bg-muted/20 p-4">
                <AnnouncementDetailContent :content-markdown="previewAnnouncement.contentMarkdown" />
              </div>
            </section>

            <section class="border-t border-border pt-5">
              <h3 class="text-sm font-semibold">审计记录</h3>
              <p v-if="auditLogsLoading" class="mt-4 text-sm text-muted-foreground">正在读取审计记录...</p>
              <div v-else-if="previewAuditLogs.length" class="mt-4">
                <article
                  v-for="log in previewAuditLogs"
                  :key="log.id"
                  class="border-b border-border py-4 first:pt-0 last:border-b-0 last:pb-0"
                >
                  <div class="flex items-start justify-between gap-3">
                    <div class="min-w-0">
                      <div class="flex flex-wrap items-center gap-2">
                        <Badge variant="secondary">{{ announcementAuditActionLabels[log.action] }}</Badge>
                        <span class="text-sm font-medium">{{ log.operatorName }}</span>
                      </div>
                      <p v-if="log.reason" class="mt-2 text-sm leading-6 text-muted-foreground">{{ log.reason }}</p>
                    </div>
                    <time class="shrink-0 text-xs text-muted-foreground">{{ formatAnnouncementDateTime(log.createdAt) }}</time>
                  </div>
                </article>
              </div>
              <p v-else class="mt-4 text-sm text-muted-foreground">当前公告暂无审计记录。</p>
            </section>
          </div>

          <DialogFooter v-if="previewAnnouncement" class="border-t border-border px-5 py-4">
            <div class="flex w-full items-center gap-2">
              <Button class="flex-1" @click="editAnnouncement(previewAnnouncement)">
                <FilePenLine class="h-4 w-4" />
                编辑公告
              </Button>
              <DropdownMenu>
                <DropdownMenuTrigger as-child>
                  <Button size="icon" variant="outline" aria-label="更多公告操作">
                    <Ellipsis class="h-4 w-4" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" class="w-40">
                  <DropdownMenuItem
                    :disabled="!canPublish(previewAnnouncement) || publishMutation.isPending.value"
                    @select="publishAnnouncement(previewAnnouncement)"
                  >
                    <Send />
                    发布
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    variant="destructive"
                    :disabled="!canOffline(previewAnnouncement) || offlineMutation.isPending.value"
                    @select="startOffline(previewAnnouncement)"
                  >
                    <XCircle />
                    下线
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    :disabled="duplicateMutation.isPending.value"
                    @select="duplicateAnnouncement(previewAnnouncement)"
                  >
                    <Copy />
                    复制为草稿
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          </DialogFooter>
        </div>
      </DialogContent>
    </Dialog>

    <Dialog v-model:open="offlineOpen">
      <DialogContent class="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>下线公告</DialogTitle>
          <DialogDescription v-if="offlineTarget">
            确认下线“{{ offlineTarget.title }}”，并记录本次操作原因。
          </DialogDescription>
        </DialogHeader>

        <template v-if="offlineTarget">
          <Alert variant="destructive">
            <TriangleAlert class="h-4 w-4" />
            <AlertTitle>下线后立即对用户不可见</AlertTitle>
            <AlertDescription>用户端列表、详情和首页公告条都会停止展示，审计记录仍会保留。</AlertDescription>
          </Alert>

          <label class="space-y-2">
            <span class="text-sm font-medium">下线原因</span>
            <Textarea
              v-model="offlineReason"
              class="min-h-28"
              maxlength="500"
              placeholder="填写内容过期、规则调整或需要重新审核等具体原因"
            />
            <span class="block text-right text-xs text-muted-foreground">{{ offlineReason.length }}/500</span>
          </label>

          <label class="flex items-start gap-3 rounded-lg border border-border p-3 text-sm leading-6">
            <Checkbox v-model="offlineConfirmed" class="mt-1" />
            <span>我已核对公告状态和影响范围，确认下线并保留审计记录。</span>
          </label>
        </template>

        <DialogFooter>
          <Button variant="outline" :disabled="offlineMutation.isPending.value" @click="offlineOpen = false">
            取消
          </Button>
          <Button
            variant="destructive"
            :disabled="offlineMutation.isPending.value || !offlineReason.trim() || !offlineConfirmed"
            @click="confirmOffline"
          >
            {{ offlineMutation.isPending.value ? '正在下线...' : '确认下线' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>

<style scoped>
.announcement-workbench-table :deep(.c2c-soft-table th:nth-child(1)),
.announcement-workbench-table :deep(.c2c-soft-table td:nth-child(1)) {
  width: 36%;
}

.announcement-workbench-table :deep(.c2c-soft-table th:nth-child(2)),
.announcement-workbench-table :deep(.c2c-soft-table td:nth-child(2)) {
  width: 19%;
}

.announcement-workbench-table :deep(.c2c-soft-table th:nth-child(3)),
.announcement-workbench-table :deep(.c2c-soft-table td:nth-child(3)) {
  width: 20%;
}

.announcement-workbench-table :deep(.c2c-soft-table th:nth-child(4)),
.announcement-workbench-table :deep(.c2c-soft-table td:nth-child(4)) {
  width: 10%;
}

.announcement-workbench-table :deep(.c2c-soft-table th:nth-child(5)),
.announcement-workbench-table :deep(.c2c-soft-table td:nth-child(5)) {
  width: 15%;
  white-space: nowrap;
}
</style>
