<script setup lang="ts">
import { Info } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import type { ApiDeliveryMode } from '@/lib/api'
import { getApiDeliveryModeDescription, getApiDeliveryModeLabel } from '@/lib/api'

const props = defineProps<{
  mode: ApiDeliveryMode
}>()
</script>

<template>
  <Popover>
    <PopoverTrigger as-child>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        class="h-6 w-6 rounded-full text-muted-foreground hover:text-foreground"
        :aria-label="`${getApiDeliveryModeLabel(props.mode)}说明`"
      >
        <Info class="h-3.5 w-3.5" />
      </Button>
    </PopoverTrigger>
    <PopoverContent class="w-72 text-xs leading-5" side="top" align="center">
      <div class="font-semibold text-foreground">{{ getApiDeliveryModeLabel(props.mode) }}</div>
      <p class="mt-1 text-muted-foreground">{{ getApiDeliveryModeDescription(props.mode) }}</p>
    </PopoverContent>
  </Popover>
</template>
