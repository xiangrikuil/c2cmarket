<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { CheckCircle2, CircleAlert, Clock3, MessageSquarePlus, SendHorizonal } from 'lucide-vue-next'
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
  type FeedbackTicket,
  type FeedbackTicketType,
} from '@/lib/api'
import {
  useAddFeedbackSupplementMutation,
  useMarkFeedbackReadMutation,
  useMyFeedbackTicket,
  useMyFeedbackTickets,
  useSubmitFeedbackMutation,
} from '@/queries/useMarketQueries'

const route = useRoute()
const router = useRouter()
const { data: tickets } = useMyFeedbackTickets()
const activeId = computed(() => typeof route.params.id === 'string' ? route.params.id : '')
const { data: detailTicket } = useMyFeedbackTicket(activeId)
const submitMutation = useSubmitFeedbackMutation()
const supplementMutation = useAddFeedbackSupplementMutation()
const markReadMutation = useMarkFeedbackReadMutation()

const activeTab = ref('全部')
const supplementText = ref('')
const form = reactive({
  type: 'function_issue' as FeedbackTicketType,
  impact: 'general' as FeedbackImpact,
  title: '',
  contextPageLabel: '',
  contextTargetLabel: '',
  contextRoleLabel: '',
  description: '',
})

const rows = computed(() => tickets.value ?? [])
const selectedTicket = computed(() => detailTicket.value ?? rows.value.find(item => item.id === activeId.value) ?? rows.value[0] ?? null)
const unreadCount = computed(() => rows.value.filter(item => item.unread).length)
const needsInfoCount = computed(() => rows.value.filter(item => item.status === 'needs_user_info').length)
const openCount = computed(() => rows.value.filter(item => ['submitted', 'recorded', 'following_up'].includes(item.status)).length)
const closedCount = computed(() => rows.value.filter(item => ['resolved', 'declined', 'closed'].includes(item.status)).length)

const visibleRows = computed(() => rows.value.filter(item => {
  if (activeTab.value === '有新回复') return item.unread
  if (activeTab.value === '待处理') return ['submitted', 'recorded', 'following_up'].includes(item.status)
  if (activeTab.value === '需要补充') return item.status === 'needs_user_info'
  if (activeTab.value === '已关闭') return ['resolved', 'declined', 'closed'].includes(item.status)
  return true
}))

const stats = computed(() => [
  { label: '有新回复', value: unreadCount.value, icon: CircleAlert },
  { label: '待处理', value: openCount.value, icon: Clock3 },
  { label: '需要补充', value: needsInfoCount.value, icon: MessageSquarePlus },
  { label: '已结束', value: closedCount.value, icon: CheckCircle2 },
])

watch(selectedTicket, item => {
  if (item?.unread && !markReadMutation.isPending.value) {
    markReadMutation.mutate(item.id)
  }
}, { immediate: true })

function submitFeedbackForm() {
  const description = form.description.trim()
  const contextPageLabel = form.contextPageLabel.trim()
  if (description.length < 4) {
    toast.warning('请填写更具体的问题描述。')
    return
  }
  if (contextPageLabel.length < 2) {
    toast.warning('请填写当前页面。')
    return
  }
  submitMutation.mutate({
    type: form.type,
    impact: form.impact,
    title: form.title.trim() || undefined,
    description,
    contextPageLabel,
    contextTargetLabel: form.contextTargetLabel.trim() || undefined,
    contextRoleLabel: form.contextRoleLabel.trim() || undefined,
  }, {
    onSuccess: item => {
      toast.success('反馈已提交。')
      form.title = ''
      form.description = ''
      form.contextTargetLabel = ''
      router.push(`/my/feedback/${item.id}`)
    },
    onError: error => toast.error(error instanceof Error ? error.message : '提交失败'),
  })
}

function addSupplement() {
  const message = supplementText.value.trim()
  if (!selectedTicket.value) return
  if (message.length < 2) {
    toast.warning('请填写补充说明。')
    return
  }
  supplementMutation.mutate({ id: selectedTicket.value.id, payload: { message } }, {
    onSuccess: item => {
      supplementText.value = ''
      toast.success('补充说明已提交。')
      router.push(`/my/feedback/${item.id}`)
    },
    onError: error => toast.error(error instanceof Error ? error.message : '提交失败'),
  })
}

