<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Textarea } from '@/components/ui/textarea'
import PageTitle from '@/components/market/PageTitle.vue'
import SoftTable from '@/components/market/SoftTable.vue'
import StatusTabs from '@/components/market/StatusTabs.vue'
import TablePagination from '@/components/market/TablePagination.vue'
import { usePagination } from '@/composables/usePagination'
import { useReviewCenterRows, useSubmitReviewMutation } from '@/queries/useMarketQueries'
import type { ReviewCenterRow, SubmitReviewPayload } from '@/lib/api'
import { toast } from 'vue-sonner'

const activeStatus = ref('可评价')
const selectedRow = ref<ReviewCenterRow | null>(null)
const { data } = useReviewCenterRows()
const submitReviewMutation = useSubmitReviewMutation()
const form = reactive({
  rating: 5,
  tags: '',
  note: '',
})

const rows = computed(() => (data.value ?? []).filter(item => activeStatus.value === '全部' || item.status === activeStatus.value))
const pagination = usePagination(rows)

function openReview(row: ReviewCenterRow) {
  selectedRow.value = row
  form.rating = row.rating || 5
  form.tags = row.status === '已评价' ? row.tags.join(', ') : ''
  form.note = row.note
}

function submitReview() {
  if (!selectedRow.value) return
  if (!form.note.trim()) {
    toast.warning('请填写评价说明。')
    return
  }
  const payload: SubmitReviewPayload = {
    sourceType: selectedRow.value.sourceType,
    sourceId: selectedRow.value.sourceId,
    rating: form.rating,
    tags: form.tags.split(',').map(item => item.trim()).filter(Boolean).slice(0, 5),
    note: form.note.trim(),
  }
  submitReviewMutation.mutate(payload, {
    onSuccess: () => {
      toast.success('评价已记录。')
      selectedRow.value = null
    },
    onError: error => toast.error(error instanceof Error ? error.message : '评价失败'),
  })
}
</script>

<template>
  <div class="space-y-5">
    <PageTitle title="评价中心" description="仅已完成拼车记录可评价；API 购买意向不进入平台履约评价闭环。" />
    <StatusTabs v-model="activeStatus" :items="['可评价', '已评价', '全部']" />
    <Card class="p-4 text-sm leading-6 text-muted-foreground">
      公开范围：评价默认展示在对应公开主页和相关记录中；API 购买意向不进入平台履约评价闭环。匿名规则：当前不支持匿名评价，提交前请勿写联系方式或敏感凭据。修改规则：已评价记录可再次编辑并覆盖原评价内容。
    </Card>

    <Card v-if="selectedRow" class="p-5">
      <div class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div>
          <h2 class="text-lg font-semibold">评价 {{ selectedRow.target }}</h2>
          <p class="mt-1 text-sm text-muted-foreground">对方：{{ selectedRow.counterparty }} · 拼车申请</p>
        </div>
        <Button variant="outline" @click="selectedRow = null">收起</Button>
      </div>
      <div class="mt-4 grid gap-4 md:grid-cols-[160px_1fr]">
        <label class="space-y-2">
          <span class="text-sm font-medium">评分</span>
          <select v-model.number="form.rating" class="h-9 w-full rounded-md border border-input bg-transparent px-3 text-sm">
            <option :value="5">5 分</option>
            <option :value="4">4 分</option>
            <option :value="3">3 分</option>
          </select>
        </label>
        <label class="space-y-2">
          <span class="text-sm font-medium">体验标签</span>
          <input v-model="form.tags" class="h-9 w-full rounded-md border border-input bg-transparent px-3 text-sm" placeholder="自行填写标签，用逗号分隔" />
        </label>
        <label class="space-y-2 md:col-span-2">
          <span class="text-sm font-medium">评价说明</span>
          <Textarea v-model="form.note" placeholder="描述上车沟通、规则是否清楚、服务是否稳定；不要填写联系方式或敏感凭据。" />
        </label>
      </div>
      <div class="mt-4 flex justify-end">
        <Button :disabled="submitReviewMutation.isPending.value" @click="submitReview">{{ submitReviewMutation.isPending.value ? '提交中' : '提交评价' }}</Button>
      </div>
    </Card>

    <SoftTable :columns="['对象', '对方', '状态', '体验标签', '操作']">
      <tr v-for="item in pagination.paginatedRows.value" :key="item.id">
        <td>
          <div class="font-medium">{{ item.target }}</div>
          <div class="text-xs text-muted-foreground">拼车申请 · {{ item.createdAt }}</div>
        </td>
        <td>{{ item.counterparty }}</td>
        <td><Badge :variant="item.status === '可评价' ? 'default' : 'secondary'">{{ item.status }}</Badge></td>
        <td><div class="flex flex-wrap gap-1"><Badge v-for="tag in item.tags" :key="tag" variant="secondary">{{ tag }}</Badge></div></td>
        <td>
          <Button size="sm" :variant="item.status === '可评价' ? 'default' : 'outline'" @click="openReview(item)">
            {{ item.status === '可评价' ? '去评价' : '查看 / 修改' }}
          </Button>
        </td>
      </tr>
      <tr v-if="rows.length === 0">
        <td colspan="5" class="py-10 text-center text-sm text-muted-foreground">当前没有符合条件的评价记录。</td>
      </tr>
      <template #footer>
        <TablePagination
          v-model:page="pagination.page.value"
          :page-count="pagination.pageCount.value"
          :total="pagination.total.value"
          :start-item="pagination.startItem.value"
          :end-item="pagination.endItem.value"
        />
      </template>
    </SoftTable>
  </div>
</template>
