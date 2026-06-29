<script setup lang="ts">
import { computed, ref } from 'vue'
import { Button } from '@/components/ui/button'

const props = defineProps<{
  items: string[]
  modelValue?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const localActive = ref(props.items[0] ?? '')
const active = computed(() => props.modelValue ?? localActive.value)

function select(item: string) {
  localActive.value = item
  emit('update:modelValue', item)
}
</script>

<template>
  <div class="mb-4 flex flex-wrap gap-2">
    <Button
      v-for="item in items"
      :key="item"
      size="sm"
      :variant="active === item ? 'default' : 'outline'"
      @click="select(item)"
    >
      {{ item }}
    </Button>
  </div>
</template>
