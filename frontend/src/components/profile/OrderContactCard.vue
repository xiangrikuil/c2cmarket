<script setup lang="ts">
import { computed } from 'vue'
import { AlertTriangle, CheckCircle2, Copy, Flag, MessageCircle } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { useCreateContactReportMutation } from '@/queries/useMarketQueries'
import type { ContactMethodType, OrderContactSnapshot, OrderContactSnapshotItem } from '@/lib/api'

const props = defineProps<{
  snapshot: OrderContactSnapshot
  title: string
  side?: 'seller' | 'buyer'
  contextLabel?: string
  visibleLabel?: string
  hiddenLabel?: string
  footerText?: string
  contactedLabel?: string
  showContactedAction?: boolean
}>()

const emit = defineEmits<{
  contacted: []
}>()

const reportMutation = useCreateContactReportMutation()
const side = computed(() => props.side ?? 'seller')
const contacts = computed(() => side.value === 'seller' ? props.snapshot.sellerContacts : props.snapshot.buyerContacts)
const contextLabel = computed(() => props.contextLabel ?? (props.snapshot.contactWindowEndsAt ? `联系窗口截止 ${props.snapshot.contactWindowEndsAt}` : '当前记录无倒计时窗口'))
const visibleLabel = computed(() => props.visibleLabel ?? '联系窗口内可见')
const hiddenLabel = computed(() => props.hiddenLabel ?? '暂不可见')
const footerText = computed(() => props.footerText ?? '联系方式来自联系快照；修改个人联系方式不会改变当前记录。')
const showContactedAction = computed(() => props.showContactedAction ?? true)

function displayValue(item: OrderContactSnapshotItem) {
  return props.snapshot.canView ? item.displayValue ?? item.maskedValue : item.maskedValue
}

async function copyContact(item: OrderContactSnapshotItem) {
  if (!props.snapshot.canView || !item.displayValue) {
    toast.warning('当前只能查看脱敏联系方式。')
    return
  }
  await navigator.clipboard.writeText(item.displayValue)
  toast.success(`${item.label} 已复制。`)
}

function openLinuxDoMessage(item: OrderContactSnapshotItem) {
  if (!item.actionUrl) {
    toast('当前联系方式没有配置站内私信链接。')
    return
  }
  window.open(item.actionUrl, '_blank', 'noopener,noreferrer')
}

function reportContact(item: OrderContactSnapshotItem, reasonCode: 'invalid' | 'unreachable' | 'impersonation' | 'other') {
  reportMutation.mutate({
    orderType: props.snapshot.orderType,
    orderId: props.snapshot.orderId,
    contactType: item.type as ContactMethodType,
    reasonCode,
    note: `${item.label} ${reasonCode}`,
  }, {
    onSuccess: () => toast.success('联系方式问题已提交。'),
    onError: error => toast.error(error instanceof Error ? error.message : '提交失败'),
  })
}
</script>

<template>
  <Card class="p-5">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <div>
        <h2 class="font-semibold">{{ title }}</h2>
        <p class="mt-1 text-xs text-muted-foreground">{{ contextLabel }}</p>
      </div>
      <Badge :variant="snapshot.canView ? 'default' : 'secondary'">
        {{ snapshot.canView ? visibleLabel : hiddenLabel }}
      </Badge>
    </div>

    <div v-if="!snapshot.canView" class="mt-4 rounded-md border border-border bg-accent/60 p-3 text-sm text-muted-foreground">
      <AlertTriangle class="mr-1 inline h-4 w-4" />
      {{ snapshot.unavailableReason ?? '当前记录状态不允许查看联系方式。' }}
    </div>

    <div v-else-if="contacts.length" class="mt-4 space-y-3">
      <div v-for="item in contacts" :key="`${item.type}-${item.label}`" class="rounded-md border border-border p-3">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <span class="font-medium">{{ item.label }}</span>
              <Badge :variant="item.verified ? 'verified' : 'secondary'">{{ item.verified ? '已验证' : '未验证' }}</Badge>
            </div>
            <div class="mt-1 break-all text-sm text-muted-foreground">{{ displayValue(item) }}</div>
          </div>
          <div class="flex flex-wrap gap-2">
            <Button v-if="item.type === 'linuxdo' && item.actionUrl" size="sm" variant="outline" @click="openLinuxDoMessage(item)">
              <MessageCircle class="h-4 w-4" />发私信
            </Button>
            <Button v-else size="sm" variant="outline" @click="copyContact(item)">
              <Copy class="h-4 w-4" />复制
            </Button>
            <Button size="sm" variant="outline" @click="reportContact(item, 'invalid')">
              <Flag class="h-4 w-4" />无效
            </Button>
          </div>
        </div>
        <div class="mt-3 flex flex-wrap gap-2">
          <Button size="sm" variant="ghost" @click="reportContact(item, 'unreachable')">无法联系</Button>
          <Button size="sm" variant="ghost" @click="reportContact(item, 'impersonation')">疑似冒充</Button>
          <Button size="sm" variant="ghost" @click="reportContact(item, 'other')">举报</Button>
        </div>
      </div>
      <Button v-if="showContactedAction" size="sm" @click="emit('contacted')">
        <CheckCircle2 class="h-4 w-4" />{{ contactedLabel ?? '我已联系对方' }}
      </Button>
      <p class="text-xs text-muted-foreground">{{ footerText }}</p>
    </div>

    <div v-else class="mt-4 rounded-md border border-border bg-muted/40 p-3 text-sm text-muted-foreground">
      当前快照没有可展示的联系方式。
    </div>
  </Card>
</template>
