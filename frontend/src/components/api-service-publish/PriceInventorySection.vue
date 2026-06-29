<script setup lang="ts">
import { computed } from 'vue'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import type { ApiServicePublishForm, UsageVisibility } from './types'
import { formatUsdQuotaForCny, usageLabels } from './utils'

const props = defineProps<{
  form: ApiServicePublishForm
  allowedUsage: UsageVisibility[]
  errors: Partial<Record<string, string>>
}>()

const dayPresets = [7, 15, 30, 60, 90]
const sub2QuotaForMinimumPurchase = computed(() => formatUsdQuotaForCny(props.form.cnyPerUsdCredit, props.form.minimumPurchaseCny ?? 0))
</script>

<template>
  <Card class="api-publish-card">
    <div class="api-publish-card-header">
      <h2>6. 库存、最低意向与有效期</h2>
      <p>最低意向金额统一使用人民币“元”；用量查看按分发系统和接入方式自动限制。</p>
    </div>

    <div class="api-publish-card-body grid gap-4 md:grid-cols-2 xl:grid-cols-4">
      <label v-if="form.billingMode !== 'fixed_package' && form.distributionSystem === 'sub2api'" class="space-y-2">
        <span class="text-sm font-medium">最大可售美元额度</span>
        <Input
          :model-value="form.availableCreditUsd ?? ''"
          placeholder="500"
          @update:model-value="value => form.availableCreditUsd = Number(value)"
        />
        <p v-if="errors.availableCreditUsd" class="text-xs text-destructive">{{ errors.availableCreditUsd }}</p>
        <p v-else class="text-xs text-muted-foreground">表示买家最多可向商户购买的美元额度参考，不是平台余额。</p>
      </label>
      <label v-if="form.billingMode !== 'fixed_package'" class="space-y-2">
        <span class="text-sm font-medium">最低意向金额</span>
        <div class="flex overflow-hidden rounded-md border border-input bg-background">
          <Input
            :model-value="form.minimumPurchaseCny ?? ''"
            class="border-0 shadow-none focus-visible:ring-0"
            placeholder="20"
            @update:model-value="value => form.minimumPurchaseCny = Number(value)"
          />
          <span class="grid w-12 place-items-center border-l border-border text-sm text-muted-foreground">元</span>
        </div>
        <p v-if="form.distributionSystem === 'sub2api'" class="text-xs text-muted-foreground">当前起步意向对应约 {{ sub2QuotaForMinimumPurchase }}，最终金额和接入由双方站外确认。</p>
        <p v-if="errors.minimumPurchaseCny" class="text-xs text-destructive">{{ errors.minimumPurchaseCny }}</p>
      </label>
      <label v-if="form.billingMode !== 'fixed_package'" class="space-y-2">
        <span class="text-sm font-medium">单笔最高意向（可选）</span>
        <Input
          :model-value="form.maximumPurchaseCny ?? ''"
          placeholder="300"
          @update:model-value="value => form.maximumPurchaseCny = String(value).trim() ? Number(value) : null"
        />
      </label>

      <div class="space-y-2">
        <label class="text-sm font-medium">站外确认后有效期</label>
        <div class="flex flex-wrap gap-2">
          <button
            v-for="days in dayPresets"
            :key="days"
            type="button"
            class="rounded-md border px-3 py-2 text-sm"
            :class="form.validity.mode === 'days' && form.validity.days === days ? 'border-primary bg-primary/10 text-primary' : 'border-border bg-background hover:bg-muted'"
            @click="form.validity.mode = 'days'; form.validity.days = days"
          >
            {{ days }} 天
          </button>
          <button
            type="button"
            class="rounded-md border px-3 py-2 text-sm"
            :class="form.validity.mode === 'permanent' ? 'border-primary bg-primary/10 text-primary' : 'border-border bg-background hover:bg-muted'"
            @click="form.validity.mode = 'permanent'; form.validity.days = null"
          >
            永久
          </button>
        </div>
        <Input
          v-if="form.validity.mode === 'days'"
          :model-value="form.validity.days ?? ''"
          placeholder="自定义天数"
          @update:model-value="value => form.validity.days = Number(value)"
        />
        <p v-if="errors.validity" class="text-xs text-destructive">{{ errors.validity }}</p>
      </div>

      <label class="space-y-2 md:col-span-2 xl:col-span-4">
        <span class="text-sm font-medium">服务补充短句（可选）</span>
        <Input v-model="form.shortDescription" maxlength="40" placeholder="面板实时用量，建议首次小额测试" />
        <p class="text-xs text-muted-foreground">{{ form.shortDescription.length }} / 40 字，不参与自动标题生成。</p>
      </label>

      <label v-if="form.distributionSystem !== 'sub2api' && form.billingMode === 'manual_credit'" class="block space-y-2 md:col-span-2 xl:col-span-4">
        <span class="text-sm font-medium">计费说明与用量核对方式</span>
        <Input
          :model-value="form.manualBillingNote"
          placeholder="例如：按商户后台用量截图核对，结算周期为每日确认"
          @update:model-value="value => form.manualBillingNote = String(value)"
        />
        <p class="text-xs text-muted-foreground">该服务无法由平台统一核验精确美元余额。</p>
        <p v-if="errors.manualBillingNote" class="text-xs text-destructive">{{ errors.manualBillingNote }}</p>
      </label>

      <div v-if="form.billingMode === 'fixed_package'" class="space-y-3 md:col-span-2 xl:col-span-4">
        <div v-for="item in form.packages" :key="item.id" class="grid gap-3 rounded-lg border border-border bg-background p-3 md:grid-cols-[1fr_110px_1fr_90px]">
          <Input v-model="item.name" placeholder="套餐名称" />
          <Input :model-value="item.priceCny" placeholder="价格" @update:model-value="value => item.priceCny = Number(value)" />
          <Input v-model="item.description" placeholder="套餐说明" />
          <Input :model-value="item.inventory ?? ''" placeholder="库存" @update:model-value="value => item.inventory = String(value).trim() ? Number(value) : null" />
        </div>
        <p v-if="errors.packages" class="text-xs text-destructive">{{ errors.packages }}</p>
      </div>

      <div class="space-y-2 md:col-span-2 xl:col-span-4">
        <label class="text-sm font-medium">用量与余额查看</label>
        <div class="grid gap-2 md:grid-cols-2">
          <button
            v-for="option in allowedUsage"
            :key="option"
            type="button"
            class="rounded-md border px-3 py-2 text-left text-sm"
            :class="form.usageVisibility === option ? 'border-primary bg-primary/10 text-primary' : 'border-border bg-background hover:bg-muted'"
            @click="form.usageVisibility = option"
          >
            {{ usageLabels[option] }}
          </button>
        </div>
        <p v-if="errors.usageVisibility" class="text-xs text-destructive">{{ errors.usageVisibility }}</p>
        <p class="text-xs text-muted-foreground">根据分发系统、计费方式和接入方式联动限制，不能任意声明不真实能力。</p>
      </div>
    </div>
  </Card>
</template>
