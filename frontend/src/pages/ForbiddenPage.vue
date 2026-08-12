<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import ShieldX from 'lucide-vue-next/dist/esm/icons/shield-x.js'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { isCapability } from '@/lib/capabilities'

const route = useRoute()
const requiredCapability = computed(() => {
  const value = Array.isArray(route.query.required) ? route.query.required[0] : route.query.required
  return isCapability(value) ? value : null
})
</script>

<template>
  <div class="mx-auto grid min-h-[60vh] max-w-2xl place-items-center py-12">
    <Card class="w-full p-7 text-center sm:p-10">
      <span class="mx-auto grid h-12 w-12 place-items-center rounded-full bg-muted text-muted-foreground">
        <ShieldX class="h-6 w-6" />
      </span>
      <h1 class="mt-5 text-xl font-semibold">当前账号不能使用此功能</h1>
      <p class="mx-auto mt-2 max-w-md text-sm leading-6 text-muted-foreground">
        此入口仅向具备相应能力的账号开放。你仍可继续浏览市场和使用已有买家订单的售后功能。
      </p>
      <p v-if="requiredCapability" class="mt-3 font-mono text-xs text-muted-foreground">
        需要 {{ requiredCapability }}
      </p>
      <div class="mt-6 flex justify-center gap-2">
        <Button as-child><RouterLink to="/">返回首页</RouterLink></Button>
        <Button as-child variant="outline"><RouterLink to="/my/account">查看账号认证</RouterLink></Button>
      </div>
    </Card>
  </div>
</template>
