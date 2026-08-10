<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { onBeforeRouteLeave, useRouter } from 'vue-router'
import { Eye, Save, Send } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import AnnouncementDetailContent from '@/components/announcements/AnnouncementDetailContent.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import {
  announcementCategoryLabels,
  announcementChannelLabels,
  announcementLevelLabels,
  announcementStatusLabels,
  announcementToFormInput,
  createDefaultAnnouncementFormInput,
  formatAnnouncementDateTime,
  fromDateTimeLocalValue,
  getAnnouncementDisplayStatus,
  toDateTimeLocalValue,
  validateAnnouncementFormInput,
} from '@/lib/announcementUtils'
import {
  useCreateAnnouncement,
  usePublishAnnouncement,
  useUpdateAnnouncement,
} from '@/queries/useAnnouncementQueries'
import type {
  Announcement,
  AnnouncementCategory,
  AnnouncementChannel,
  AnnouncementFormInput,
  AnnouncementLevel,
} from '@/types/announcement'

type EditorMode = 'create' | 'edit'
type EditorField =
  | 'title'
  | 'summary'
  | 'contentMarkdown'
  | 'category'
  | 'level'
  | 'channels'
  | 'publishAt'
  | 'expireAt'
  | 'ctaLabel'
  | 'ctaUrl'
  | 'preview'

const props = defineProps<{
  mode: EditorMode
  announcement?: Announcement | null
}>()

const router = useRouter()
const createMutation = useCreateAnnouncement()
const updateMutation = useUpdateAnnouncement()
const publishMutation = usePublishAnnouncement()
const previewVisible = ref(false)
const hasActionButton = ref(false)
const lastSavedId = ref('')
const initialSnapshot = ref('')
const errors = reactive<Partial<Record<EditorField, string>>>({})

const form = reactive({
  title: '',
  summary: '',
  contentMarkdown: '',
  category: 'platform' as AnnouncementCategory,
  level: 'normal' as AnnouncementLevel,
  homeBanner: false,
  isPinned: false,
  isDismissible: true,
  publishAtLocal: '',
  expireAtLocal: '',
  ctaLabel: '',
  ctaUrl: '',
})

const categoryOptions = Object.entries(announcementCategoryLabels) as Array<[AnnouncementCategory, string]>
const levelOptions = Object.entries(announcementLevelLabels) as Array<[AnnouncementLevel, string]>
const currentStatus = computed(() => props.announcement ? getAnnouncementDisplayStatus(props.announcement) : 'draft')
const currentStatusLabel = computed(() => announcementStatusLabels[currentStatus.value])
const editingPublished = computed(() => currentStatus.value === 'published')
const isBusy = computed(() => createMutation.isPending.value || updateMutation.isPending.value || publishMutation.isPending.value)
const effectiveAnnouncementId = computed(() => props.announcement?.id ?? lastSavedId.value)
const previewInput = computed(() => buildInput())
const hasErrors = computed(() => Object.values(errors).some(Boolean))
const dirty = computed(() => serializeForm() !== initialSnapshot.value)

watch(
  () => props.announcement,
  announcement => {
    if (props.mode === 'edit' && !announcement) return
    resetForm(announcement ? announcementToFormInput(announcement) : createDefaultAnnouncementFormInput())
    lastSavedId.value = announcement?.id ?? ''
  },
  { immediate: true },
)

function resetForm(input: AnnouncementFormInput) {
  form.title = input.title
  form.summary = input.summary
  form.contentMarkdown = input.contentMarkdown
  form.category = input.category
  form.level = input.level
  form.homeBanner = input.channels.includes('home_banner')
  form.isPinned = input.isPinned
  form.isDismissible = input.isDismissible
  form.publishAtLocal = toDateTimeLocalValue(input.publishAt)
  form.expireAtLocal = toDateTimeLocalValue(input.expireAt)
  form.ctaLabel = input.ctaLabel ?? ''
  form.ctaUrl = input.ctaUrl ?? ''
  hasActionButton.value = Boolean(input.ctaLabel || input.ctaUrl)
  previewVisible.value = false
  clearErrors()
  initialSnapshot.value = serializeForm()
}

