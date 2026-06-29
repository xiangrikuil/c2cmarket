<script setup lang="ts">
defineProps<{
  sampleCount: number
}>()
</script>

<template>
  <div class="grid min-h-[330px] place-items-center bg-background px-6 text-center">
    <div>
      <div class="text-base font-semibold">有效完成样本不足，暂不生成趋势</div>
      <div class="mt-2 text-sm text-muted-foreground">当前已有 {{ sampleCount }} 条有效完成记录。</div>
    </div>
  </div>
</template>
