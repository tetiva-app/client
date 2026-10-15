<script setup lang="ts">
import { Button } from '@/components/ui/button'
import { useCopy } from '@/composables/useLocale'
import { COMPARE_COPY } from './copy'

defineProps<{
  message: string
  offerRaw?: boolean
}>()

const emit = defineEmits<{
  (e: 'raw'): void
}>()

const copy = useCopy(COMPARE_COPY)
</script>

<template>
  <div class="flex flex-1 items-center justify-center p-6 text-center text-sm text-muted-foreground">
    <div>
      <p>{{ message }}</p>
      <Button v-if="offerRaw" variant="outline" size="sm" class="mt-3 h-7" @click="emit('raw')">
        {{ copy.state.showRaw }}
      </Button>
    </div>
  </div>
</template>
