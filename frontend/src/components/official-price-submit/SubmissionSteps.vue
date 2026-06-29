<script setup lang="ts">
const steps = ['填写线索', '自动校验', '管理员审核', '前台展示']
</script>

<template>
  <div class="overflow-x-auto rounded-xl border border-border bg-card px-3 py-2 shadow-sm">
    <ol class="flex min-w-max items-center gap-3 text-xs">
      <li v-for="(step, index) in steps" :key="step" class="flex items-center gap-3">
        <div class="flex items-center gap-2">
          <span
            class="grid h-6 w-6 place-items-center rounded-full text-xs font-semibold"
            :class="index === 0 ? 'bg-primary text-primary-foreground' : 'bg-muted text-muted-foreground'"
          >
            {{ index + 1 }}
          </span>
          <span :class="index === 0 ? 'font-semibold text-primary' : 'text-muted-foreground'">{{ step }}</span>
        </div>
        <span v-if="index < steps.length - 1" class="h-px w-10 bg-border"></span>
      </li>
    </ol>
  </div>
</template>
