<script setup lang="ts">
import { computed } from 'vue'
import { renderAnnouncementMarkdown } from '@/lib/announcementUtils'

const props = defineProps<{
  contentMarkdown: string
}>()

const html = computed(() => renderAnnouncementMarkdown(props.contentMarkdown))
</script>

<template>
  <div class="announcement-markdown" v-html="html"></div>
</template>

<style scoped>
.announcement-markdown :deep(h1),
.announcement-markdown :deep(h2),
.announcement-markdown :deep(h3) {
  margin-top: 1.25rem;
  font-weight: 650;
  line-height: 1.25;
}

.announcement-markdown :deep(h1) {
  font-size: 1.5rem;
}

.announcement-markdown :deep(h2) {
  font-size: 1.25rem;
}

.announcement-markdown :deep(h3) {
  font-size: 1.125rem;
}

.announcement-markdown :deep(p),
.announcement-markdown :deep(ul),
.announcement-markdown :deep(ol) {
  margin-top: 0.75rem;
  color: var(--muted-foreground);
  font-size: 0.9375rem;
  line-height: 1.75;
}

.announcement-markdown :deep(ul),
.announcement-markdown :deep(ol) {
  padding-left: 1.25rem;
}

.announcement-markdown :deep(ul) {
  list-style: disc;
}

.announcement-markdown :deep(ol) {
  list-style: decimal;
}

.announcement-markdown :deep(a) {
  color: var(--primary);
  text-decoration: underline;
  text-underline-offset: 3px;
}
</style>
