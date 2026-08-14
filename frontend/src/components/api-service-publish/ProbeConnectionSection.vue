<script setup lang="ts">
import { computed } from 'vue'
import { Cable, ExternalLink, RefreshCw } from 'lucide-vue-next'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import type { OwnerAPIProbeConnection } from '@/types/apiHealth'

const props = defineProps<{
  modelValue: string
  connections: OwnerAPIProbeConnection[]
  loading?: boolean
  error?: string
  fieldError?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
  refresh: []
}>()

const readyConnections = computed(() => props.connections.filter(connection => (
  connection.enabled && connection.verificationStatus === 'verified'
)))

const selectedConnection = computed(() => props.connections.find(connection => connection.id === props.modelValue) ?? null)

function selectConnection(value: unknown) {
  emit('update:modelValue', String(value ?? ''))
}
</script>

<template>
  <Card class="api-publish-card">
    <div class="api-publish-card-header">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div class="flex items-start gap-2">
          <span class="grid h-7 w-7 shrink-0 place-items-center rounded-md bg-sky-50 text-sky-700"><Cable class="h-4 w-4" /></span>
          <div>
            <h2>探针连接</h2>
            <p>选择已验证且启用的共享连接。</p>
          </div>
        </div>
        <Badge v-if="selectedConnection" :variant="selectedConnection.enabled && selectedConnection.verificationStatus === 'verified' ? 'verified' : 'status'">
          {{ selectedConnection.enabled && selectedConnection.verificationStatus === 'verified' ? '连接可用' : '连接不可用' }}
        </Badge>
      </div>
    </div>

    <div class="api-publish-card-body space-y-3">
      <div v-if="loading" class="flex items-center gap-2 text-sm text-muted-foreground">
        <RefreshCw class="h-4 w-4 animate-spin" />正在读取探针连接...
      </div>

      <Alert v-else-if="error" variant="destructive">
        <AlertTitle>无法读取探针连接</AlertTitle>
        <AlertDescription class="flex flex-wrap items-center justify-between gap-2">
          <span>{{ error }}</span>
          <Button size="sm" variant="outline" type="button" @click="emit('refresh')"><RefreshCw class="h-4 w-4" />重试</Button>
        </AlertDescription>
      </Alert>

      <template v-else>
        <Select :model-value="modelValue || undefined" @update:model-value="selectConnection">
          <SelectTrigger :aria-invalid="Boolean(fieldError)" class="w-full">
            <SelectValue placeholder="选择探针连接" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem v-for="connection in readyConnections" :key="connection.id" :value="connection.id">
              {{ connection.name }} · {{ connection.baseUrl }}
            </SelectItem>
          </SelectContent>
        </Select>
        <p v-if="fieldError" class="text-xs text-destructive" role="alert">{{ fieldError }}</p>
        <p v-else-if="selectedConnection" class="break-all font-mono text-xs text-muted-foreground">{{ selectedConnection.baseUrl }}</p>

        <Alert v-if="readyConnections.length === 0">
          <AlertTitle>暂无可绑定连接</AlertTitle>
          <AlertDescription>先创建连接并完成鉴权验证，再返回继续发布。</AlertDescription>
        </Alert>

        <div class="flex flex-wrap gap-2">
          <Button as-child size="sm" variant="outline">
            <RouterLink to="/my/api-probe-connections"><Cable class="h-4 w-4" />管理连接</RouterLink>
          </Button>
          <Button as-child size="sm" variant="outline">
            <RouterLink to="/my/api-probe-connections?create=1"><ExternalLink class="h-4 w-4" />新建连接</RouterLink>
          </Button>
        </div>
      </template>
    </div>
  </Card>
</template>
