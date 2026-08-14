<script setup lang="ts">
import type { RouteLocationRaw } from 'vue-router'
import { ArrowLeft } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'

withDefaults(defineProps<{
  title?: string
  description?: string
  backTo?: RouteLocationRaw
  backLabel?: string
}>(), {
  title: '',
  description: '',
  backTo: '',
  backLabel: '返回登录',
})
</script>

<template>
  <main class="auth-page min-h-[100dvh] px-4 py-7 sm:px-6 sm:py-10">
    <div class="mx-auto flex w-full max-w-[456px] flex-col items-center">
      <section class="mb-5 flex flex-col items-center text-center">
        <div class="h-12 w-12 overflow-hidden rounded-lg border border-border/70 shadow-sm">
          <img src="/c2cmarket-icon-512.png?v=20260806-deep-violet" alt="" class="h-full w-full object-cover" />
        </div>
        <h1 class="mt-3 text-2xl font-semibold text-primary">C2CMarket</h1>
        <p class="mt-1 text-sm text-muted-foreground">订阅拼车、API 服务与官网价格市场</p>
      </section>

      <Card class="w-full overflow-hidden rounded-lg border-border bg-card shadow-sm">
        <div v-if="backTo" class="border-b border-border/70 px-4 py-2.5 sm:px-6">
          <Button variant="ghost" size="sm" class="-ml-2 text-muted-foreground" as-child>
            <RouterLink :to="backTo">
              <ArrowLeft class="h-4 w-4" />
              {{ backLabel }}
            </RouterLink>
          </Button>
        </div>

        <div v-if="title || description" class="px-5 pt-5 text-center sm:px-6 sm:pt-6">
          <h2 v-if="title" class="text-xl font-semibold text-foreground">{{ title }}</h2>
          <p v-if="description" class="mt-1.5 text-sm leading-5 text-muted-foreground">{{ description }}</p>
        </div>

        <div class="p-5 sm:p-6">
          <slot />
        </div>
      </Card>

      <p class="mt-4 text-xs text-muted-foreground">© 2026 C2CMarket. All rights reserved.</p>
    </div>
  </main>
</template>

<style scoped>
.auth-page {
  background:
    linear-gradient(180deg, color-mix(in oklch, var(--primary) 5%, var(--background)) 0, var(--background) 34%);
}
</style>
