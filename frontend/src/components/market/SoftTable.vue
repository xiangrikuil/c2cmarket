<script setup lang="ts">
import { Card } from '@/components/ui/card'
import { Table, TableBody, TableHead, TableHeader, TableRow } from '@/components/ui/table'

defineProps<{
  columns: string[]
}>()
</script>

<template>
  <Card class="overflow-hidden p-0">
    <Table class="c2c-table c2c-soft-table">
      <TableHeader>
        <TableRow>
          <TableHead v-for="column in columns" :key="column">{{ column }}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        <slot />
      </TableBody>
    </Table>
    <slot name="footer" />
  </Card>
</template>
