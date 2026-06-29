<script setup lang="ts">
import { Card } from '@/components/ui/card'
import { Textarea } from '@/components/ui/textarea'
import type { ApiServicePublishForm } from './types'
import { sub2ApiPricingPolicy } from './utils'

defineProps<{
  form: ApiServicePublishForm
  hasImageCapableModel: boolean
  errors: Partial<Record<string, string>>
}>()
</script>

<template>
  <Card class="api-publish-card">
    <div class="api-publish-card-header">
      <h2>5. GPT 图像生成</h2>
      <p>仅在 Sub2API + GPT 时显示；倍率和价格由平台锁定。</p>
    </div>

    <div class="api-publish-card-body space-y-3">
      <button
        type="button"
        class="api-publish-switch-row"
        :disabled="!hasImageCapableModel"
        role="switch"
        :aria-checked="form.imageCapability.enabled"
        @click="form.imageCapability.enabled = !form.imageCapability.enabled"
      >
        <span>
          <strong>支持图像生成</strong>
          <span>{{ hasImageCapableModel ? '开启后在市集与详情页展示图像能力。' : '请先选择支持图像能力的 GPT 模型。' }}</span>
        </span>
        <span class="api-publish-toggle" :class="{ 'is-on': form.imageCapability.enabled }" />
      </button>

      <template v-if="form.imageCapability.enabled">
        <button
          type="button"
          class="api-publish-switch-row"
          role="switch"
          :aria-checked="form.imageCapability.supportsTextToImage"
          @click="form.imageCapability.supportsTextToImage = !form.imageCapability.supportsTextToImage"
        >
          <span>
            <strong>支持文生图</strong>
            <span>买家可按平台只读价格提交文本生成图片请求。</span>
          </span>
          <span class="api-publish-toggle" :class="{ 'is-on': form.imageCapability.supportsTextToImage }" />
        </button>
        <button
          type="button"
          class="api-publish-switch-row"
          role="switch"
          :aria-checked="form.imageCapability.supportsImageToImage"
          @click="form.imageCapability.supportsImageToImage = !form.imageCapability.supportsImageToImage"
        >
          <span>
            <strong>支持图生图</strong>
            <span>买家可按平台只读价格提交图片编辑或图像变体请求。</span>
          </span>
          <span class="api-publish-toggle" :class="{ 'is-on': form.imageCapability.supportsImageToImage }" />
        </button>

        <div class="grid gap-3 md:grid-cols-2">
          <label class="space-y-2">
            <span class="text-sm font-medium">生图倍率</span>
            <div class="rounded-md border border-border bg-muted/50 px-3 py-2 text-sm font-semibold">1.00x（平台锁定）</div>
          </label>
          <label class="space-y-2">
            <span class="text-sm font-medium">价格策略</span>
            <div class="rounded-md border border-border bg-muted/50 px-3 py-2 text-sm font-semibold">平台统一价格</div>
          </label>
        </div>

        <div class="api-publish-image-price-grid">
          <div class="api-publish-image-price">
            <span>1K</span>
            <b>${{ sub2ApiPricingPolicy.imagePrices.resolution1k }} / 张</b>
          </div>
          <div class="api-publish-image-price">
            <span>2K</span>
            <b>${{ sub2ApiPricingPolicy.imagePrices.resolution2k }} / 张</b>
          </div>
          <div class="api-publish-image-price">
            <span>4K</span>
            <b>${{ sub2ApiPricingPolicy.imagePrices.resolution4k }} / 张</b>
          </div>
        </div>

        <label class="space-y-2">
          <span class="text-sm font-medium">图像能力说明</span>
          <Textarea
            :model-value="form.imageCapability.note ?? ''"
            class="min-h-20"
            placeholder="说明图像生成的适用范围、模型维护和临时下线情况。"
            @update:model-value="value => form.imageCapability.note = String(value)"
          />
        </label>
      </template>

      <p v-if="errors.imageCapability" class="text-xs text-destructive">{{ errors.imageCapability }}</p>
    </div>
  </Card>
</template>
