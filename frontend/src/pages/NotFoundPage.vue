<script setup lang="ts">
import { ArrowLeft, Home, Search } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'

function goBack() {
  window.history.length > 1 ? window.history.back() : window.location.assign('/')
}
</script>

<template>
  <div class="grid min-h-[64vh] place-items-center">
    <Card class="w-full max-w-2xl p-8 text-center">
      <div class="mx-auto grid h-14 w-14 place-items-center rounded-xl bg-accent">
        <Search class="h-7 w-7 text-primary" />
      </div>
      <h1 class="mt-5 text-3xl font-semibold tracking-tight">页面不存在</h1>
      <p class="mt-3 text-sm leading-6 text-muted-foreground">该地址没有匹配的前端路由，可能是 mock 记录尚未创建、已下架，或链接输入有误。</p>
      <div class="mt-6 flex flex-wrap justify-center gap-2">
        <Button variant="outline" @click="goBack"><ArrowLeft class="h-4 w-4" />返回上一页</Button>
        <RouterLink to="/"><Button><Home class="h-4 w-4" />回到首页</Button></RouterLink>
        <RouterLink to="/search"><Button variant="outline"><Search class="h-4 w-4" />全局搜索</Button></RouterLink>
      </div>
    </Card>
  </div>
</template>
