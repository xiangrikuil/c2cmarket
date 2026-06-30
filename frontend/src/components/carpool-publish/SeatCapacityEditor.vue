<script setup lang="ts">
import { computed } from 'vue'
import { Minus, Plus } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import type { CarpoolPublishForm } from './types'
import { availableSeats, clampNumber } from './utils'
import PublishSectionCard from './PublishSectionCard.vue'

const props = defineProps<{
  form: CarpoolPublishForm
  errors: Partial<Record<string, string>>
}>()

const remaining = computed(() => availableSeats(props.form))
const seatDots = computed(() => Array.from({ length: props.form.totalSeats }, (_, index) => ({
  index,
  filled: index < props.form.occupiedSeats,
  available: index >= props.form.occupiedSeats,
})))

function setTotalSeats(value: number) {
  props.form.totalSeats = clampNumber(value, 1, 20)
  props.form.occupiedSeats = clampNumber(props.form.occupiedSeats, 0, props.form.totalSeats)
}

function setOccupiedSeats(value: number) {
  props.form.occupiedSeats = clampNumber(value, 0, props.form.totalSeats)
}
</script>

<template>
  <PublishSectionCard
    :index="2"
    title="名额设置"
    description="分别维护总名额和已上车人数，剩余名额由系统自动计算并重点展示。"
  >
    <div class="grid gap-3 xl:grid-cols-[1fr_1fr_1.15fr]">
      <div class="rounded-lg border border-border bg-muted/40 p-3">
        <div class="text-xs text-muted-foreground">总名额</div>
        <div class="mt-2 grid grid-cols-[34px_1fr_34px] items-center gap-2">
          <Button variant="outline" size="icon" :disabled="form.totalSeats <= 1" @click="setTotalSeats(form.totalSeats - 1)"><Minus class="h-4 w-4" /></Button>
          <strong class="text-center text-2xl">{{ form.totalSeats }}</strong>
          <Button variant="outline" size="icon" :disabled="form.totalSeats >= 20" @click="setTotalSeats(form.totalSeats + 1)"><Plus class="h-4 w-4" /></Button>
        </div>
      </div>

      <div class="rounded-lg border border-border bg-muted/40 p-3">
        <div class="text-xs text-muted-foreground">已上车人数</div>
        <div class="mt-2 grid grid-cols-[34px_1fr_34px] items-center gap-2">
          <Button variant="outline" size="icon" :disabled="form.occupiedSeats <= 0" @click="setOccupiedSeats(form.occupiedSeats - 1)"><Minus class="h-4 w-4" /></Button>
          <strong class="text-center text-2xl">{{ form.occupiedSeats }}</strong>
          <Button variant="outline" size="icon" :disabled="form.occupiedSeats >= form.totalSeats" @click="setOccupiedSeats(form.occupiedSeats + 1)"><Plus class="h-4 w-4" /></Button>
        </div>
      </div>

      <div class="rounded-lg border border-primary/25 bg-primary/10 p-3">
        <div class="flex items-start justify-between gap-3">
          <div>
            <div class="text-xs text-muted-foreground">当前剩余名额</div>
            <div class="mt-1 text-2xl font-bold tracking-tight">{{ remaining }} <span class="text-sm font-semibold">席可上车</span></div>
          </div>
          <Badge :variant="remaining > 0 ? 'verified' : 'secondary'">{{ remaining > 0 ? '招募中' : '已满' }}</Badge>
        </div>
        <div class="mt-3 flex gap-1">
          <span
            v-for="dot in seatDots"
            :key="dot.index"
            class="h-2.5 flex-1 rounded-full"
            :class="dot.filled ? 'bg-primary' : 'bg-primary/25'"
          />
        </div>
        <div class="mt-2 flex justify-between text-[11px] text-muted-foreground">
          <span>已上车 {{ form.occupiedSeats }} 人</span>
          <span>剩余 {{ remaining }} 席 / 共 {{ form.totalSeats }} 席</span>
        </div>
      </div>
    </div>

    <div class="mt-3 flex flex-wrap gap-2">
      <Button variant="outline" size="sm" :disabled="form.occupiedSeats >= form.totalSeats" @click="setOccupiedSeats(form.occupiedSeats + 1)">+1 人上车</Button>
      <Button variant="outline" size="sm" :disabled="form.occupiedSeats <= 0" @click="setOccupiedSeats(form.occupiedSeats - 1)">-1 人下车</Button>
      <Button variant="outline" size="sm" @click="setOccupiedSeats(form.totalSeats)">标记已满</Button>
      <Button variant="outline" size="sm" :disabled="form.occupiedSeats <= 0" @click="setOccupiedSeats(Math.max(form.totalSeats - 1, 0))">重新招募</Button>
    </div>
    <p v-if="errors.seats" class="mt-2 text-xs text-destructive">{{ errors.seats }}</p>
  </PublishSectionCard>
</template>