function buildInput(): AnnouncementFormInput {
  const channels: AnnouncementChannel[] = ['message_center']
  if (form.homeBanner) channels.push('home_banner')
  return {
    title: form.title,
    summary: form.summary,
    contentMarkdown: form.contentMarkdown,
    category: form.category,
    level: form.level,
    channels,
    isPinned: form.isPinned,
    isDismissible: form.isDismissible,
    publishAt: fromDateTimeLocalValue(form.publishAtLocal),
    expireAt: form.expireAtLocal ? fromDateTimeLocalValue(form.expireAtLocal) : undefined,
    ctaLabel: hasActionButton.value ? form.ctaLabel.trim() || undefined : undefined,
    ctaUrl: hasActionButton.value ? form.ctaUrl.trim() || undefined : undefined,
  }
}

function validateForm(requirePreview: boolean) {
  clearErrors()
  const input = buildInput()
  const result = validateAnnouncementFormInput(input)
  Object.assign(errors, result.errors)
  if (hasActionButton.value && !input.ctaLabel) errors.ctaLabel = '请输入按钮文案。'
  if (hasActionButton.value && !input.ctaUrl) errors.ctaUrl = '请输入跳转链接。'
  if (requirePreview && !previewVisible.value) {
    errors.preview = '发布前必须先打开预览并核对内容。'
  }
  return !hasErrors.value
}

async function saveDraft() {
  if (!validateForm(false)) {
    toast.warning('请先修正公告表单。')
    return null
  }

  try {
    const input = buildInput()
    const existingId = effectiveAnnouncementId.value
    const saved = existingId
      ? await updateMutation.mutateAsync({ id: effectiveAnnouncementId.value, input })
      : await createMutation.mutateAsync(input)
    lastSavedId.value = saved.id
    initialSnapshot.value = serializeForm()
    toast.success(existingId ? '公告草稿已保存。' : '公告草稿已创建。')
    return saved
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '公告保存失败')
    return null
  }
}

async function publish() {
  if (!validateForm(true)) {
    toast.warning(errors.preview ?? '请先修正公告表单。')
    return
  }

  if (editingPublished.value) {
    if (!dirty.value) {
      toast.warning('发布中公告没有未保存修改，不能重复执行发布。')
      return
    }
    const saved = await saveDraft()
    if (!saved) return
    toast.success('发布中公告已更新。')
    router.push('/admin/announcements')
    return
  }

  const saved = await saveDraft()
  if (!saved) return

  try {
    const published = await publishMutation.mutateAsync(saved.id)
    initialSnapshot.value = serializeForm()
    toast.success(getAnnouncementDisplayStatus(published) === 'scheduled' ? '公告已保存为待发布。' : '公告已发布。')
    router.push('/admin/announcements')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '公告发布失败')
  }
}

function showPreview() {
  if (!validateForm(false)) {
    toast.warning('请先修正公告表单。')
    return
  }
  previewVisible.value = true
}

function clearErrors() {
  for (const key of Object.keys(errors) as EditorField[]) delete errors[key]
}

function setHasActionButton(value: boolean) {
  hasActionButton.value = value
  if (value) return
  form.ctaLabel = ''
  form.ctaUrl = ''
  delete errors.ctaLabel
  delete errors.ctaUrl
}

function serializeForm() {
  return JSON.stringify({
    title: form.title,
    summary: form.summary,
    contentMarkdown: form.contentMarkdown,
    category: form.category,
    level: form.level,
    homeBanner: form.homeBanner,
    isPinned: form.isPinned,
    isDismissible: form.isDismissible,
    publishAtLocal: form.publishAtLocal,
    expireAtLocal: form.expireAtLocal,
    hasActionButton: hasActionButton.value,
    ctaLabel: form.ctaLabel,
    ctaUrl: form.ctaUrl,
  })
}

function beforeUnload(event: BeforeUnloadEvent) {
  if (!dirty.value) return
  event.preventDefault()
  event.returnValue = ''
}

if (typeof window !== 'undefined') {
  window.addEventListener('beforeunload', beforeUnload)
}

onBeforeUnmount(() => {
  if (typeof window !== 'undefined') window.removeEventListener('beforeunload', beforeUnload)
})

onBeforeRouteLeave(() => {
  if (!dirty.value) return true
  return window.confirm('公告内容尚未保存，确认离开当前页面？')
})
</script>

