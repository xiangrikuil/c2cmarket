<script setup lang="ts">
import { CheckCircle2, Circle, Eye } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'

defineProps<{
  completedCount: number
  wechatComplete: boolean
  emailComplete: boolean
  paymentComplete: boolean
}>()

const emit = defineEmits<{
  preview: []
}>()

const items = [
  { key: 'wechat', label: '微信' },
  { key: 'email', label: '验证邮箱' },
  { key: 'payment', label: 'API 收款' },
] as const
</script>

<template>
  <Card class="configuration-progress-card p-4">
    <div class="flex items-center justify-between gap-3">
      <div>
        <h2 class="font-semibold">配置完成度</h2>
        <p class="mt-1 text-xs text-muted-foreground">{{ completedCount }} / 3 项已完成</p>
      </div>
      <Badge :variant="completedCount === 3 ? 'verified' : 'secondary'">
        {{ completedCount === 3 ? '已完成' : '待完善' }}
      </Badge>
    </div>

    <div class="mt-4 space-y-3">
      <div v-for="item in items" :key="item.key" class="flex items-center justify-between gap-3 text-sm">
        <span class="flex items-center gap-2">
          <CheckCircle2
            v-if="item.key === 'wechat' ? wechatComplete : item.key === 'email' ? emailComplete : paymentComplete"
            class="h-4 w-4 text-success"
          />
          <Circle v-else class="h-4 w-4 text-muted-foreground" />
          {{ item.label }}
        </span>
        <span class="text-xs text-muted-foreground">
          {{ (item.key === 'wechat' ? wechatComplete : item.key === 'email' ? emailComplete : paymentComplete) ? '已配置' : '待配置' }}
        </span>
      </div>
    </div>

    <p class="mt-4 border-t border-border pt-4 text-xs leading-5 text-muted-foreground">
      仅在有效联系窗口或订单中展示，不会出现在公开主页。
    </p>
    <Button type="button" class="mt-4 w-full" variant="outline" @click="emit('preview')">
      <Eye class="h-4 w-4" />
      预览买家看到的信息
    </Button>
  </Card>
</template>
