<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { ArrowDown, ArrowUp, Boxes, Clock3, GripVertical, Package, Pencil, Plus, Trash2 } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import ApiQuotaPolicyFields from '@/components/api-market/ApiQuotaPolicyFields.vue'
import { applyApiServicePackageDraft, cloneApiServicePackageDraft, createDefaultApiServicePackage } from './packages'
import { sellingModeLabels, type ApiServicePackage, type ApiServicePublishForm } from './types'

const props = defineProps<{
  form: ApiServicePublishForm
  errors: Partial<Record<string, string>>
}>()

const packageListRef = ref<HTMLElement | null>(null)
const draggedPackageId = ref<string | null>(null)
const dragOverPackageId = ref<string | null>(null)
const packageEditorOpen = ref(false)
const editingPackageDraft = ref<ApiServicePackage | null>(null)

const editingPackageIndex = computed(() => props.form.packages.findIndex(item => item.id === editingPackageDraft.value?.id))

const setDurationDays = (value: unknown) => {
  if (typeof value !== 'string' && typeof value !== 'number') return
  const durationDays = Number(value)
  if ([1, 3, 7, 30].includes(durationDays)) {
    editingPackageDraft.value!.durationDays = durationDays as 1 | 3 | 7 | 30
  }
}

watch(
  () => props.form.packages.map(item => item.id),
  (packageIds) => {
    if (editingPackageDraft.value && !packageIds.includes(editingPackageDraft.value.id)) {
      packageEditorOpen.value = false
      editingPackageDraft.value = null
    }
  },
)

const openPackageEditor = (packageId: string) => {
  const item = props.form.packages.find(candidate => candidate.id === packageId)
  if (!item) return
  editingPackageDraft.value = cloneApiServicePackageDraft(item)
  packageEditorOpen.value = true
}

const setPackageEditorOpen = (open: boolean) => {
  packageEditorOpen.value = open
  if (!open) editingPackageDraft.value = null
}

const savePackage = () => {
  if (!editingPackageDraft.value) return
  const target = props.form.packages.find(item => item.id === editingPackageDraft.value?.id)
  if (!target) {
    setPackageEditorOpen(false)
    return
  }
  applyApiServicePackageDraft(target, editingPackageDraft.value)
  setPackageEditorOpen(false)
}

const addPackage = async () => {
  const item = createDefaultApiServicePackage(
    props.form.selectedModels.filter(model => model.enabled).map(model => model.modelId),
  )
  props.form.packages.push(item)
  openPackageEditor(item.id)
  await nextTick()
  packageListRef.value
    ?.querySelector<HTMLElement>(`[data-package-id="${item.id}"]`)
    ?.scrollIntoView({ block: 'nearest' })
}

const removePackage = (index: number) => {
  if (props.form.packages.length === 1) return
  const item = props.form.packages[index]
  props.form.packages.splice(index, 1)
  if (editingPackageDraft.value?.id === item?.id) setPackageEditorOpen(false)
}

const movePackage = (index: number, offset: number) => {
  const target = index + offset
  if (target < 0 || target >= props.form.packages.length) return
  const [item] = props.form.packages.splice(index, 1)
  props.form.packages.splice(target, 0, item)
}

const startPackageDrag = (event: DragEvent, packageId: string) => {
  draggedPackageId.value = packageId
  event.dataTransfer?.setData('text/plain', packageId)
  if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move'
}

const dropPackage = (targetIndex: number) => {
  const packageId = draggedPackageId.value
  const sourceIndex = props.form.packages.findIndex(item => item.id === packageId)
  if (sourceIndex < 0 || sourceIndex === targetIndex) {
    endPackageDrag()
    return
  }
  const [item] = props.form.packages.splice(sourceIndex, 1)
  props.form.packages.splice(targetIndex, 0, item)
  endPackageDrag()
}

const endPackageDrag = () => {
  draggedPackageId.value = null
  dragOverPackageId.value = null
}
</script>