<template>
  <div class="grid gap-5 xl:grid-cols-[minmax(0,1.15fr)_minmax(360px,0.85fr)]">
    <Card class="p-5">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 class="font-semibold">公告内容</h2>
          <p class="mt-1 text-sm text-muted-foreground">公告保存为草稿后才会进入管理列表；发布前必须先预览。</p>
        </div>
        <Badge :variant="editingPublished ? 'default' : 'secondary'">{{ currentStatusLabel }}</Badge>
      </div>

      <div class="mt-5 grid gap-4">
        <label class="space-y-2">
          <span class="text-sm font-medium">标题</span>
          <Input v-model="form.title" maxlength="80" placeholder="例如：API 服务发布规范已调整" />
          <p v-if="errors.title" class="text-xs text-destructive">{{ errors.title }}</p>
        </label>

        <label class="space-y-2">
          <span class="text-sm font-medium">摘要</span>
          <Textarea v-model="form.summary" class="min-h-20" maxlength="160" placeholder="用一两句话说明公告影响范围和用户需要知道的变化。" />
          <p v-if="errors.summary" class="text-xs text-destructive">{{ errors.summary }}</p>
        </label>

        <div class="grid gap-4 md:grid-cols-2">
          <label class="space-y-2">
            <span class="text-sm font-medium">分类</span>
            <Select v-model="form.category">
              <SelectTrigger class="w-full bg-background"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem v-for="[value, label] in categoryOptions" :key="value" :value="value">{{ label }}</SelectItem>
              </SelectContent>
            </Select>
            <p v-if="errors.category" class="text-xs text-destructive">{{ errors.category }}</p>
          </label>

          <label class="space-y-2">
            <span class="text-sm font-medium">级别</span>
            <Select v-model="form.level">
              <SelectTrigger class="w-full bg-background"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem v-for="[value, label] in levelOptions" :key="value" :value="value">{{ label }}</SelectItem>
              </SelectContent>
            </Select>
            <p v-if="errors.level" class="text-xs text-destructive">{{ errors.level }}</p>
          </label>
        </div>

        <div class="grid gap-3 rounded-md border border-border bg-muted/30 p-4 md:grid-cols-2">
          <div class="flex items-start gap-3 text-sm">
            <Checkbox :model-value="true" disabled class="mt-1" aria-label="公告中心为必选展示渠道" />
            <span>
              <span class="font-medium">{{ announcementChannelLabels.message_center }}</span>
              <span class="mt-0.5 block text-xs text-muted-foreground">公告中心为必选展示渠道。</span>
            </span>
          </div>
          <div class="flex items-start gap-3 text-sm">
            <Switch id="announcement-home-banner" v-model="form.homeBanner" class="mt-0.5" />
            <label for="announcement-home-banner" class="cursor-pointer">
              <span class="font-medium">{{ announcementChannelLabels.home_banner }}</span>
              <span class="mt-0.5 block text-xs text-muted-foreground">只有发布中且未结束公告会进入首页候选。</span>
            </label>
          </div>
          <div class="flex items-start gap-3 text-sm">
            <Switch id="announcement-pinned" v-model="form.isPinned" class="mt-0.5" />
            <label for="announcement-pinned" class="cursor-pointer">
              <span class="font-medium">置顶</span>
              <span class="mt-0.5 block text-xs text-muted-foreground">置顶影响首页候选和公告列表排序展示。</span>
            </label>
          </div>
          <div class="flex items-start gap-3 text-sm">
            <Switch id="announcement-dismissible" v-model="form.isDismissible" class="mt-0.5" />
            <label for="announcement-dismissible" class="cursor-pointer">
              <span class="font-medium">允许关闭首页公告</span>
              <span class="mt-0.5 block text-xs text-muted-foreground">重要公告通常不允许关闭。</span>
            </label>
          </div>
          <p v-if="errors.channels" class="text-xs text-destructive md:col-span-2">{{ errors.channels }}</p>
        </div>

        <div class="grid gap-4 md:grid-cols-2">
          <label class="space-y-2">
            <span class="text-sm font-medium">发布时间</span>
            <Input v-model="form.publishAtLocal" type="datetime-local" />
            <p v-if="errors.publishAt" class="text-xs text-destructive">{{ errors.publishAt }}</p>
          </label>

          <label class="space-y-2">
            <span class="text-sm font-medium">结束时间</span>
            <Input v-model="form.expireAtLocal" type="datetime-local" />
            <p v-if="errors.expireAt" class="text-xs text-destructive">{{ errors.expireAt }}</p>
          </label>
        </div>

        <div class="rounded-md border border-border bg-muted/30 p-4">
          <div class="flex items-center justify-between gap-4">
            <label for="announcement-action-button" class="cursor-pointer text-sm font-medium">添加跳转按钮</label>
            <Switch
              id="announcement-action-button"
              :model-value="hasActionButton"
              :aria-expanded="hasActionButton"
              aria-controls="announcement-action-button-fields"
              @update:model-value="value => setHasActionButton(Boolean(value))"
            />
          </div>

          <div v-if="hasActionButton" id="announcement-action-button-fields" class="mt-4 grid gap-4 border-t border-border pt-4 md:grid-cols-2">
            <label class="space-y-2">
              <span class="text-sm font-medium">按钮文案</span>
              <Input v-model="form.ctaLabel" placeholder="查看 API 集市" />
              <p v-if="errors.ctaLabel" class="text-xs text-destructive">{{ errors.ctaLabel }}</p>
            </label>

            <label class="space-y-2">
              <span class="text-sm font-medium">跳转链接</span>
              <Input v-model="form.ctaUrl" inputmode="url" placeholder="/api-market 或 https://linux.do/..." />
              <p v-if="errors.ctaUrl" class="text-xs text-destructive">{{ errors.ctaUrl }}</p>
            </label>
          </div>
        </div>

        <label class="space-y-2">
          <span class="text-sm font-medium">正文 Markdown</span>
          <Textarea v-model="form.contentMarkdown" class="min-h-72 font-mono text-sm" placeholder="## 公告标题&#10;&#10;- 说明一&#10;- 说明二" />
          <p v-if="errors.contentMarkdown" class="text-xs text-destructive">{{ errors.contentMarkdown }}</p>
        </label>
      </div>

      <div class="mt-5 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <p class="text-xs text-muted-foreground">
          <span v-if="dirty">有未保存修改。</span>
          <span v-else>当前表单已保存。</span>
          <span v-if="errors.preview" class="ml-2 text-destructive">{{ errors.preview }}</span>
        </p>
        <div class="flex flex-wrap gap-2">
          <Button type="button" variant="outline" :disabled="isBusy" @click="showPreview">
            <Eye class="h-4 w-4" />
            预览
          </Button>
          <Button type="button" variant="outline" :disabled="isBusy" @click="saveDraft">
            <Save class="h-4 w-4" />
            保存草稿
          </Button>
          <Button type="button" :disabled="isBusy" @click="publish">
            <Send class="h-4 w-4" />
            发布
          </Button>
        </div>
      </div>
    </Card>

    <div class="space-y-5">
      <Card class="p-5">
        <h2 class="font-semibold">发布预览</h2>
        <p class="mt-1 text-sm text-muted-foreground">预览使用与用户端公告详情相同的 Markdown 清洗和渲染逻辑。</p>

        <div v-if="previewVisible" class="mt-5 space-y-4">
          <div class="flex flex-wrap items-center gap-2">
            <Badge variant="outline">{{ announcementCategoryLabels[previewInput.category] }}</Badge>
            <Badge :variant="previewInput.level === 'important' ? 'default' : 'secondary'">{{ announcementLevelLabels[previewInput.level] }}</Badge>
            <Badge v-if="previewInput.isPinned" variant="secondary">置顶</Badge>
          </div>
          <div>
            <h3 class="text-xl font-semibold tracking-tight">{{ previewInput.title }}</h3>
            <p class="mt-2 text-sm leading-6 text-muted-foreground">{{ previewInput.summary }}</p>
          </div>
          <div class="grid gap-2 rounded-md border border-border bg-muted/30 p-3 text-xs text-muted-foreground">
            <div>发布时间：{{ formatAnnouncementDateTime(previewInput.publishAt) }}</div>
            <div>结束时间：{{ formatAnnouncementDateTime(previewInput.expireAt) }}</div>
            <div>展示渠道：{{ previewInput.channels.map(channel => announcementChannelLabels[channel]).join('、') }}</div>
          </div>
          <AnnouncementDetailContent :content-markdown="previewInput.contentMarkdown" />
          <div v-if="previewInput.ctaLabel && previewInput.ctaUrl">
            <Button size="sm" variant="outline">{{ previewInput.ctaLabel }}</Button>
          </div>
        </div>

        <div v-else class="mt-5 rounded-md border border-dashed border-border p-6 text-center text-sm text-muted-foreground">
          点击“预览”后显示公告详情效果。发布动作会要求先完成预览。
        </div>
      </Card>

      <Card class="p-5">
        <h2 class="font-semibold">状态规则</h2>
        <div class="mt-3 space-y-2 text-sm leading-6 text-muted-foreground">
          <p>发布时间晚于当前时间时，发布后显示为待发布。</p>
          <p>结束时间晚于发布时间才允许保存；到达结束时间后用户端仍可在公告历史查看。</p>
          <p>发布中公告再次编辑会写入审计记录；发布中记录不能重复执行无意义发布。</p>
        </div>
      </Card>
    </div>
  </div>
</template>
