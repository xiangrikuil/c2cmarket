<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft } from 'lucide-vue-next'
import AnnouncementEditor from '@/components/announcements/AnnouncementEditor.vue'
import PageTitle from '@/components/market/PageTitle.vue'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { useAdminAnnouncement } from '@/queries/useAnnouncementQueries'

const route = useRoute()
const router = useRouter()
const isCreate = computed(() => route.name === 'admin-announcement-new')
const announcementId = computed(() => isCreate.value ? '' : String(route.params.id ?? ''))
const { data: announcement, isLoading } = useAdminAnnouncement(announcementId)
const pageTitle = computed(() => isCreate.value ? '新建公告' : '编辑公告')
const pageDescription = computed(() => isCreate.value
  ? '创建平台公告草稿，预览后可以立即发布或设置未来发布时间。'
  : '编辑公告内容、展示位置、发布时间和 CTA；编辑发布中公告会写入审计记录。')
</script>

<template>
  <div class="space-y-5">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <PageTitle :title="pageTitle" :description="pageDescription" />
      <Button variant="outline" class="w-full sm:w-auto" @click="router.push('/admin/announcements')">
        <ArrowLeft class="h-4 w-4" />
        返回公告管理
      </Button>
    </div>

    <Card v-if="!isCreate && isLoading" class="p-6 text-sm text-muted-foreground">
      公告加载中...
    </Card>

    <Card v-else-if="!isCreate && !announcement" class="p-8 text-center">
      <h2 class="text-xl font-semibold">公告不存在</h2>
      <p class="mt-2 text-sm text-muted-foreground">该公告可能已被移除，或当前链接参数有误。</p>
      <div class="mt-5">
        <Button variant="outline" @click="router.push('/admin/announcements')">
          <ArrowLeft class="h-4 w-4" />
          返回公告管理
        </Button>
      </div>
    </Card>

    <AnnouncementEditor v-else :mode="isCreate ? 'create' : 'edit'" :announcement="announcement" />
  </div>
</template>
