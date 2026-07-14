<script setup lang="ts">
import { computed } from 'vue'
import { formatLocalDateTime } from '@/lib/presentation'

const props = withDefaults(defineProps<{ value: string | Date, seconds?: boolean }>(), { seconds: false })
const text = computed(() => formatLocalDateTime(props.value, props.seconds))
</script>

<template><time :datetime="value instanceof Date ? value.toISOString() : value" :title="String(value)">{{ text }}</time></template>
