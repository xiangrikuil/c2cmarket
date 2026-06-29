<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { CheckCircle2, Clock3, MessageSquareWarning, UserRoundCheck } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import PageTitle from '@/components/market/PageTitle.vue'
import StatusTabs from '@/components/market/StatusTabs.vue'
import {
  getFeedbackImpactLabel,
  getFeedbackStatusLabel,
  getFeedbackTypeLabel,
  type FeedbackImpact,
  type FeedbackStatus,
  type FeedbackTicket,
  type FeedbackTicketType,
} from '@/lib/api'
import { useAdminFeedbackTicket, useAdminFeedbackTickets, useHandleFeedbackTicketMutation } from '@/queries/useMarketQueries'

const route = useRoute()
const router = useRouter()
const { data: tickets } = useAdminFeedbackTickets()
const activeId = computed(() => typeof route.params.id === 'string' ? route.params.id : '')
const { data: detailTicket } = useAdminFeedbackTicket(activeId)
const handleMutation = useHandleFeedbackTicketMutation()

const activeTab = ref('全部')
const searchText = ref('')
const typeFilter = ref<'all' | FeedbackTicketType>('all')
const impactFilter = ref<'all' | FeedbackImpact>('all')
const selectedId = ref('')
const handleForm = reactive({
  status: 'recorded' as Exclude<FeedbackStatus, 'submitted'>,
  response: '',
  internalNote: '',
})

const rows = computed(() => tickets.value ?? [])
const selectedTicket = computed(() => detailTicket.value ?? rows.value.find(item => item.id === selectedId.value || item.id === activeId.value) ?? rows.value[0] ?? null)

watch(rows, items => {
  if (activeId.value) {
    selectedId.value = activeId.value
    return
  }
  if (!items.some(item => item.id === selectedId.value)) selectedId.value = items[0]?.id ?? ''
}, { immediate: true })

watch(selectedTicket, item => {
  if (!item) return
  handleForm.status = item.status === 'submitted' ? 'recorded' : item.status
  handleForm.response = item.adminResponse ?? ''
  handleForm.internalNote = item.adminInternalNote ?? ''
}, { immediate: true })

const metrics = computed(() => [
  { label: '待处理', value: rows.value.filter(item => item.status === 'submitted').length, icon: Clock3 },
  { label: '需要补充', value: rows.value.filter(item => item.status === 'needs_user_info').length, icon: MessageSquareWarning },
  { label: '用户未读', value: rows.value.filter(item => item.unread).length, icon: UserRoundCheck },
  { label: '已结束', value: rows.value.filter(item => ['resolved', 'declined', 'closed'].includes(item.status)).length, icon: CheckCircle2 },
])

const filteredRows = computed(() => {
  const keyword = searchText.value.trim().toLowerCase()
  return rows.value.filter(item => {
    const tabMatched = activeTab.value === '全部'
      || (activeTab.value === '待处理' && ['submitted', 'recorded', 'following_up'].includes(item.status))
      || (activeTab.value === '需要补充' && item.status === 'needs_user_info')
      || (activeTab.value === '用户未读' && item.unread)
      || (activeTab.value === '已结束' && ['resolved', 'declined', 'closed'].includes(item.status))
    const keywordMatched = !keyword || [
      item.title,
      item.description,
      item.submitterName,
      item.contextPageLabel,
      item.contextTargetLabel,
    ].some(value => value.toLowerCase().includes(keyword))
    return tabMatched
      && keywordMatched
      && (typeFilter.value === 'all' || item.type === typeFilter.value)
      && (impactFilter.value === 'all' || item.impact === impactFilter.value)
  })
})

function selectTicket(item: FeedbackTicket) {
  selectedId.value = item.id
  router.replace(`/admin/feedback/${item.id}`)
}

function submitHandle() {
  if (!selectedTicket.value) return
  const response = handleForm.response.trim()
  if (response.length < 2) {
    toast.warning('请填写面向用户的处理说明。')
    return
  }
  handleMutation.mutate({
    id: selectedTicket.value.id,
    version: selectedTicket.value.version,
    payload: {
      status: handleForm.status,
      response,
      internalNote: handleForm.internalNote.trim() || undefined,
    },
  }, {
    onSuccess: item => {
      toast.success('处理结果已发送给用户。')
      router.replace(`/admin/feedback/${item.id}`)
    },
    onError: error => toast.error(error instanceof Error ? error.message : '处理失败'),
  })
}