function markCurrentRead() {
  if (!selectedTicket.value) return
  markReadMutation.mutate(selectedTicket.value.id, {
    onSuccess: () => toast.success('已确认处理结果。'),
    onError: error => toast.error(error instanceof Error ? error.message : '操作失败'),
  })
}

function statusVariant(item: FeedbackTicket) {
  if (item.unread || item.status === 'needs_user_info') return 'default'
  return 'secondary'
}
</script>

<template>
  <div class="space-y-5">
    <PageTitle title="我的反馈" description="提交页面问题、数据纠错和体验建议；举报、纠纷和申诉仍请使用对应入口。" />

    <div class="grid gap-3 md:grid-cols-4">
      <Card v-for="item in stats" :key="item.label" class="p-4">
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

    <div class="grid gap-5 xl:grid-cols-[minmax(0,0.95fr)_minmax(360px,0.75fr)]">
      <div class="space-y-4">
        <Card class="p-4">
          <div class="mb-4 flex items-center justify-between gap-3">
            <div>
              <h2 class="font-semibold">反馈记录</h2>
              <p class="mt-1 text-sm text-muted-foreground">红点表示管理员已有你未读的处理结果或补充要求。</p>
            </div>
            <Badge v-if="unreadCount" variant="default">{{ unreadCount }}</Badge>
          </div>
          <StatusTabs v-model="activeTab" :items="['全部', '有新回复', '待处理', '需要补充', '已关闭']" />
          <div class="grid gap-2">
            <RouterLink
              v-for="item in visibleRows"
              :key="item.id"
              :to="`/my/feedback/${item.id}`"
              class="rounded-md border border-border bg-background p-3 transition hover:border-primary/40 hover:bg-accent/40"
              :class="selectedTicket?.id === item.id ? 'border-primary/50 bg-accent/60' : ''"
            >
              <div class="flex flex-wrap items-center justify-between gap-2">
                <div class="font-medium">{{ item.title }}</div>
                <Badge :variant="statusVariant(item)">{{ item.unread ? '有新回复' : getFeedbackStatusLabel(item.status) }}</Badge>
              </div>
              <div class="mt-2 text-sm text-muted-foreground">{{ getFeedbackTypeLabel(item.type) }} · {{ getFeedbackImpactLabel(item.impact) }} · {{ item.contextPageLabel }}</div>
              <div class="mt-1 text-xs text-muted-foreground">{{ item.updatedAt }}</div>
            </RouterLink>
            <div v-if="visibleRows.length === 0" class="rounded-md border border-dashed border-border p-8 text-center text-sm text-muted-foreground">当前筛选下暂无反馈记录。</div>
          </div>
        </Card>

        <Card class="p-4">
          <div class="mb-4">
            <h2 class="font-semibold">提交反馈</h2>
            <p class="mt-1 text-sm text-muted-foreground">提交时会附带页面上下文、关联内容和文字描述，便于管理员定位问题。</p>
          </div>
          <div class="grid gap-3 md:grid-cols-2">
            <label class="space-y-1.5">
              <span class="text-sm font-medium">反馈类型</span>
              <select v-model="form.type" class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm">
                <option value="function_issue">功能问题</option>
                <option value="data_correction">数据纠错</option>
                <option value="experience_suggestion">体验建议</option>
                <option value="publish_contact_block">发布/联系受阻</option>
              </select>
            </label>
            <label class="space-y-1.5">
              <span class="text-sm font-medium">影响程度</span>
              <select v-model="form.impact" class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm">
                <option value="general">一般</option>
                <option value="blocks_operation">影响操作</option>
                <option value="cannot_continue">无法继续</option>
              </select>
            </label>
            <label class="space-y-1.5">
              <span class="text-sm font-medium">当前页面</span>
              <Input v-model="form.contextPageLabel" placeholder="例如：API 服务详情" />
            </label>
            <label class="space-y-1.5">
              <span class="text-sm font-medium">关联内容</span>
              <Input v-model="form.contextTargetLabel" placeholder="例如：小葵 API 服务" />
            </label>
            <label class="space-y-1.5">
              <span class="text-sm font-medium">当前身份</span>
              <Input v-model="form.contextRoleLabel" placeholder="例如：买家、车主、商户" />
            </label>
            <label class="space-y-1.5">
              <span class="text-sm font-medium">标题</span>
              <Input v-model="form.title" placeholder="可选，留空将自动生成" />
            </label>
          </div>
          <label class="mt-3 block space-y-1.5">
            <span class="text-sm font-medium">问题描述</span>
            <Textarea v-model="form.description" class="min-h-28" placeholder="描述你遇到的问题、预期结果和已尝试的操作。" />
          </label>
          <div class="mt-4 flex justify-end">
            <Button :disabled="submitMutation.isPending.value" @click="submitFeedbackForm">
              <SendHorizonal class="h-4 w-4" />
              提交反馈
            </Button>
          </div>
        </Card>
      </div>

      <Card class="p-4">
        <div v-if="selectedTicket" class="space-y-4">
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div>
              <h2 class="text-lg font-semibold">{{ selectedTicket.title }}</h2>
              <p class="mt-1 text-sm text-muted-foreground">{{ getFeedbackTypeLabel(selectedTicket.type) }} · {{ getFeedbackImpactLabel(selectedTicket.impact) }}</p>
            </div>
            <Badge :variant="statusVariant(selectedTicket)">{{ selectedTicket.unread ? '有新回复' : getFeedbackStatusLabel(selectedTicket.status) }}</Badge>
          </div>

          <div class="grid gap-3 rounded-md border border-border bg-muted/30 p-3 text-sm sm:grid-cols-2">
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
              <div class="text-xs text-muted-foreground">最近更新</div>
              <div class="mt-1 font-medium">{{ selectedTicket.updatedAt }}</div>
            </div>
          </div>

          <div>
            <div class="text-sm font-medium">你的描述</div>
            <p class="mt-2 whitespace-pre-wrap rounded-md border border-border bg-background p-3 text-sm leading-6">{{ selectedTicket.description }}</p>
          </div>

          <div v-if="selectedTicket.adminResponse" class="rounded-md border border-primary/20 bg-primary/5 p-3">
            <div class="flex items-center justify-between gap-3">
              <div class="text-sm font-medium">管理员处理说明</div>
              <Button size="sm" variant="outline" :disabled="!selectedTicket.unread || markReadMutation.isPending.value" @click="markCurrentRead">
                {{ selectedTicket.unread ? '我知道了' : '已确认' }}
              </Button>
            </div>
            <p class="mt-2 whitespace-pre-wrap text-sm leading-6">{{ selectedTicket.adminResponse }}</p>
          </div>

          <div class="space-y-2">
            <div class="text-sm font-medium">后续补充说明</div>
            <Textarea v-model="supplementText" class="min-h-24" :disabled="selectedTicket.status === 'closed'" placeholder="补充你看到的新情况或管理员要求的信息。" />
            <div class="flex justify-end">
              <Button variant="outline" :disabled="selectedTicket.status === 'closed' || supplementMutation.isPending.value" @click="addSupplement">追加说明</Button>
            </div>
          </div>

          <div>
            <div class="mb-2 text-sm font-medium">处理时间线</div>
            <div class="space-y-2">
              <div v-for="event in selectedTicket.events ?? []" :key="event.id" class="rounded-md border border-border p-3">
                <div class="flex flex-wrap items-center justify-between gap-2">
                  <span class="text-sm font-medium">{{ event.actorName }}</span>
                  <span class="text-xs text-muted-foreground">{{ event.createdAt }}</span>
                </div>
                <p class="mt-2 whitespace-pre-wrap text-sm leading-6 text-muted-foreground">{{ event.publicMessage }}</p>
              </div>
              <div v-if="!(selectedTicket.events ?? []).length" class="rounded-md border border-dashed border-border p-6 text-center text-sm text-muted-foreground">暂无处理记录。</div>
            </div>
          </div>
        </div>
        <div v-else class="grid min-h-[360px] place-items-center text-center text-sm text-muted-foreground">
          暂无反馈记录。提交第一条反馈后会在这里查看处理结果。
        </div>
      </Card>
    </div>
  </div>
</template>
