<script setup lang="ts">
import { RouterLink } from 'vue-router'
import { ExternalLink, UserRound } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import type { ApiServicePublishForm } from './types'

defineProps<{
  form: ApiServicePublishForm
  profileLoading: boolean
  displayNameStatus: string
  error?: string
}>()

const emit = defineEmits<{
  setStoreAliasVisible: [value: boolean]
}>()
</script>

<template>
  <Card class="api-publish-card">
    <div class="api-publish-card-header">
      <div class="flex items-start gap-2">
        <span class="grid h-7 w-7 shrink-0 place-items-center rounded-md bg-sky-50 text-sky-600">
          <UserRound class="h-4 w-4" />
        </span>
        <div>
          <h2>展示身份</h2>
          <p>默认只展示商家展示名，订单后再站外确认接入细节。</p>
        </div>
      </div>
    </div>
    <div class="api-publish-card-body space-y-3">
      <label class="flex items-start gap-2 text-sm">
        <Checkbox
          :model-value="form.merchantIdentityMode === 'store_alias'"
          class="mt-0.5"
          @update:model-value="value => emit('setStoreAliasVisible', Boolean(value))"
        />
        <span>
          不公开社区身份，仅展示商家展示名
          <span class="mt-0.5 block text-xs text-muted-foreground">买家仍可看到已绑定 linux.do、信任等级、交易评价与纠纷记录。</span>
        </span>
      </label>
      <div
        v-if="form.merchantIdentityMode === 'store_alias'"
        class="max-w-md rounded-md border border-border bg-muted/35 px-3 py-2"
        :aria-invalid="Boolean(error)"
        :aria-describedby="error ? 'api-publish-merchant-display-name-error' : undefined"
        :tabindex="error ? -1 : undefined"
      >
        <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <div class="min-w-0">
            <div class="text-xs font-medium text-muted-foreground">商家展示名</div>
            <div class="mt-1 truncate text-sm font-semibold">{{ form.merchantDisplayName || (profileLoading ? '正在读取个人资料...' : '待设置显示名称') }}</div>
          </div>
          <RouterLink to="/my/profile" class="shrink-0">
            <Button size="sm" variant="outline">
              去个人资料修改 <ExternalLink class="h-3.5 w-3.5" />
            </Button>
          </RouterLink>
        </div>
        <p v-if="error" id="api-publish-merchant-display-name-error" class="text-xs text-destructive" role="alert">{{ error }}</p>
        <p v-else class="text-xs leading-5 text-muted-foreground">{{ displayNameStatus }}</p>
      </div>
    </div>
  </Card>
</template>