function badgeVariant(item: FeedbackTicket) {
  if (item.status === 'submitted' || item.status === 'needs_user_info' || item.unread) return 'default'
  return 'secondary'
}
</script>

<template>
  <div class="space-y-5">
    <PageTitle title="问题反馈处理台" description="处理产品问题、数据纠错和体验建议；举报、纠纷和申诉仍在对应管理入口处理。" />

    <div class="grid gap-3 md:grid-cols-4">
      <Card v-for="item in metrics" :key="item.label" class="p-4">
        <div class="flex items-center justify-between gap-3">
          <div>
            <div class="text-xs text-muted-foreground">{{ item.label }}</div>
            <div class="mt-1 text-2xl font-semibold">{{ item.value }}</div>
          </div>
          <div class="grid h-9 w-9 place-items-center rounded-md bg-accent text-primary">
            <component :is="item.icon" class="h-4 w-4" />
          </div>
        </div>
      </Card>
    </div>

    <div class="grid gap-5 xl:grid-cols-[minmax(380px,0.8fr)_minmax(0,1fr)]">
      <Card class="p-4">
        <div class="mb-4 flex flex-col gap-3">
          <div>
            <h2 class="font-semibold">反馈队列</h2>
            <p class="mt-1 text-sm text-muted-foreground">按状态、类型、影响程度和关键词筛选。</p>
          </div>
          <StatusTabs v-model="activeTab" :items="['全部', '待处理', '需要补充', '用户未读', '已结束']" />
          <div class="grid gap-2 md:grid-cols-[1fr_150px_150px]">
            <Input v-model="searchText" placeholder="搜索标题、描述、用户或关联内容" />
            <select v-model="typeFilter" class="h-9 rounded-md border border-input bg-background px-3 text-sm">
              <option value="all">全部类型</option>
              <option value="function_issue">功能问题</option>
              <option value="data_correction">数据纠错</option>
              <option value="experience_suggestion">体验建议</option>
              <option value="publish_contact_block">发布/联系受阻</option>
            </select>
            <select v-model="impactFilter" class="h-9 rounded-md border border-input bg-background px-3 text-sm">
              <option value="all">全部影响</option>
              <option value="general">一般</option>
              <option value="blocks_operation">影响操作</option>
              <option value="cannot_continue">无法继续</option>
            </select>
          </div>
        </div>

        <div class="grid gap-2">
          <button
            v-for="item in filteredRows"
            :key="item.id"
            type="button"
            class="rounded-md border border-border bg-background p-3 text-left transition hover:border-primary/40 hover:bg-accent/40"
            :class="selectedTicket?.id === item.id ? 'border-primary/50 bg-accent/60' : ''"
            @click="selectTicket(item)"
          >
            <div class="flex flex-wrap items-center justify-between gap-2">
              <div class="font-medium">{{ item.title }}</div>
              <Badge :variant="badgeVariant(item)">{{ item.unread ? '用户未读' : getFeedbackStatusLabel(item.status) }}</Badge>
            </div>
            <div class="mt-2 text-sm text-muted-foreground">{{ getFeedbackTypeLabel(item.type) }} · {{ getFeedbackImpactLabel(item.impact) }} · {{ item.submitterName }}</div>
            <div class="mt-1 text-xs text-muted-foreground">{{ item.contextPageLabel }} · {{ item.contextTargetLabel || '未指定关联内容' }}</div>
          </button>
          <div v-if="filteredRows.length === 0" class="rounded-md border border-dashed border-border p-8 text-center text-sm text-muted-foreground">当前筛选下暂无反馈。</div>
        </div>
      </Card>

      <div class="space-y-4">
        <Card class="p-4">
          <div v-if="selectedTicket" class="space-y-4">
            <div class="flex flex-wrap items-start justify-between gap-3">
              <div>
                <h2 class="text-lg font-semibold">{{ selectedTicket.title }}</h2>
                <p class="mt-1 text-sm text-muted-foreground">{{ selectedTicket.submitterName }} · {{ getFeedbackTypeLabel(selectedTicket.type) }} · {{ getFeedbackImpactLabel(selectedTicket.impact) }}</p>
              </div>
              <Badge :variant="badgeVariant(selectedTicket)">{{ getFeedbackStatusLabel(selectedTicket.status) }}</Badge>
            </div>

            <div class="grid gap-3 rounded-md border border-border bg-muted/30 p-3 text-sm md:grid-cols-4">
              <div>
                <div class="text-xs text-muted-foreground">当前页面</div>
                <div class="mt-1 font-medium">{{ selectedTicket.contextPageLabel }}</div>
              </div>
              <div>
                <div class="text-xs text-muted-foreground">关联内容</div>
                <div class="mt-1 font-medium">{{ selectedTicket.contextTargetLabel || '未指定' }}</div>
              </div>
              <div>
                <div class="text-xs text-muted-foreground">当前身份</div>
                <div class="mt-1 font-medium">{{ selectedTicket.contextRoleLabel || '普通用户' }}</div>
              </div>
              <div>
                <div class="text-xs text-muted-foreground">用户已读</div>
                <div class="mt-1 font-medium">{{ selectedTicket.unread ? '否' : '是' }}</div>
              </div>
            </div>

            <div>
              <div class="text-sm font-medium">用户描述</div>
              <p class="mt-2 whitespace-pre-wrap rounded-md border border-border bg-background p-3 text-sm leading-6">{{ selectedTicket.description }}</p>
            </div>

            <div class="grid gap-3 md:grid-cols-[220px_1fr]">
              <label class="space-y-1.5">
                <span class="text-sm font-medium">处理状态</span>
                <select v-model="handleForm.status" class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm">
                  <option value="recorded">已记录</option>
                  <option value="following_up">跟进中</option>
                  <option value="resolved">已修复/已调整</option>
                  <option value="declined">暂不处理</option>
                  <option value="needs_user_info">需要补充信息</option>
                  <option value="closed">关闭反馈</option>
                </select>
              </label>
              <label class="space-y-1.5">
                <span class="text-sm font-medium">面向用户的处理说明</span>
                <Textarea v-model="handleForm.response" class="min-h-24" placeholder="说明已记录、处理结果、下一步安排或需要用户补充的信息。" />
              </label>
            </div>

            <label class="block space-y-1.5">
              <span class="text-sm font-medium">内部备注</span>
              <Textarea v-model="handleForm.internalNote" class="min-h-20" placeholder="仅管理员可见，可记录排查结论、负责人或后续处理建议。" />
            </label>

            <div class="flex flex-wrap items-center justify-between gap-3">
              <p class="text-xs leading-5 text-muted-foreground">提交处理后，用户头像下拉的“问题反馈”会出现未读提示，直到用户查看详情或确认处理结果。</p>
              <Button :disabled="handleMutation.isPending.value || selectedTicket.status === 'closed'" @click="submitHandle">发送处理结果</Button>
            </div>
          </div>
          <div v-else class="grid min-h-[360px] place-items-center text-center text-sm text-muted-foreground">暂无反馈记录。</div>
        </Card>

        <Card v-if="selectedTicket" class="p-4">
          <div class="mb-3 font-semibold">反馈时间线</div>
          <div class="space-y-2">
            <div v-for="event in selectedTicket.events ?? []" :key="event.id" class="rounded-md border border-border p-3">
              <div class="flex flex-wrap items-center justify-between gap-2">
                <span class="text-sm font-medium">{{ event.actorName }}</span>
                <span class="text-xs text-muted-foreground">{{ event.createdAt }}</span>
              </div>
              <p class="mt-2 whitespace-pre-wrap text-sm leading-6 text-muted-foreground">{{ event.publicMessage }}</p>
              <p v-if="event.internalNote" class="mt-2 rounded-md bg-muted/50 p-2 text-xs leading-5 text-muted-foreground">内部备注：{{ event.internalNote }}</p>
            </div>
            <div v-if="!(selectedTicket.events ?? []).length" class="rounded-md border border-dashed border-border p-6 text-center text-sm text-muted-foreground">暂无时间线记录。</div>
          </div>
        </Card>
      </div>
    </div>
  </div>
</template>