<template>
  <Card class="api-publish-card">
    <div class="api-publish-card-header">
      <div class="flex items-center gap-2">
        <Package class="h-4 w-4 text-primary" />
        <h2>{{ sellingModeLabels.package }}</h2>
      </div>
      <p>设置固定价格、面板额度、库存和 1 / 3 / 7 / 30 天有效期。</p>
    </div>

    <div class="api-publish-card-body space-y-4">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <div class="text-sm font-semibold">套餐列表</div>
          <p class="mt-1 text-xs text-muted-foreground">套餐有效期从商家提交交付时开始计算。</p>
        </div>
        <Button size="sm" variant="outline" @click="addPackage"><Plus class="h-4 w-4" />添加套餐</Button>
      </div>

      <p v-if="errors.packages" class="text-xs text-destructive">{{ errors.packages }}</p>
      <div
        ref="packageListRef"
        class="fixed-package-list max-h-[284px] space-y-2 overflow-y-auto overscroll-contain rounded-md border border-border bg-muted/20 p-2"
      >
        <div
          v-for="(item, index) in form.packages"
          :key="item.id"
          :data-package-id="item.id"
          class="group grid min-h-[62px] grid-cols-[minmax(0,1fr)_auto] items-center gap-2 rounded-md border bg-background px-2.5 py-2 text-left transition-colors sm:grid-cols-[auto_minmax(0,1fr)_auto] sm:gap-3 sm:px-3"
          :class="[
            packageEditorOpen && editingPackageDraft?.id === item.id ? 'border-primary/60 bg-primary/5' : 'border-border hover:border-primary/30 hover:bg-muted/30',
            dragOverPackageId === item.id && draggedPackageId !== item.id ? 'border-primary bg-primary/10' : '',
          ]"
          @dragenter.prevent="dragOverPackageId = item.id"
          @dragover.prevent
          @drop.prevent="dropPackage(index)"
        >
          <Button
            variant="ghost"
            draggable="true"
            class="hidden h-8 w-6 cursor-grab rounded p-0 text-muted-foreground hover:bg-muted hover:text-foreground active:cursor-grabbing sm:flex"
            title="拖动排序"
            aria-label="拖动排序"
            @click.stop
            @dragstart.stop="startPackageDrag($event, item.id)"
            @dragend="endPackageDrag"
          >
            <GripVertical class="h-4 w-4" />
          </Button>

          <Button
            variant="ghost"
            class="h-auto min-w-0 justify-start rounded-sm p-0 text-left font-normal hover:bg-transparent"
            :aria-label="`编辑套餐 ${index + 1}：${item.name || '未命名套餐'}`"
            @click="openPackageEditor(item.id)"
          >
            <div class="flex min-w-0 items-center gap-2">
              <span class="truncate text-sm font-medium">{{ item.name || `套餐 ${index + 1}` }}</span>
              <span class="inline-flex shrink-0 items-center gap-1 text-xs" :class="item.enabled ? 'text-emerald-700 dark:text-emerald-400' : 'text-muted-foreground'">
                <span class="h-1.5 w-1.5 rounded-full" :class="item.enabled ? 'bg-emerald-500' : 'bg-muted-foreground/50'" />
                {{ item.enabled ? '启用' : '停用' }}
              </span>
            </div>
            <div class="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
              <span class="font-medium text-foreground">¥{{ item.priceCny }}</span>
              <span>${{ item.panelAllowance }} 额度</span>
              <span class="inline-flex items-center gap-1"><Clock3 class="h-3.5 w-3.5" />{{ item.durationDays }} 天</span>
              <span class="inline-flex items-center gap-1"><Boxes class="h-3.5 w-3.5" />库存 {{ item.stockTotal }}</span>
            </div>
          </Button>

          <div class="flex items-center gap-0.5">
            <Button
              class="sm:hidden"
              size="icon"
              variant="ghost"
              title="上移套餐"
              aria-label="上移套餐"
              :disabled="index === 0"
              @click="movePackage(index, -1)"
            ><ArrowUp class="h-4 w-4" /></Button>
            <Button
              class="sm:hidden"
              size="icon"
              variant="ghost"
              title="下移套餐"
              aria-label="下移套餐"
              :disabled="index === form.packages.length - 1"
              @click="movePackage(index, 1)"
            ><ArrowDown class="h-4 w-4" /></Button>
            <Button size="icon" variant="ghost" title="编辑套餐" aria-label="编辑套餐" @click="openPackageEditor(item.id)"><Pencil class="h-4 w-4" /></Button>
            <Button size="icon" variant="ghost" title="删除套餐" aria-label="删除套餐" :disabled="form.packages.length === 1" @click="removePackage(index)"><Trash2 class="h-4 w-4" /></Button>
          </div>
        </div>
      </div>

    </div>
  </Card>

  <Dialog :open="packageEditorOpen" @update:open="setPackageEditorOpen">
    <DialogContent class="grid max-h-[calc(100dvh-1rem)] grid-rows-[auto_minmax(0,1fr)_auto] gap-0 overflow-hidden p-0 sm:max-w-2xl">
      <DialogHeader class="border-b border-border px-4 py-4 pr-12 text-left sm:px-6 sm:py-5">
        <div class="flex flex-wrap items-center gap-2">
          <DialogTitle>编辑套餐</DialogTitle>
          <Badge v-if="editingPackageIndex >= 0" variant="secondary">套餐 {{ editingPackageIndex + 1 }}</Badge>
        </div>
        <DialogDescription>调整价格、额度、有效期、库存和买家可见说明，保存后应用到当前发布内容。</DialogDescription>
      </DialogHeader>

      <div v-if="editingPackageDraft" class="min-h-0 space-y-4 overflow-y-auto px-4 py-5 sm:px-6">
        <label class="flex items-center justify-between gap-4 rounded-md border border-border bg-muted/25 px-3 py-2.5 text-sm">
          <span>
            <span class="block font-medium">启用该套餐</span>
            <span class="mt-0.5 block text-xs text-muted-foreground">停用后不会继续向买家展示。</span>
          </span>
          <Checkbox
            :model-value="editingPackageDraft.enabled"
            aria-label="启用该套餐"
            @update:model-value="value => editingPackageDraft!.enabled = Boolean(value)"
          />
        </label>

        <p v-if="errors.packages" class="text-xs text-destructive" role="alert">{{ errors.packages }}</p>

        <div class="grid gap-4 sm:grid-cols-2">
          <label class="space-y-1.5 sm:col-span-2">
            <span class="text-xs font-medium">套餐名称</span>
            <Input v-model="editingPackageDraft.name" />
          </label>
          <label class="space-y-1.5">
            <span class="text-xs font-medium">价格（元）</span>
            <Input :model-value="editingPackageDraft.priceCny" min="0.01" step="0.01" type="number" @update:model-value="value => editingPackageDraft!.priceCny = Number(value)" />
          </label>
          <label class="space-y-1.5">
            <span class="text-xs font-medium">美元额度</span>
            <div class="flex overflow-hidden rounded-md border border-input bg-background">
              <span class="grid w-9 shrink-0 place-items-center border-r border-border text-sm text-muted-foreground">$</span>
              <Input class="border-0 shadow-none focus-visible:ring-0" :model-value="editingPackageDraft.panelAllowance" min="0.000001" step="0.01" type="number" @update:model-value="value => editingPackageDraft!.panelAllowance = Number(value)" />
            </div>
          </label>
          <label class="space-y-1.5">
            <span class="text-xs font-medium">有效期</span>
            <Select
              :model-value="String(editingPackageDraft.durationDays)"
              @update:model-value="setDurationDays"
            >
              <SelectTrigger class="w-full"><SelectValue placeholder="选择有效期" /></SelectTrigger>
              <SelectContent>
                <SelectItem value="1">1 天</SelectItem>
                <SelectItem value="3">3 天</SelectItem>
                <SelectItem value="7">7 天</SelectItem>
                <SelectItem value="30">30 天</SelectItem>
              </SelectContent>
            </Select>
          </label>
          <label class="space-y-1.5">
            <span class="text-xs font-medium">总库存</span>
            <Input :model-value="editingPackageDraft.stockTotal" min="0" step="1" type="number" @update:model-value="value => editingPackageDraft!.stockTotal = Number(value)" />
          </label>
          <label class="space-y-1.5 sm:col-span-2">
            <span class="text-xs font-medium">套餐说明</span>
            <Input v-model="editingPackageDraft.description" />
          </label>
        </div>

        <div class="border-t border-border pt-4">
          <ApiQuotaPolicyFields v-model="editingPackageDraft.quotaUsagePolicy" />
        </div>
      </div>

      <DialogFooter class="border-t border-border px-4 py-4 sm:px-6">
        <Button type="button" variant="outline" @click="setPackageEditorOpen(false)">取消</Button>
        <Button type="button" :disabled="!editingPackageDraft" @click="savePackage">保存套餐</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>

<style scoped>
.fixed-package-list {
  scrollbar-color: hsl(var(--border)) transparent;
  scrollbar-gutter: stable;
  scrollbar-width: thin;
}

.fixed-package-list::-webkit-scrollbar {
  width: 8px;
}

.fixed-package-list::-webkit-scrollbar-thumb {
  background: hsl(var(--border));
  border: 2px solid transparent;
  border-radius: 999px;
  background-clip: padding-box;
}
</style>
