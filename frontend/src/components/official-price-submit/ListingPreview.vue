<script setup lang="ts">
import { CheckCircle2, ExternalLink, Info, MapPin } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import type { OfficialPriceSubmitForm, SourceLinkState, SubmitterPreview } from './types'

const props = defineProps<{
  form: OfficialPriceSubmitForm
  title: string
  formattedPrice: string
  methodTags: string[]
  sourceLinkState: SourceLinkState
  sourceHost: string
  submitter: SubmitterPreview
}>()

function initial(value: string) {
  return (value.trim()[0] ?? 'C').toUpperCase()
}

function noteSummary(value: string) {
  const trimmed = value.trim()
  if (!trimmed) return '备注会在此处摘要展示。'
  return trimmed.length > 42 ? `${trimmed.slice(0, 42)}...` : trimmed
}
</script>

<template>
  <Card class="overflow-hidden p-0 shadow-sm">
    <div class="flex items-start justify-between gap-3 px-4 py-3">
      <div>
        <h2 class="text-base font-semibold">前台展示预览</h2>
        <p class="mt-1 text-xs text-muted-foreground">右侧内容会随表单即时更新，提交前即可检查展示效果。</p>
      </div>
      <Badge variant="verified">实时同步</Badge>
    </div>
    <div class="mx-4 mb-4 overflow-hidden rounded-xl border border-border">
      <div class="bg-muted/45 p-4">
        <div class="flex items-start gap-3">
          <span class="grid h-10 w-10 shrink-0 place-items-center rounded-xl bg-foreground text-sm font-semibold text-background">
            {{ initial(form.product) }}
          </span>
          <div class="min-w-0 flex-1">
            <div class="flex items-start justify-between gap-2">
              <div class="min-w-0">
                <h3 class="truncate text-base font-semibold">{{ title || '选择产品与套餐' }}</h3>
                <p class="mt-1 text-xs text-muted-foreground">{{ form.region || '适用地区待填写' }} · {{ form.channel || '渠道待填写' }}</p>
              </div>
              <Badge variant="secondary">待审核</Badge>
            </div>
          </div>
        </div>
      </div>
      <div class="space-y-3 p-4">
        <div>
          <div class="text-xs text-muted-foreground">参考原价</div>
          <div class="mt-1 text-2xl font-semibold">{{ formattedPrice || '待填写' }}<span class="ml-1 text-sm text-muted-foreground">/月</span></div>
        </div>
        <div class="flex flex-wrap gap-1.5">
          <Badge variant="trust">低价线索</Badge>
          <Badge v-for="tag in methodTags" :key="tag" variant="model">{{ tag }}</Badge>
        </div>
        <div class="grid gap-2 text-xs text-muted-foreground">
          <div class="flex items-center gap-2">
            <MapPin class="h-3.5 w-3.5" />
            <span>适用地区：{{ form.region || '待填写' }}</span>
          </div>
          <div class="flex items-center gap-2">
            <ExternalLink class="h-3.5 w-3.5" />
            <span>来源：{{ sourceHost || '待填写' }} · {{ sourceLinkState === 'success' ? '链接格式有效' : '待验证' }}</span>
          </div>
          <div class="flex items-start gap-2">
            <Info class="mt-0.5 h-3.5 w-3.5" />
            <span class="line-clamp-2">备注：{{ noteSummary(form.note) }}</span>
          </div>
        </div>
      </div>
      <div class="flex items-center justify-between border-t border-border bg-muted/35 px-4 py-3 text-xs">
        <div class="flex items-center gap-2">
          <span class="grid h-5 w-5 place-items-center rounded-full bg-primary/10 text-[11px] font-semibold text-primary">
            {{ submitter.avatarText }}
          </span>
          <span>{{ submitter.name }} · 信任等级 {{ submitter.trustLevel ?? '-' }}</span>
        </div>
        <Badge v-if="submitter.verified" variant="verified">
          <CheckCircle2 class="h-3 w-3" />已验证用户
        </Badge>
      </div>
    </div>
    <div class="border-t border-border px-4 py-3 text-xs leading-5 text-muted-foreground">
      前台仅展示昵称、信誉等级与线索内容，不展示联系方式或敏感凭证。
    </div>
  </Card>
</template>
